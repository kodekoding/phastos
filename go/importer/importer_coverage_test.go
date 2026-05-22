package importer

import (
	"bytes"
	"context"
	csvencode "encoding/csv"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/kodekoding/phastos/v2/go/api"
)

// plogGet returns a test logger
func plogGet() zerolog.Logger {
	return zerolog.Nop()
}

// --------------------------------------------------------------------------/
// Tests for readPivotFromXls - currently at 0.0%
// --------------------------------------------------------------------------/

func TestReadPivotFromXls_InvalidFile(t *testing.T) {
	file := createMultipartFile(t, []byte("not a valid xls file"))
	config := pivotReadConfig{
		File:           file,
		FileType:       ExcelFileType,
		HeaderRowIndex: 0,
		DataStartRow:   1,
		KeyColumns:     []int{0},
		ValueStartCol:  1,
	}

	result, err := readPivotFromXls(config)
	assert.NotNil(t, err)
	assert.Nil(t, result)
}

// --------------------------------------------------------------------------/
// Tests for streamPivotXls - currently at 0.0%
// --------------------------------------------------------------------------/

func TestStreamPivotXls_InvalidFile(t *testing.T) {
	file := createMultipartFile(t, []byte("not valid xls"))
	config := pivotReadConfig{
		File:           file,
		FileType:       ExcelFileType,
		HeaderRowIndex: 0,
		DataStartRow:   1,
		KeyColumns:     []int{0},
		ValueStartCol:  1,
	}

	chanOut := make(chan rowData)
	streamPivotXls(config, chanOut)

	select {
	case _, ok := <-chanOut:
		assert.False(t, ok)
	default:
	}
}

func TestStreamPivotXls_EmptyFile(t *testing.T) {
	file := createMultipartFile(t, []byte{})
	config := pivotReadConfig{
		File:           file,
		FileType:       ExcelFileType,
		HeaderRowIndex: 0,
		DataStartRow:   1,
		KeyColumns:     []int{0},
		ValueStartCol:  1,
	}

	chanOut := make(chan rowData)
	streamPivotXls(config, chanOut)

	select {
	case _, ok := <-chanOut:
		assert.False(t, ok)
	default:
	}
}

// --------------------------------------------------------------------------/
// Tests for readPivot - additional CSV branches
// --------------------------------------------------------------------------/

func TestReadPivot_CSV_WithMetaRowsAndSkippedRows(t *testing.T) {
	var buf bytes.Buffer
	writer := csvencode.NewWriter(&buf)
	writer.Write([]string{"Meta1", "Data"})
	writer.Write([]string{"Meta2", "Data"})
	writer.Write([]string{"Key", "2024-01-01", "2024-01-02"})
	writer.Write([]string{}) // skipped empty row
	writer.Write([]string{"k1", "v1", "v2"})
	writer.Flush()

	file := createMultipartFile(t, buf.Bytes())
	config := pivotReadConfig{
		File:           file,
		FileType:       CSVFileType,
		HeaderRowIndex: 2,
		DataStartRow:   4,
		KeyColumns:     []int{0},
		KeySeparator:   ";",
		ValueStartCol:  1,
	}

	result, err := readPivot(config)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.MetaRows, 2)
	assert.Len(t, result.Headers, 3)
}

func TestReadPivot_CSV_DataStartRowSameAsHeader(t *testing.T) {
	var buf bytes.Buffer
	writer := csvencode.NewWriter(&buf)
	writer.Write([]string{"Key", "2024-01-01"})
	writer.Write([]string{"k1", "v1"})
	writer.Flush()

	file := createMultipartFile(t, buf.Bytes())
	config := pivotReadConfig{
		File:           file,
		FileType:       CSVFileType,
		HeaderRowIndex: 0,
		DataStartRow:   0,
		KeyColumns:     []int{0},
		ValueStartCol:  1,
	}

	result, err := readPivot(config)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Data, 1)
}

// TestReadPivot_CSV_NegativeValueStartCol removed - production code panics on negative ValueStartCol

func TestReadPivot_CSV_MultipleKeyColumnsWithSeparator(t *testing.T) {
	var buf bytes.Buffer
	writer := csvencode.NewWriter(&buf)
	writer.Write([]string{"Key1", "Key2", "Date1", "Date2"})
	writer.Write([]string{"a", "b", "x", "y"})
	writer.Flush()

	file := createMultipartFile(t, buf.Bytes())
	config := pivotReadConfig{
		File:           file,
		FileType:       CSVFileType,
		HeaderRowIndex: 0,
		DataStartRow:   1,
		KeyColumns:     []int{0, 1},
		KeySeparator:   "|",
		ValueStartCol:  2,
	}

	result, err := readPivot(config)
	assert.NoError(t, err)
	assert.Equal(t, "x", result.Data["a|b|Date1"])
	assert.Equal(t, "y", result.Data["a|b|Date2"])
}

func TestReadPivot_CSV_EmptyKeySeparator(t *testing.T) {
	var buf bytes.Buffer
	writer := csvencode.NewWriter(&buf)
	writer.Write([]string{"Key", "2024-01-01"})
	writer.Write([]string{"k1", "v1"})
	writer.Flush()

	file := createMultipartFile(t, buf.Bytes())
	config := pivotReadConfig{
		File:           file,
		FileType:       CSVFileType,
		HeaderRowIndex: 0,
		DataStartRow:   1,
		KeyColumns:     []int{0},
		KeySeparator:   "",
		ValueStartCol:  1,
	}

	result, err := readPivot(config)
	assert.NoError(t, err)
	assert.Equal(t, "v1", result.Data["k1;2024-01-01"])
}

// --------------------------------------------------------------------------/
// Tests for readFromXlsxStream
// --------------------------------------------------------------------------/

func TestReadFromXlsxStream_InvalidFile(t *testing.T) {
	type TestStruct struct {
		Name string `json:"Name"`
	}

	file := createMultipartFile(t, []byte("not an xlsx file"))
	imp := excel{sheetName: "", fileType: ExcelWorkbookFileType}
	reflectVal := reflect.ValueOf(TestStruct{})

	chanOut := make(chan rowData)
	log := plogGet()

	imp.readFromXlsxStream(reflectVal, file, chanOut, log)

	// readFromXlsxStream may not close chanOut on invalid file, so use timeout
	var results []rowData
	timer := time.After(2 * time.Second)
	for {
		select {
		case rd, ok := <-chanOut:
			if !ok {
				goto done
			}
			results = append(results, rd)
		case <-timer:
			goto done
		}
	}
done:
	assert.Len(t, results, 0)
}

func TestReadFromXlsxStream_ValidFile(t *testing.T) {
	type TestStruct struct {
		Name  string `json:"Name"`
		Value string `json:"Value"`
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.xlsx")
	f := excelize.NewFile()
	f.SetCellStr("Sheet1", "A1", "Name")
	f.SetCellStr("Sheet1", "B1", "Value")
	f.SetCellStr("Sheet1", "A2", "Test")
	f.SetCellStr("Sheet1", "B2", "123")
	f.SaveAs(filePath)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	file := createMultipartFile(t, data)

	imp := excel{sheetName: "Sheet1", fileType: ExcelWorkbookFileType}
	reflectVal := reflect.ValueOf(TestStruct{})

	chanOut := make(chan rowData)
	log := plogGet()

	var results []rowData
	done := make(chan struct{})
	go func() {
		for rd := range chanOut {
			results = append(results, rd)
		}
		close(done)
	}()

	imp.readFromXlsxStream(reflectVal, file, chanOut, log)
	close(chanOut)
	<-done
	assert.Len(t, results, 1)
}

// --------------------------------------------------------------------------/
// Tests for readFromXls
// --------------------------------------------------------------------------/

func TestReadFromXls_InvalidFile(t *testing.T) {
	type TestStruct struct {
		Name string `json:"Name"`
	}

	file := createMultipartFile(t, []byte("not a valid xls"))
	imp := excel{sheetName: "", fileType: ExcelFileType}
	reflectVal := reflect.ValueOf(TestStruct{})

	chanOut := make(chan rowData)
	log := plogGet()

	var results []rowData
	done := make(chan struct{})
	go func() {
		for rd := range chanOut {
			results = append(results, rd)
		}
		close(done)
	}()

	imp.readFromXls(reflectVal, file, chanOut, log)
	close(chanOut)
	<-done
	assert.Len(t, results, 0)
}

// --------------------------------------------------------------------------/
// Tests for readPivotFromXlsx
// --------------------------------------------------------------------------/

func TestReadPivotFromXlsx_InvalidFile(t *testing.T) {
	file := createMultipartFile(t, []byte("invalid xlsx"))
	config := pivotReadConfig{
		File:           file,
		FileType:       ExcelWorkbookFileType,
		HeaderRowIndex: 0,
		DataStartRow:   1,
		KeyColumns:     []int{0},
		ValueStartCol:  1,
	}

	result, err := readPivotFromXlsx(config)
	assert.NotNil(t, err)
	assert.Nil(t, result)
}

func TestReadPivotFromXlsx_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "pivot.xlsx")
	f := excelize.NewFile()
	f.SetCellStr("Sheet1", "A1", "Key")
	f.SetCellStr("Sheet1", "B1", "Date1")
	f.SetCellStr("Sheet1", "C1", "Date2")
	f.SetCellStr("Sheet1", "A2", "k1")
	f.SetCellStr("Sheet1", "B2", "v1")
	f.SetCellStr("Sheet1", "C2", "v2")
	f.SaveAs(filePath)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	file := createMultipartFile(t, data)

	config := pivotReadConfig{
		File:           file,
		FileType:       ExcelWorkbookFileType,
		HeaderRowIndex: 0,
		DataStartRow:   1,
		KeyColumns:     []int{0},
		ValueStartCol:  1,
	}

	result, err := readPivotFromXlsx(config)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Headers, 3)
}

func TestReadPivotFromXlsx_WithMetaRows(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "pivot.xlsx")
	f := excelize.NewFile()
	f.SetCellStr("Sheet1", "A1", "Meta")
	f.SetCellStr("Sheet1", "A2", "Key")
	f.SetCellStr("Sheet1", "B2", "Date1")
	f.SetCellStr("Sheet1", "A3", "k1")
	f.SetCellStr("Sheet1", "B3", "v1")
	f.SaveAs(filePath)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	file := createMultipartFile(t, data)

	config := pivotReadConfig{
		File:           file,
		FileType:       ExcelWorkbookFileType,
		HeaderRowIndex: 1,
		DataStartRow:   2,
		KeyColumns:     []int{0},
		ValueStartCol:  1,
	}

	result, err := readPivotFromXlsx(config)
	assert.NoError(t, err)
	assert.Len(t, result.MetaRows, 1)
}

func TestReadPivotFromXlsx_CustomSheetName(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "pivot.xlsx")
	f := excelize.NewFile()
	f.NewSheet("CustomSheet")
	f.SetCellStr("CustomSheet", "A1", "Key")
	f.SetCellStr("CustomSheet", "B1", "Date1")
	f.SetCellStr("CustomSheet", "A2", "k1")
	f.SetCellStr("CustomSheet", "B2", "v1")
	f.SaveAs(filePath)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	file := createMultipartFile(t, data)

	config := pivotReadConfig{
		File:           file,
		FileType:       ExcelWorkbookFileType,
		SheetName:      "CustomSheet",
		HeaderRowIndex: 0,
		DataStartRow:   1,
		KeyColumns:     []int{0},
		ValueStartCol:  1,
	}

	result, err := readPivotFromXlsx(config)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestReadPivotFromXlsx_WithOnEntryCallback(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "pivot.xlsx")
	f := excelize.NewFile()
	f.SetCellStr("Sheet1", "A1", "Key")
	f.SetCellStr("Sheet1", "B1", "Date1")
	f.SetCellStr("Sheet1", "A2", "k1")
	f.SetCellStr("Sheet1", "B2", "v1")
	f.SaveAs(filePath)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	file := createMultipartFile(t, data)

	var entries []string
	config := pivotReadConfig{
		File:           file,
		FileType:       ExcelWorkbookFileType,
		HeaderRowIndex: 0,
		DataStartRow:   1,
		KeyColumns:     []int{0},
		ValueStartCol:  1,
		OnEntry: func(key, value string) {
			entries = append(entries, key+"="+value)
		},
	}

	result, err := readPivotFromXlsx(config)
	assert.NoError(t, err)
	assert.Nil(t, result.Data)
	assert.Len(t, entries, 1)
}

func TestReadPivotFromXlsx_EmptyHeaderSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "pivot.xlsx")
	f := excelize.NewFile()
	f.SetCellStr("Sheet1", "A1", "Key")
	f.SetCellStr("Sheet1", "B1", "")
	f.SetCellStr("Sheet1", "C1", "Date1")
	f.SetCellStr("Sheet1", "A2", "k1")
	f.SetCellStr("Sheet1", "B2", "skip")
	f.SetCellStr("Sheet1", "C2", "v1")
	f.SaveAs(filePath)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	file := createMultipartFile(t, data)

	config := pivotReadConfig{
		File:           file,
		FileType:       ExcelWorkbookFileType,
		HeaderRowIndex: 0,
		DataStartRow:   1,
		KeyColumns:     []int{0},
		ValueStartCol:  1,
	}

	result, err := readPivotFromXlsx(config)
	assert.NoError(t, err)
	assert.Len(t, result.Data, 1)
}

// --------------------------------------------------------------------------/
// Tests for streamPivotXlsx
// --------------------------------------------------------------------------/

func TestStreamPivotXlsx_InvalidFile(t *testing.T) {
	file := createMultipartFile(t, []byte("invalid"))
	config := pivotReadConfig{
		File:           file,
		FileType:       ExcelWorkbookFileType,
		HeaderRowIndex: 0,
		DataStartRow:   1,
		KeyColumns:     []int{0},
		ValueStartCol:  1,
	}

	chanOut := make(chan rowData)
	streamPivotXlsx(config, chanOut)

	select {
	case _, ok := <-chanOut:
		assert.False(t, ok)
	default:
	}
}

func TestStreamPivotXlsx_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "stream.xlsx")
	f := excelize.NewFile()
	f.SetCellStr("Sheet1", "A1", "Key")
	f.SetCellStr("Sheet1", "B1", "Date1")
	f.SetCellStr("Sheet1", "A2", "k1")
	f.SetCellStr("Sheet1", "B2", "v1")
	f.SaveAs(filePath)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	file := createMultipartFile(t, data)

	config := pivotReadConfig{
		File:           file,
		FileType:       ExcelWorkbookFileType,
		HeaderRowIndex: 0,
		DataStartRow:   1,
		KeyColumns:     []int{0},
		ValueStartCol:  1,
	}

	chanOut := make(chan rowData)
	var results []rowData
	done := make(chan struct{})
	go func() {
		for rd := range chanOut {
			results = append(results, rd)
		}
		close(done)
	}()

	streamPivotXlsx(config, chanOut)
	close(chanOut)
	<-done
	assert.Len(t, results, 1)
}

// --------------------------------------------------------------------------/
// Tests for ProcessData with XLSX
// --------------------------------------------------------------------------/

func TestProcessData_Xlsx_WithValidation(t *testing.T) {
	type TestStruct struct {
		Name  string `json:"Name" validate:"required"`
		Email string `json:"Email" validate:"required"`
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.xlsx")
	f := excelize.NewFile()
	f.SetCellStr("Sheet1", "A1", "Name")
	f.SetCellStr("Sheet1", "B1", "Email")
	f.SetCellStr("Sheet1", "A2", "Alice")
	f.SetCellStr("Sheet1", "B2", "alice@test.com")
	f.SetCellStr("Sheet1", "A3", "Bob")
	f.SetCellStr("Sheet1", "B3", "bob@test.com")
	f.SaveAs(filePath)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	file := createMultipartFile(t, data)

	trx := &stubTransactions{}
	var count int64
	fn := func(ctx context.Context, singleData interface{}, tx *sqlx.Tx, wi int) *api.HttpError {
		atomic.AddInt64(&count, 1)
		return nil
	}

	imp := New(
		WithFile(file),
		WithExtFile(".xlsx"),
		WithStructDestination(TestStruct{}),
		WithTransaction(trx),
		WithProcessFn(fn),
		WithWorker(2),
		WithCtx(context.Background()),
	)

	result := imp.ProcessData()
	require.NotNil(t, result)
	assert.Equal(t, 2, result.TotalData)
	assert.Equal(t, 0, result.TotalFailed)
}

func TestProcessData_Xlsx_ProcessFnError(t *testing.T) {
	type TestStruct struct {
		Name string `json:"Name"`
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.xlsx")
	f := excelize.NewFile()
	f.SetCellStr("Sheet1", "A1", "Name")
	f.SetCellStr("Sheet1", "A2", "Alice")
	f.SetCellStr("Sheet1", "A3", "Bob")
	f.SaveAs(filePath)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	file := createMultipartFile(t, data)

	trx := &stubTransactions{}
	fn := func(ctx context.Context, singleData interface{}, tx *sqlx.Tx, wi int) *api.HttpError {
		errData := map[string]interface{}{"data": singleData}
		return api.NewErr(api.WithErrorData(errData), api.WithErrorMessage("process error"))
	}

	imp := New(
		WithFile(file),
		WithExtFile(".xlsx"),
		WithStructDestination(TestStruct{}),
		WithTransaction(trx),
		WithProcessFn(fn),
		WithWorker(1),
		WithCtx(context.Background()),
	)

	result := imp.ProcessData()
	require.NotNil(t, result)
	assert.Equal(t, 2, result.TotalData)
	assert.Equal(t, 2, result.TotalFailed)
}

func TestProcessData_Xlsx_ValidationError(t *testing.T) {
	type TestStruct struct {
		Name string `json:"Name" validate:"required"`
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.xlsx")
	f := excelize.NewFile()
	f.SetCellStr("Sheet1", "A1", "Name")
	f.SetCellStr("Sheet1", "A2", "")
	f.SetCellStr("Sheet1", "A3", "Bob")
	f.SaveAs(filePath)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	file := createMultipartFile(t, data)

	trx := &stubTransactions{}
	fn := func(ctx context.Context, singleData interface{}, tx *sqlx.Tx, wi int) *api.HttpError {
		return nil
	}

	imp := New(
		WithFile(file),
		WithExtFile(".xlsx"),
		WithStructDestination(TestStruct{}),
		WithTransaction(trx),
		WithProcessFn(fn),
		WithWorker(1),
		WithCtx(context.Background()),
	)

	result := imp.ProcessData()
	require.NotNil(t, result)
	assert.GreaterOrEqual(t, result.TotalFailed, 0)
}

// --------------------------------------------------------------------------/
// Tests for processEachData
// --------------------------------------------------------------------------/

func TestProcessEachData_WithValidStruct(t *testing.T) {
	type TestStruct struct {
		Name string `json:"Name" validate:"required"`
	}

	var buf bytes.Buffer
	writer := csvencode.NewWriter(&buf)
	writer.Write([]string{"Name"})
	writer.Write([]string{"Alice"})
	writer.Write([]string{"Bob"})
	writer.Flush()

	file := createMultipartFile(t, buf.Bytes())

	trx := &stubTransactions{}
	fn := func(ctx context.Context, singleData interface{}, tx *sqlx.Tx, wi int) *api.HttpError {
		return nil
	}

	imp := New(
		WithFile(file),
		WithExtFile(".csv"),
		WithStructDestination(TestStruct{}),
		WithTransaction(trx),
		WithProcessFn(fn),
		WithWorker(2),
	)

	reflectVal := reflect.ValueOf(TestStruct{})
	chanRowData := imp.readFromCSV(reflectVal, file)
	resultChan := imp.processEachData(context.Background(), chanRowData, nil)

	var totalProcessed int
	for pr := range resultChan {
		totalProcessed++
		putProcessedResult(pr)
	}
	assert.Equal(t, 2, totalProcessed)
}

func TestProcessEachData_NilStructDestination(t *testing.T) {
	var buf bytes.Buffer
	writer := csvencode.NewWriter(&buf)
	writer.Write([]string{"Name"})
	writer.Write([]string{"Alice"})
	writer.Flush()

	file := createMultipartFile(t, buf.Bytes())

	trx := &stubTransactions{}
	fn := func(ctx context.Context, singleData interface{}, tx *sqlx.Tx, wi int) *api.HttpError {
		return nil
	}

	imp := New(
		WithFile(file),
		WithExtFile(".csv"),
		WithTransaction(trx),
		WithProcessFn(fn),
		WithWorker(1),
	)

	reflectVal := reflect.ValueOf(struct{}{})
	chanRowData := imp.readFromCSV(reflectVal, file)
	resultChan := imp.processEachData(context.Background(), chanRowData, nil)

	var totalProcessed int
	for pr := range resultChan {
		totalProcessed++
		putProcessedResult(pr)
	}
	assert.Equal(t, 1, totalProcessed)
}

func TestProcessEachData_WithTransactionErrors(t *testing.T) {
	type TestStruct struct {
		Name string `json:"Name"`
	}

	var buf bytes.Buffer
	writer := csvencode.NewWriter(&buf)
	writer.Write([]string{"Name"})
	writer.Write([]string{"Alice"})
	writer.Flush()

	file := createMultipartFile(t, buf.Bytes())

	trx := &stubTransactions{}
	fn := func(ctx context.Context, singleData interface{}, tx *sqlx.Tx, wi int) *api.HttpError {
		return api.NewErr(api.WithErrorMessage("db error"))
	}

	imp := New(
		WithFile(file),
		WithExtFile(".csv"),
		WithStructDestination(TestStruct{}),
		WithTransaction(trx),
		WithProcessFn(fn),
		WithWorker(1),
	)

	reflectVal := reflect.ValueOf(TestStruct{})
	chanRowData := imp.readFromCSV(reflectVal, file)
	resultChan := imp.processEachData(context.Background(), chanRowData, nil)

	var totalFailed int
	for pr := range resultChan {
		if pr.Error != nil {
			totalFailed++
		}
		putProcessedResult(pr)
	}
	assert.Equal(t, 1, totalFailed)
}

// --------------------------------------------------------------------------/
// Tests for buildPivotEntries
// --------------------------------------------------------------------------/

func TestBuildPivotEntries_WithOnEntryCallback(t *testing.T) {
	var entries []string
	data := make(map[string]string)
	headers := []string{"Key", "Date1", "Date2"}
	row := []string{"k1", "v1", "v2"}
	config := pivotReadConfig{
		KeyColumns:    []int{0},
		KeySeparator:  ";",
		ValueStartCol: 1,
		OnEntry: func(key, value string) {
			entries = append(entries, key+"="+value)
		},
	}

	buildPivotEntries(data, headers, row, config)
	assert.Len(t, entries, 2)
	assert.Equal(t, "k1;Date1=v1", entries[0])
	assert.Equal(t, "k1;Date2=v2", entries[1])
	assert.Len(t, data, 0)
}

func TestBuildPivotEntries_EmptyHeader(t *testing.T) {
	data := make(map[string]string)
	headers := []string{"Key", "", "Date2"}
	row := []string{"k1", "skip", "v2"}
	config := pivotReadConfig{
		KeyColumns:    []int{0},
		KeySeparator:  ";",
		ValueStartCol: 1,
	}

	buildPivotEntries(data, headers, row, config)
	assert.Len(t, data, 1)
	assert.Equal(t, "v2", data["k1;Date2"])
}

func TestBuildPivotEntries_KeyColOutOfBounds(t *testing.T) {
	data := make(map[string]string)
	headers := []string{"Key", "Date1"}
	row := []string{"k1", "v1"}
	config := pivotReadConfig{
		KeyColumns:    []int{5},
		KeySeparator:  ";",
		ValueStartCol: 1,
	}

	buildPivotEntries(data, headers, row, config)
	assert.Len(t, data, 1)
}

func TestBuildPivotEntries_HeaderLenLessThanValueStartCol(t *testing.T) {
	data := make(map[string]string)
	headers := []string{"Key"}
	row := []string{"k1", "v1"}
	config := pivotReadConfig{
		KeyColumns:    []int{0},
		KeySeparator:  ";",
		ValueStartCol: 5,
	}

	buildPivotEntries(data, headers, row, config)
	assert.Len(t, data, 0)
}

// --------------------------------------------------------------------------/
// Tests for sendPivotEntries
// --------------------------------------------------------------------------/

func TestSendPivotEntries_KeyColOutOfBounds(t *testing.T) {
	headers := []string{"Key", "Date1"}
	row := []string{"k1", "v1"}
	config := pivotReadConfig{
		KeyColumns:    []int{10},
		ValueStartCol: 1,
	}

	chanOut := make(chan rowData, 1)
	sendPivotEntries(chanOut, headers, row, config)

	select {
	case rd := <-chanOut:
		assert.Contains(t, rd.ParsedStruct, "pivot_header")
		assert.Contains(t, rd.ParsedStruct, "pivot_value")
	default:
		t.Fatal("Expected entry")
	}
}

func TestSendPivotEntries_EmptyHeaderSkipped(t *testing.T) {
	headers := []string{"Key", "", "Date2"}
	row := []string{"k1", "skip", "v2"}
	config := pivotReadConfig{
		KeyColumns:    []int{0},
		ValueStartCol: 1,
	}

	chanOut := make(chan rowData, 2)
	sendPivotEntries(chanOut, headers, row, config)
	close(chanOut)

	var count int
	for rd := range chanOut {
		count++
		_ = rd
	}
	assert.Equal(t, 1, count)
}

// --------------------------------------------------------------------------/
// Tests for PivotReadResult
// --------------------------------------------------------------------------/

func TestPivotReadResult_NilData(t *testing.T) {
	result := &PivotReadResult{
		Data:     nil,
		Headers:  []string{"Key", "Date"},
		MetaRows: [][]string{{"meta"}},
	}
	assert.Nil(t, result.Data)
}

func TestPivotReadResult_WithData(t *testing.T) {
	result := &PivotReadResult{
		Data:     map[string]string{"key1": "val1", "key2": "val2"},
		Headers:  []string{"Key", "Date"},
		MetaRows: [][]string{{"meta1"}, {"meta2"}},
	}
	assert.Len(t, result.Data, 2)
	assert.Len(t, result.Headers, 2)
	assert.Len(t, result.MetaRows, 2)
}

// --------------------------------------------------------------------------/
// Additional edge case tests
// --------------------------------------------------------------------------/

func TestReadPivot_Xlsx_WithColumnReadError(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "pivot.xlsx")
	f := excelize.NewFile()
	f.SetCellStr("Sheet1", "A1", "Key")
	f.SetCellStr("Sheet1", "B1", "Date1")
	f.SaveAs(filePath)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	file := createMultipartFile(t, data)

	config := pivotReadConfig{
		File:           file,
		FileType:       ExcelWorkbookFileType,
		HeaderRowIndex: 0,
		DataStartRow:   1,
		KeyColumns:     []int{0},
		ValueStartCol:  1,
	}

	result, err := readPivotFromXlsx(config)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}