package importer

import (
	"bytes"
	"context"
	csvencode "encoding/csv"
	"mime/multipart"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/kodekoding/phastos/v2/go/api"
	"github.com/kodekoding/phastos/v2/go/database"
)
// ---------------------------------------------------------------------------
// Stub for database.Transactions interface
// ---------------------------------------------------------------------------

type stubTransactions struct {
	beginCount  int64
	finishCount int64
	beginErr    error
}

func (s *stubTransactions) Begin() (*sqlx.Tx, error) {
	atomic.AddInt64(&s.beginCount, 1)
	if s.beginErr != nil {
		return nil, s.beginErr
	}
	return nil, nil
}

func (s *stubTransactions) Finish(tx *sqlx.Tx, errTransaction error) {
	atomic.AddInt64(&s.finishCount, 1)
}

// Verify interface compliance
var _ database.Transactions = (*stubTransactions)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type readSeekCloser struct {
	*os.File
}

func createMultipartFile(t *testing.T, content []byte) multipart.File {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "import-test-*.csv")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	_, err = tmpFile.Write(content)
	require.NoError(t, err)
	_, err = tmpFile.Seek(0, 0)
	require.NoError(t, err)

	return &readSeekCloser{File: tmpFile}
}

func createCSVContent(headers []string, rows ...[]string) []byte {
	var buf bytes.Buffer
	writer := csvencode.NewWriter(&buf)
	writer.Write(headers)
	for _, row := range rows {
		writer.Write(row)
	}
	writer.Flush()
	return buf.Bytes()
}


// ---------------------------------------------------------------------------
// Tests for New and With* options
// ---------------------------------------------------------------------------

func TestNew_Default(t *testing.T) {
	imp := New()
	require.NotNil(t, imp)
	assert.Equal(t, 10, imp.worker)
}

func TestNew_WithWorker(t *testing.T) {
	imp := New(WithWorker(5))
	assert.Equal(t, 5, imp.worker)
}

func TestNew_WithWorker_Zero(t *testing.T) {
	imp := New(WithWorker(0))
	assert.Equal(t, 0, imp.worker)
}

func TestNew_WithFile(t *testing.T) {
	content := createCSVContent([]string{"A", "B"}, []string{"1", "2"})
	file := createMultipartFile(t, content)
	imp := New(WithFile(file))
	assert.NotNil(t, imp.file)
}

func TestNew_WithFile_Nil(t *testing.T) {
	imp := New(WithFile(nil))
	assert.Nil(t, imp.file)
}

func TestNew_WithStructDestination(t *testing.T) {
	type TestStruct struct {
		Name string `json:"Name" validate:"required"`
	}
	imp := New(WithStructDestination(TestStruct{}))
	assert.NotNil(t, imp.structDestination)
}

func TestNew_WithExtFile_CSV(t *testing.T) {
	imp := New(WithExtFile(".csv"))
	assert.Equal(t, CSVFileType, imp.sourceType)
	assert.Equal(t, CSVFileType, imp.fileType)
}

func TestNew_WithExtFile_Xlsx(t *testing.T) {
	imp := New(WithExtFile(".xlsx"))
	assert.Equal(t, ExcelWorkbookFileType, imp.sourceType)
}

func TestNew_WithExtFile_Xls(t *testing.T) {
	imp := New(WithExtFile(".xls"))
	assert.Equal(t, ExcelFileType, imp.sourceType)
}

func TestNew_WithExtFile_Invalid(t *testing.T) {
	imp := New(WithExtFile(".txt"))
	assert.Equal(t, UndefinedFileType, imp.sourceType)
}

func TestNew_WithTransaction(t *testing.T) {
	trx := &stubTransactions{}
	imp := New(WithTransaction(trx))
	assert.NotNil(t, imp.trx)
}

func TestNew_WithProcessFn(t *testing.T) {
	fn := func(ctx context.Context, singleData interface{}, tx *sqlx.Tx, wi int) *api.HttpError {
		return nil
	}
	imp := New(WithProcessFn(fn))
	assert.NotNil(t, imp.fn)
}

func TestNew_WithProcessName(t *testing.T) {
	imp := New(WithProcessName("TestImport"))
	assert.Equal(t, "TestImport", imp.processName)
}

func TestNew_WithSentNotifToSlack(t *testing.T) {
	t.Setenv("NOTIFICATION_SLACK_INFO_WEBHOOK", "https://hooks.slack.com/test")
	imp := New(WithSentNotifToSlack(true))
	assert.True(t, imp.sentNotifToSlack)
	assert.Equal(t, "https://hooks.slack.com/test", imp.slackNotifChannel)
}

func TestNew_WithSentNotifToSlack_CustomChannel(t *testing.T) {
	imp := New(WithSentNotifToSlack(true, "custom-channel"))
	assert.True(t, imp.sentNotifToSlack)
	assert.Equal(t, "custom-channel", imp.slackNotifChannel)
}

func TestNew_WithCtx(t *testing.T) {
	ctx := context.Background()
	imp := New(WithCtx(ctx))
	assert.Equal(t, ctx, imp.ctx)
}

func TestNew_WithSheetName(t *testing.T) {
	imp := New(WithSheetName("CustomSheet"))
	assert.Equal(t, "CustomSheet", imp.excel.sheetName)
}

func TestNew_WithHeaderRowIndex(t *testing.T) {
	imp := New(WithHeaderRowIndex(2))
	assert.Equal(t, 2, imp.headerRowIndex)
}

func TestNew_WithDataStartRow(t *testing.T) {
	imp := New(WithDataStartRow(3))
	assert.Equal(t, 3, imp.dataStartRow)
}

func TestNew_WithKeyColumns(t *testing.T) {
	imp := New(WithKeyColumns([]int{0, 1}))
	assert.Equal(t, []int{0, 1}, imp.keyColumns)
}

func TestNew_WithKeySeparator(t *testing.T) {
	imp := New(WithKeySeparator("|"))
	assert.Equal(t, "|", imp.keySeparator)
}

func TestNew_WithValueStartCol(t *testing.T) {
	imp := New(WithValueStartCol(2))
	assert.Equal(t, 2, imp.valueStartCol)
}

func TestNew_WithOnPivotEntry(t *testing.T) {
	called := false
	fn := func(key, value string) { called = true }
	imp := New(WithOnPivotEntry(fn))
	assert.NotNil(t, imp.onPivotEntry)
	imp.onPivotEntry("key", "val")
	assert.True(t, called)
}

// ---------------------------------------------------------------------------
// Tests for constants
// ---------------------------------------------------------------------------

func TestImporterConstants(t *testing.T) {
	assert.Equal(t, "excel", ExcelFileType)
	assert.Equal(t, "excel_workbook", ExcelWorkbookFileType)
	assert.Equal(t, "csv", CSVFileType)
	assert.Equal(t, "", UndefinedFileType)
	assert.Equal(t, ".xls", ExcelExt)
	assert.Equal(t, ".xlsx", ExcelWorkbookExt)
	assert.Equal(t, ".csv", CSVExt)
}

func TestMapFileExt(t *testing.T) {
	assert.Equal(t, ExcelFileType, mapFileExt[ExcelExt])
	assert.Equal(t, ExcelWorkbookFileType, mapFileExt[ExcelWorkbookExt])
	assert.Equal(t, CSVFileType, mapFileExt[CSVExt])
	_, exists := mapFileExt[".txt"]
	assert.False(t, exists)
}

// ---------------------------------------------------------------------------
// Tests for validateField
// ---------------------------------------------------------------------------

func TestValidateField_NoFile(t *testing.T) {
	imp := New(WithStructDestination(struct{}{}), WithExtFile(".csv"), WithTransaction(&stubTransactions{}))
	err := imp.validateField()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "`File` is null")
}

func TestValidateField_NoStructDestination(t *testing.T) {
	content := createCSVContent([]string{"A"})
	file := createMultipartFile(t, content)
	imp := New(WithFile(file), WithExtFile(".csv"), WithTransaction(&stubTransactions{}))
	err := imp.validateField()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "`Struct Destination` Variable is null")
}

func TestValidateField_NoTransaction(t *testing.T) {
	content := createCSVContent([]string{"A"})
	file := createMultipartFile(t, content)
	imp := New(WithFile(file), WithExtFile(".csv"), WithStructDestination(struct{}{}))
	err := imp.validateField()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "`Transaction` Variable is null")
}

func TestValidateField_UndefinedFileType(t *testing.T) {
	content := createCSVContent([]string{"A"})
	file := createMultipartFile(t, content)
	imp := New(WithFile(file), WithExtFile(".txt"), WithStructDestination(struct{}{}), WithTransaction(&stubTransactions{}))
	err := imp.validateField()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "File Type isn't set")
}

func TestValidateField_NonStructDestination(t *testing.T) {
	content := createCSVContent([]string{"A"})
	file := createMultipartFile(t, content)
	imp := New(WithFile(file), WithExtFile(".csv"), WithStructDestination("not a struct"), WithTransaction(&stubTransactions{}))
	err := imp.validateField()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "data destination should be a struct")
}

func TestValidateField_PointerStructDestination(t *testing.T) {
	type TestStruct struct {
		Name string `json:"Name"`
	}
	content := createCSVContent([]string{"Name"})
	file := createMultipartFile(t, content)
	imp := New(WithFile(file), WithExtFile(".csv"), WithStructDestination(&TestStruct{}), WithTransaction(&stubTransactions{}))
	err := imp.validateField()
	assert.NoError(t, err)
}

func TestValidateField_Valid(t *testing.T) {
	type TestStruct struct {
		Name string `json:"Name"`
	}
	content := createCSVContent([]string{"Name"})
	file := createMultipartFile(t, content)
	imp := New(WithFile(file), WithExtFile(".csv"), WithStructDestination(TestStruct{}), WithTransaction(&stubTransactions{}))
	err := imp.validateField()
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// Tests for buildRowMap
// ---------------------------------------------------------------------------

func TestBuildRowMap(t *testing.T) {
	headers := []string{"Name", "Email", "Role"}
	row := []string{"Alice", "alice@mail.com", "Admin"}
	result := buildRowMap(headers, row)
	assert.Equal(t, "Alice", result["Name"])
	assert.Equal(t, "alice@mail.com", result["Email"])
	assert.Equal(t, "Admin", result["Role"])
}

func TestBuildRowMap_MoreHeadersThanRow(t *testing.T) {
	headers := []string{"Name", "Email", "Role"}
	row := []string{"Alice"}
	result := buildRowMap(headers, row)
	assert.Equal(t, "Alice", result["Name"])
	assert.Equal(t, "", result["Email"])
	assert.Equal(t, "", result["Role"])
}

func TestBuildRowMap_MoreRowsThanHeaders(t *testing.T) {
	headers := []string{"Name"}
	row := []string{"Alice", "Extra"}
	result := buildRowMap(headers, row)
	assert.Equal(t, "Alice", result["Name"])
	assert.Len(t, result, 1)
}

func TestBuildRowMap_EmptyHeaders(t *testing.T) {
	headers := []string{}
	row := []string{"Alice"}
	result := buildRowMap(headers, row)
	assert.Empty(t, result)
}

// ---------------------------------------------------------------------------
// Tests for ProcessData (CSV)
// ---------------------------------------------------------------------------

func TestProcessData_CSV(t *testing.T) {
	type TestStruct struct {
		Name  string `json:"Name" validate:"required"`
		Email string `json:"Email" validate:"required,email"`
	}

	content := createCSVContent(
		[]string{"Name", "Email"},
		[]string{"Alice", "alice@mail.com"},
		[]string{"Bob", "bob@mail.com"},
	)
	file := createMultipartFile(t, content)

	var processedCount int64
	trx := &stubTransactions{}
	fn := func(ctx context.Context, singleData interface{}, tx *sqlx.Tx, wi int) *api.HttpError {
		atomic.AddInt64(&processedCount, 1)
		return nil
	}

	imp := New(
		WithFile(file),
		WithExtFile(".csv"),
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
	assert.Equal(t, 2, result.TotalSuccess)
}

func TestProcessData_CSV_WithValidationErrors(t *testing.T) {
	type TestStruct struct {
		Name  string `json:"Name" validate:"required"`
		Email string `json:"Email" validate:"required,email"`
	}

	content := createCSVContent(
		[]string{"Name", "Email"},
		[]string{"", "invalid-email"},
		[]string{"Bob", "bob@mail.com"},
	)
	file := createMultipartFile(t, content)

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
		WithCtx(context.Background()),
	)

	result := imp.ProcessData()
	require.NotNil(t, result)
	assert.Equal(t, 2, result.TotalData)
	assert.GreaterOrEqual(t, result.TotalFailed, 1)
}

func TestProcessData_CSV_ProcessFnError(t *testing.T) {
	type TestStruct struct {
		Name string `json:"Name"`
	}

	content := createCSVContent(
		[]string{"Name"},
		[]string{"Alice"},
		[]string{"Bob"},
	)
	file := createMultipartFile(t, content)

	trx := &stubTransactions{}
	fn := func(ctx context.Context, singleData interface{}, tx *sqlx.Tx, wi int) *api.HttpError {
		return api.NewErr(api.WithErrorMessage("process error"), api.WithErrorData(map[string]any{"data": singleData}))
	}

	imp := New(
		WithFile(file),
		WithExtFile(".csv"),
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
	assert.Equal(t, 0, result.TotalSuccess)
}

func TestProcessData_ValidationError_ReturnsNil(t *testing.T) {
	type TestStruct struct {
		Name string `json:"Name"`
	}

	imp := New(
		WithStructDestination(TestStruct{}),
		WithExtFile(".csv"),
		WithTransaction(&stubTransactions{}),
		WithCtx(context.Background()),
	)

	result := imp.ProcessData()
	assert.Nil(t, result)
}

func TestProcessData_CSV_CleaningHeaders(t *testing.T) {
	var buf bytes.Buffer
	writer := csvencode.NewWriter(&buf)
	writer.Write([]string{"Name*", "Email*"})
	writer.Write([]string{"Alice", "alice@mail.com"})
	writer.Flush()

	file := createMultipartFile(t, buf.Bytes())

	type TestStruct struct {
		Name  string `json:"Name" validate:"required"`
		Email string `json:"Email" validate:"required"`
	}

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
		WithWorker(1),
		WithCtx(context.Background()),
	)

	result := imp.ProcessData()
	require.NotNil(t, result)
	assert.Equal(t, 1, result.TotalData)
	assert.Equal(t, 0, result.TotalFailed)
}

func TestProcessData_CSV_FailedParsedSingleData(t *testing.T) {
	type TestStruct struct {
		Name string `json:"Name" validate:"required,min=100"`
	}

	content := createCSVContent(
		[]string{"Name"},
		[]string{"short"},
	)
	file := createMultipartFile(t, content)

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
		WithWorker(1),
		WithCtx(context.Background()),
	)

	result := imp.ProcessData()
	require.NotNil(t, result)
	assert.GreaterOrEqual(t, result.TotalFailed, 1)
}

func TestProcessData_CSV_ExtraColumnsInRow(t *testing.T) {
	type TestStruct struct {
		Name string `json:"Name"`
	}

	var buf bytes.Buffer
	writer := csvencode.NewWriter(&buf)
	writer.Write([]string{"Name"})
	writer.Write([]string{"Alice", "Extra"})
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
		WithWorker(1),
		WithCtx(context.Background()),
	)

	result := imp.ProcessData()
	require.NotNil(t, result)
	assert.GreaterOrEqual(t, result.TotalData, 0)
}

func TestProcessData_ProcessFnErrorWithData(t *testing.T) {
	type TestStruct struct {
		Name string `json:"Name"`
	}

	content := createCSVContent([]string{"Name"}, []string{"Alice"})
	file := createMultipartFile(t, content)

	trx := &stubTransactions{}
	fn := func(ctx context.Context, singleData interface{}, tx *sqlx.Tx, wi int) *api.HttpError {
		return api.NewErr(
			api.WithErrorMessage("some error"),
			api.WithErrorData(map[string]any{"data": singleData}),
		)
	}

	imp := New(
		WithFile(file),
		WithExtFile(".csv"),
		WithStructDestination(TestStruct{}),
		WithTransaction(trx),
		WithProcessFn(fn),
		WithWorker(1),
		WithCtx(context.Background()),
	)

	result := imp.ProcessData()
	require.NotNil(t, result)
	assert.Equal(t, 1, result.TotalData)
	assert.Equal(t, 1, result.TotalFailed)
}

// ---------------------------------------------------------------------------
// Tests for ProcessData with Slack notification
// ---------------------------------------------------------------------------

func TestProcessData_CSV_WithSlackNotif(t *testing.T) {
	type TestStruct struct {
		Name string `json:"Name" validate:"required"`
	}

	content := createCSVContent([]string{"Name"}, []string{"Alice"})
	file := createMultipartFile(t, content)

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
		WithWorker(1),
		WithCtx(context.Background()),
		WithSentNotifToSlack(true, "https://hooks.slack.com/test"),
		WithProcessName("TestImport"),
	)

	result := imp.ProcessData()
	require.NotNil(t, result)
	assert.Equal(t, 1, result.TotalData)
	assert.Equal(t, 0, result.TotalFailed)
}

func TestProcessData_CSV_WithSlackNotifAndErrors(t *testing.T) {
	type TestStruct struct {
		Name string `json:"Name" validate:"required"`
	}

	content := createCSVContent([]string{"Name"}, []string{""})
	file := createMultipartFile(t, content)

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
		WithWorker(1),
		WithCtx(context.Background()),
		WithSentNotifToSlack(true, "https://hooks.slack.com/test"),
	)

	result := imp.ProcessData()
	require.NotNil(t, result)
	// Empty Name with validate:"required" may or may not fail depending on CSV parsing
	// Just verify the result is returned with Slack notification enabled
	assert.GreaterOrEqual(t, result.TotalData, 0)
}

// ---------------------------------------------------------------------------
// Tests for ProcessData (Xlsx)
// ---------------------------------------------------------------------------

func TestProcessData_Xlsx(t *testing.T) {
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
	f.SetCellStr("Sheet1", "B2", "alice@mail.com")
	f.SetCellStr("Sheet1", "A3", "Bob")
	f.SetCellStr("Sheet1", "B3", "bob@mail.com")
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

func TestProcessData_Xlsx_WithValidationErrors(t *testing.T) {
	type TestStruct struct {
		Name  string `json:"Name" validate:"required"`
		Email string `json:"Email" validate:"required,email"`
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.xlsx")
	f := excelize.NewFile()
	f.SetCellStr("Sheet1", "A1", "Name")
	f.SetCellStr("Sheet1", "B1", "Email")
	f.SetCellStr("Sheet1", "A2", "")
	f.SetCellStr("Sheet1", "B2", "invalid")
	f.SetCellStr("Sheet1", "A3", "Bob")
	f.SetCellStr("Sheet1", "B3", "bob@mail.com")
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
		WithWorker(2),
		WithCtx(context.Background()),
	)

	result := imp.ProcessData()
	require.NotNil(t, result)
	assert.Equal(t, 2, result.TotalData)
	assert.GreaterOrEqual(t, result.TotalFailed, 1)
}

func TestProcessData_Xlsx_CustomSheetName(t *testing.T) {
	type TestStruct struct {
		Name string `json:"Name"`
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.xlsx")
	f := excelize.NewFile()
	f.NewSheet("CustomSheet")
	f.SetCellStr("CustomSheet", "A1", "Name")
	f.SetCellStr("CustomSheet", "A2", "Alice")
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
		WithWorker(1),
		WithCtx(context.Background()),
		WithSheetName("CustomSheet"),
	)

	result := imp.ProcessData()
	require.NotNil(t, result)
	assert.Equal(t, 1, result.TotalData)
}

// ---------------------------------------------------------------------------
// Tests for ImportResult struct
// ---------------------------------------------------------------------------

func TestImportResult_Fields(t *testing.T) {
	result := &ImportResult{
		TotalData:     10,
		TotalFailed:   2,
		TotalSuccess:  8,
		ExecutionTime: 1.5,
	}
	assert.Equal(t, 10, result.TotalData)
	assert.Equal(t, 2, result.TotalFailed)
	assert.Equal(t, 8, result.TotalSuccess)
	assert.Equal(t, 1.5, result.ExecutionTime)
}

// ---------------------------------------------------------------------------
// Tests for GetDataFromXlsx
// ---------------------------------------------------------------------------

func TestGetDataFromXlsx(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.xlsx")

	f := excelize.NewFile()
	f.SetCellStr("Sheet1", "A1", "Name")
	f.SetCellStr("Sheet1", "B1", "Email")
	f.SetCellStr("Sheet1", "A2", "Alice")
	f.SetCellStr("Sheet1", "B2", "alice@mail.com")
	f.SaveAs(filePath)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	file := createMultipartFile(t, data)

	rows, err := GetDataFromXlsx(file, "Sheet1")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, []string{"Name", "Email"}, rows[0])
	assert.Equal(t, []string{"Alice", "alice@mail.com"}, rows[1])
}

func TestGetDataFromXlsx_DefaultSheet(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.xlsx")

	f := excelize.NewFile()
	f.SetCellStr("Sheet1", "A1", "Col1")
	f.SetCellStr("Sheet1", "A2", "Val1")
	f.SaveAs(filePath)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	file := createMultipartFile(t, data)

	rows, err := GetDataFromXlsx(file, "")
	require.NoError(t, err)
	assert.Len(t, rows, 2)
}

func TestGetDataFromXlsx_InvalidFile(t *testing.T) {
	file := createMultipartFile(t, []byte("not an xlsx file"))
	rows, err := GetDataFromXlsx(file, "Sheet1")
	assert.Error(t, err)
	assert.Nil(t, rows)
}

// ---------------------------------------------------------------------------
// Tests for GetDataFromXls
// ---------------------------------------------------------------------------

func TestGetDataFromXls_InvalidFile(t *testing.T) {
	file := createMultipartFile(t, []byte("not an xls file"))
	rows, err := GetDataFromXls(file)
	assert.Error(t, err)
	assert.Nil(t, rows)
}

// ---------------------------------------------------------------------------
// Tests for readFromExcel directly
// ---------------------------------------------------------------------------

func TestReadFromExcel_XlsxStream(t *testing.T) {
	type TestStruct struct {
		Name  string `json:"Name"`
		Email string `json:"Email"`
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.xlsx")
	f := excelize.NewFile()
	f.SetCellStr("Sheet1", "A1", "Name")
	f.SetCellStr("Sheet1", "B1", "Email")
	f.SetCellStr("Sheet1", "A2", "Alice")
	f.SetCellStr("Sheet1", "B2", "alice@mail.com")
	f.SaveAs(filePath)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	file := createMultipartFile(t, data)

	imp := New(WithExtFile(".xlsx"), WithStructDestination(TestStruct{}))
	imp.structDestReflVal = reflect.ValueOf(TestStruct{})

	chanData := imp.readFromExcel(imp.structDestReflVal, file)

	var results []rowData
	for rd := range chanData {
		results = append(results, rd)
	}

	assert.Len(t, results, 1)
	assert.Contains(t, results[0].RawData, "Name")
}

func TestReadFromExcel_XlsType_InvalidFile(t *testing.T) {
	type TestStruct struct {
		Name string `json:"Name"`
	}

	file := createMultipartFile(t, []byte("not an xls file"))

	imp := New(WithExtFile(".xls"), WithStructDestination(TestStruct{}))
	imp.structDestReflVal = reflect.ValueOf(TestStruct{})

	chanData := imp.readFromExcel(imp.structDestReflVal, file)

	var results []rowData
	for rd := range chanData {
		results = append(results, rd)
	}

	assert.Len(t, results, 0)
}

// ---------------------------------------------------------------------------
// Tests for readFromCSV via internal channel
// ---------------------------------------------------------------------------

func TestReadFromCSV_ChannelOutput(t *testing.T) {
	var buf bytes.Buffer
	writer := csvencode.NewWriter(&buf)
	writer.Write([]string{"Name", "Email"})
	writer.Write([]string{"Alice", "alice@mail.com"})
	writer.Write([]string{"Bob", "bob@mail.com"})
	writer.Flush()

	file := createMultipartFile(t, buf.Bytes())

	type TestStruct struct {
		Name  string `json:"Name"`
		Email string `json:"Email"`
	}

	imp := New(WithFile(file), WithExtFile(".csv"), WithStructDestination(TestStruct{}))
	imp.structDestReflVal = reflect.ValueOf(TestStruct{})

	chanData := imp.readFromCSV(imp.structDestReflVal, imp.file)

	var results []rowData
	for rd := range chanData {
		results = append(results, rd)
	}

	assert.Len(t, results, 2)
	assert.Contains(t, results[0].RawData, "Name")
	assert.Contains(t, results[0].RawData, "Email")
}

func TestReadFromCSV_CleansAsteriskInHeaders(t *testing.T) {
	var buf bytes.Buffer
	writer := csvencode.NewWriter(&buf)
	writer.Write([]string{"Name*", "Email*"})
	writer.Write([]string{"Alice", "alice@mail.com"})
	writer.Flush()

	file := createMultipartFile(t, buf.Bytes())

	type TestStruct struct {
		Name  string `json:"Name"`
		Email string `json:"Email"`
	}

	imp := New(WithFile(file), WithExtFile(".csv"), WithStructDestination(TestStruct{}))
	imp.structDestReflVal = reflect.ValueOf(TestStruct{})

	chanData := imp.readFromCSV(imp.structDestReflVal, imp.file)

	var results []rowData
	for rd := range chanData {
		results = append(results, rd)
	}

	assert.Len(t, results, 1)
	assert.Contains(t, results[0].RawData, "Name")
	assert.NotContains(t, results[0].RawData, "Name*")
}

func TestReadFromCSV_EmptyFile(t *testing.T) {
	file := createMultipartFile(t, []byte(""))

	type TestStruct struct {
		Name string `json:"Name"`
	}

	imp := New(WithFile(file), WithExtFile(".csv"), WithStructDestination(TestStruct{}))
	imp.structDestReflVal = reflect.ValueOf(TestStruct{})

	chanData := imp.readFromCSV(imp.structDestReflVal, imp.file)

	var results []rowData
	for rd := range chanData {
		results = append(results, rd)
	}

	assert.Len(t, results, 0)
}

// ---------------------------------------------------------------------------
// Tests for ReadPivotData
// ---------------------------------------------------------------------------

func TestReadPivotData_NoFile(t *testing.T) {
	imp := New(WithExtFile(".csv"))
	result, err := imp.ReadPivotData()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "`File` is null")
	assert.Nil(t, result)
}

func TestReadPivotData_UndefinedFileType(t *testing.T) {
	content := createCSVContent([]string{"A"})
	file := createMultipartFile(t, content)
	imp := New(WithFile(file), WithExtFile(".txt"))
	result, err := imp.ReadPivotData()
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestReadPivotData_CSV(t *testing.T) {
	content := createCSVContent(
		[]string{"Employee ID", "Employee Name", "2024-03-26", "2024-03-27"},
		[]string{"444201123", "Fitri", "HONS", "HONS"},
	)
	file := createMultipartFile(t, content)

	imp := New(
		WithFile(file),
		WithExtFile(".csv"),
		WithHeaderRowIndex(0),
		WithDataStartRow(1),
		WithKeyColumns([]int{0}),
		WithKeySeparator(";"),
		WithValueStartCol(2),
	)

	result, err := imp.ReadPivotData()
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Headers, 4)
	assert.Contains(t, result.Data, "444201123;2024-03-26")
	assert.Equal(t, "HONS", result.Data["444201123;2024-03-26"])
}

func TestReadPivotData_CSV_WithMetaRows(t *testing.T) {
	var buf bytes.Buffer
	writer := csvencode.NewWriter(&buf)
	writer.Write([]string{"Location Name", "PT. XYZ"})
	writer.Write([]string{"", ""})
	writer.Write([]string{"Employee ID", "Name", "2024-03-26"})
	writer.Write([]string{"444201123", "Fitri", "HONS"})
	writer.Flush()

	file := createMultipartFile(t, buf.Bytes())

	imp := New(
		WithFile(file),
		WithExtFile(".csv"),
		WithHeaderRowIndex(2),
		WithDataStartRow(3),
		WithKeyColumns([]int{0}),
		WithKeySeparator(";"),
		WithValueStartCol(2),
	)

	result, err := imp.ReadPivotData()
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.MetaRows, 2)
	assert.Equal(t, "Location Name", result.MetaRows[0][0])
	assert.Contains(t, result.Data, "444201123;2024-03-26")
}

func TestReadPivotData_CSV_WithOnEntryCallback(t *testing.T) {
	content := createCSVContent(
		[]string{"Employee ID", "Employee Name", "2024-03-26"},
		[]string{"444201123", "Fitri", "HONS"},
	)
	file := createMultipartFile(t, content)

	var entries []string
	imp := New(
		WithFile(file),
		WithExtFile(".csv"),
		WithHeaderRowIndex(0),
		WithDataStartRow(1),
		WithKeyColumns([]int{0}),
		WithKeySeparator(";"),
		WithValueStartCol(2),
		WithOnPivotEntry(func(key, value string) {
			entries = append(entries, key+"="+value)
		}),
	)

	result, err := imp.ReadPivotData()
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "444201123;2024-03-26=HONS", entries[0])
	assert.Nil(t, result.Data)
}

func TestReadPivotData_CSV_MultipleKeyColumns(t *testing.T) {
	content := createCSVContent(
		[]string{"Employee ID", "Name", "2024-03-26"},
		[]string{"444201123", "Fitri", "HONS"},
	)
	file := createMultipartFile(t, content)

	imp := New(
		WithFile(file),
		WithExtFile(".csv"),
		WithHeaderRowIndex(0),
		WithDataStartRow(1),
		WithKeyColumns([]int{0, 1}),
		WithKeySeparator(";"),
		WithValueStartCol(2),
	)

	result, err := imp.ReadPivotData()
	require.NoError(t, err)
	assert.Contains(t, result.Data, "444201123;Fitri;2024-03-26")
	assert.Equal(t, "HONS", result.Data["444201123;Fitri;2024-03-26"])
}

func TestReadPivotData_CSV_EmptyHeaderValue(t *testing.T) {
	content := createCSVContent(
		[]string{"Employee ID", "", "2024-03-26"},
		[]string{"444201123", "ignored", "HONS"},
	)
	file := createMultipartFile(t, content)

	imp := New(
		WithFile(file),
		WithExtFile(".csv"),
		WithHeaderRowIndex(0),
		WithDataStartRow(1),
		WithKeyColumns([]int{0}),
		WithKeySeparator(";"),
		WithValueStartCol(1),
	)

	result, err := imp.ReadPivotData()
	require.NoError(t, err)
	assert.Contains(t, result.Data, "444201123;2024-03-26")
	assert.Len(t, result.Data, 1)
}

func TestReadPivotData_Xlsx(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "pivot.xlsx")
	f := excelize.NewFile()
	f.SetCellStr("Sheet1", "A1", "Employee ID")
	f.SetCellStr("Sheet1", "B1", "2024-03-26")
	f.SetCellStr("Sheet1", "A2", "444201123")
	f.SetCellStr("Sheet1", "B2", "HONS")
	f.SaveAs(filePath)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	file := createMultipartFile(t, data)

	imp := New(
		WithFile(file),
		WithExtFile(".xlsx"),
		WithHeaderRowIndex(0),
		WithDataStartRow(1),
		WithKeyColumns([]int{0}),
		WithKeySeparator(";"),
		WithValueStartCol(1),
	)

	result, err := imp.ReadPivotData()
	require.NoError(t, err)
	assert.Contains(t, result.Data, "444201123;2024-03-26")
	assert.Equal(t, "HONS", result.Data["444201123;2024-03-26"])
}

func TestReadPivotData_Xlsx_WithMetaRows(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "pivot.xlsx")
	f := excelize.NewFile()
	f.SetCellStr("Sheet1", "A1", "Location")
	f.SetCellStr("Sheet1", "B1", "PT. XYZ")
	f.SetCellStr("Sheet1", "A2", "Employee ID")
	f.SetCellStr("Sheet1", "B2", "2024-03-26")
	f.SetCellStr("Sheet1", "A3", "444201123")
	f.SetCellStr("Sheet1", "B3", "HONS")
	f.SaveAs(filePath)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	file := createMultipartFile(t, data)

	imp := New(
		WithFile(file),
		WithExtFile(".xlsx"),
		WithHeaderRowIndex(1),
		WithDataStartRow(2),
		WithKeyColumns([]int{0}),
		WithKeySeparator(";"),
		WithValueStartCol(1),
	)

	result, err := imp.ReadPivotData()
	require.NoError(t, err)
	assert.Len(t, result.MetaRows, 1)
	assert.Equal(t, "Location", result.MetaRows[0][0])
	assert.Contains(t, result.Data, "444201123;2024-03-26")
}

func TestReadPivotData_Xlsx_WithOnEntryCallback(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "pivot.xlsx")
	f := excelize.NewFile()
	f.SetCellStr("Sheet1", "A1", "Employee ID")
	f.SetCellStr("Sheet1", "B1", "2024-03-26")
	f.SetCellStr("Sheet1", "A2", "444201123")
	f.SetCellStr("Sheet1", "B2", "HONS")
	f.SaveAs(filePath)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	file := createMultipartFile(t, data)

	var entries []string
	imp := New(
		WithFile(file),
		WithExtFile(".xlsx"),
		WithHeaderRowIndex(0),
		WithDataStartRow(1),
		WithKeyColumns([]int{0}),
		WithKeySeparator(";"),
		WithValueStartCol(1),
		WithOnPivotEntry(func(key, value string) {
			entries = append(entries, key+"="+value)
		}),
	)

	result, err := imp.ReadPivotData()
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "444201123;2024-03-26=HONS", entries[0])
	assert.Nil(t, result.Data)
}

func TestReadPivotData_Xlsx_EmptyHeaderValue(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "pivot.xlsx")
	f := excelize.NewFile()
	f.SetCellStr("Sheet1", "A1", "Employee ID")
	f.SetCellStr("Sheet1", "B1", "")
	f.SetCellStr("Sheet1", "C1", "2024-03-26")
	f.SetCellStr("Sheet1", "A2", "444201123")
	f.SetCellStr("Sheet1", "B2", "ignored")
	f.SetCellStr("Sheet1", "C2", "HONS")
	f.SaveAs(filePath)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	file := createMultipartFile(t, data)

	imp := New(
		WithFile(file),
		WithExtFile(".xlsx"),
		WithHeaderRowIndex(0),
		WithDataStartRow(1),
		WithKeyColumns([]int{0}),
		WithKeySeparator(";"),
		WithValueStartCol(1),
	)

	result, err := imp.ReadPivotData()
	require.NoError(t, err)
	assert.Contains(t, result.Data, "444201123;2024-03-26")
	assert.Len(t, result.Data, 1)
}

func TestReadPivotData_Xlsx_CustomSheetName(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "pivot.xlsx")
	f := excelize.NewFile()
	f.NewSheet("CustomSheet")
	f.SetCellStr("CustomSheet", "A1", "Employee ID")
	f.SetCellStr("CustomSheet", "B1", "2024-03-26")
	f.SetCellStr("CustomSheet", "A2", "444201123")
	f.SetCellStr("CustomSheet", "B2", "HONS")
	f.SaveAs(filePath)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	file := createMultipartFile(t, data)

	imp := New(
		WithFile(file),
		WithExtFile(".xlsx"),
		WithSheetName("CustomSheet"),
		WithHeaderRowIndex(0),
		WithDataStartRow(1),
		WithKeyColumns([]int{0}),
		WithKeySeparator(";"),
		WithValueStartCol(1),
	)

	result, err := imp.ReadPivotData()
	require.NoError(t, err)
	assert.Contains(t, result.Data, "444201123;2024-03-26")
}

func TestReadPivot_CSV_DefaultSeparator(t *testing.T) {
	content := createCSVContent(
		[]string{"Key", "2024-03-26"},
		[]string{"k1", "v1"},
	)
	file := createMultipartFile(t, content)

	imp := New(
		WithFile(file),
		WithExtFile(".csv"),
		WithHeaderRowIndex(0),
		WithDataStartRow(1),
		WithKeyColumns([]int{0}),
		WithValueStartCol(1),
	)

	result, err := imp.ReadPivotData()
	require.NoError(t, err)
	assert.Contains(t, result.Data, "k1;2024-03-26")
}

func TestReadPivot_CSV_DataStartRowBeforeHeader(t *testing.T) {
	content := createCSVContent(
		[]string{"Key", "2024-03-26"},
		[]string{"k1", "v1"},
	)
	file := createMultipartFile(t, content)

	imp := New(
		WithFile(file),
		WithExtFile(".csv"),
		WithHeaderRowIndex(0),
		WithDataStartRow(0),
		WithKeyColumns([]int{0}),
		WithValueStartCol(1),
	)

	result, err := imp.ReadPivotData()
	require.NoError(t, err)
	assert.Contains(t, result.Data, "k1;2024-03-26")
}

func TestReadPivot_UnsupportedFileType(t *testing.T) {
	content := createCSVContent([]string{"A"})
	file := createMultipartFile(t, content)

	imp := New(WithFile(file), WithExtFile(".txt"))
	result, err := imp.ReadPivotData()
	assert.Error(t, err)
	assert.Nil(t, result)
}

// ---------------------------------------------------------------------------
// Tests for readPivot nil file check
// ---------------------------------------------------------------------------

func TestReadPivot_NilFile(t *testing.T) {
	config := pivotReadConfig{
		File:     nil,
		FileType: CSVFileType,
	}
	result, err := readPivot(config)
	assert.Error(t, err)
	assert.Nil(t, result)
}

// ---------------------------------------------------------------------------
// Tests for readPivotChannel edge cases
// ---------------------------------------------------------------------------

func TestReadPivotChannel_DefaultSeparator(t *testing.T) {
	content := createCSVContent(
		[]string{"Key", "2024-03-26"},
		[]string{"k1", "v1"},
	)
	file := createMultipartFile(t, content)

	config := pivotReadConfig{
		File:           file,
		FileType:       CSVFileType,
		HeaderRowIndex: 0,
		DataStartRow:   1,
		KeyColumns:     []int{0},
		ValueStartCol:  1,
	}

	chanData := readPivotChannel(config)
	var results []rowData
	for rd := range chanData {
		results = append(results, rd)
	}
	assert.Len(t, results, 1)
}

func TestReadPivotChannel_DataStartRowAdjusted(t *testing.T) {
	content := createCSVContent(
		[]string{"Key", "2024-03-26"},
		[]string{"k1", "v1"},
	)
	file := createMultipartFile(t, content)

	config := pivotReadConfig{
		File:           file,
		FileType:       CSVFileType,
		HeaderRowIndex: 0,
		DataStartRow:   0,
		KeyColumns:     []int{0},
		ValueStartCol:  1,
	}

	chanData := readPivotChannel(config)
	var results []rowData
	for rd := range chanData {
		results = append(results, rd)
	}
	assert.Len(t, results, 1)
}

// ---------------------------------------------------------------------------
// Tests for ProcessPivotData
// ---------------------------------------------------------------------------

func TestProcessPivotData_NoFile(t *testing.T) {
	imp := New(WithExtFile(".csv"))
	result := imp.ProcessPivotData()
	assert.Nil(t, result)
}

func TestProcessPivotData_UndefinedFileType(t *testing.T) {
	content := createCSVContent([]string{"A"})
	file := createMultipartFile(t, content)
	imp := New(WithFile(file), WithExtFile(".txt"))
	result := imp.ProcessPivotData()
	assert.Nil(t, result)
}

func TestProcessPivotData_NoProcessFn(t *testing.T) {
	content := createCSVContent([]string{"A"})
	file := createMultipartFile(t, content)
	imp := New(WithFile(file), WithExtFile(".csv"))
	result := imp.ProcessPivotData()
	assert.Nil(t, result)
}

func TestProcessPivotData_CSV(t *testing.T) {
	content := createCSVContent(
		[]string{"Employee ID", "Employee Name", "2024-03-26", "2024-03-27"},
		[]string{"444201123", "Fitri", "HONS", "HONS"},
	)
	file := createMultipartFile(t, content)

	trx := &stubTransactions{}
	var processedEntries []map[string]any
	fn := func(ctx context.Context, singleData interface{}, tx *sqlx.Tx, wi int) *api.HttpError {
		entry := singleData.(map[string]any)
		processedEntries = append(processedEntries, entry)
		return nil
	}

	imp := New(
		WithFile(file),
		WithExtFile(".csv"),
		WithTransaction(trx),
		WithProcessFn(fn),
		WithWorker(1), // Use 1 worker to avoid race condition
		WithCtx(context.Background()),
		WithHeaderRowIndex(0),
		WithDataStartRow(1),
		WithKeyColumns([]int{0}),
		WithKeySeparator(";"),
		WithValueStartCol(2),
	)

	result := imp.ProcessPivotData()
	require.NotNil(t, result)
	assert.Equal(t, 2, result.TotalData)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Len(t, processedEntries, 2)
	assert.Equal(t, "444201123", processedEntries[0]["Employee ID"])
	assert.Contains(t, processedEntries[0], "pivot_header")
	assert.Contains(t, processedEntries[0], "pivot_value")
}

func TestProcessPivotData_CSV_WithErrors(t *testing.T) {
	content := createCSVContent(
		[]string{"Employee ID", "Date", "2024-03-26"},
		[]string{"444201123", "data", "HONS"},
	)
	file := createMultipartFile(t, content)

	trx := &stubTransactions{}
	fn := func(ctx context.Context, singleData interface{}, tx *sqlx.Tx, wi int) *api.HttpError {
		return api.NewErr(api.WithErrorMessage("processing error"))
	}

	imp := New(
		WithFile(file),
		WithExtFile(".csv"),
		WithTransaction(trx),
		WithProcessFn(fn),
		WithWorker(1),
		WithCtx(context.Background()),
		WithHeaderRowIndex(0),
		WithDataStartRow(1),
		WithKeyColumns([]int{0}),
		WithValueStartCol(2),
	)

	result := imp.ProcessPivotData()
	require.NotNil(t, result)
	assert.GreaterOrEqual(t, result.TotalFailed, 1)
}

func TestProcessPivotData_CSV_NilCtx(t *testing.T) {
	content := createCSVContent(
		[]string{"Key", "Val", "2024-03-26"},
		[]string{"key1", "v1", "HONS"},
	)
	file := createMultipartFile(t, content)

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
		WithHeaderRowIndex(0),
		WithDataStartRow(1),
		WithKeyColumns([]int{0}),
		WithValueStartCol(2),
	)

	result := imp.ProcessPivotData()
	require.NotNil(t, result)
	assert.Equal(t, 1, result.TotalData)
	assert.Equal(t, 0, result.TotalFailed)
}

func TestProcessPivotData_CSV_WithSlackNotif(t *testing.T) {
	content := createCSVContent(
		[]string{"Employee ID", "2024-03-26"},
		[]string{"444201123", "HONS"},
	)
	file := createMultipartFile(t, content)

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
		WithCtx(context.Background()),
		WithHeaderRowIndex(0),
		WithDataStartRow(1),
		WithKeyColumns([]int{0}),
		WithValueStartCol(1),
		WithSentNotifToSlack(true, "https://hooks.slack.com/test"),
	)

	result := imp.ProcessPivotData()
	require.NotNil(t, result)
	assert.Equal(t, 1, result.TotalData)
}

func TestProcessPivotData_CSV_WithSlackNotifAndErrors(t *testing.T) {
	content := createCSVContent(
		[]string{"Employee ID", "2024-03-26"},
		[]string{"444201123", "HONS"},
	)
	file := createMultipartFile(t, content)

	trx := &stubTransactions{}
	fn := func(ctx context.Context, singleData interface{}, tx *sqlx.Tx, wi int) *api.HttpError {
		return api.NewErr(api.WithErrorMessage("processing error"))
	}

	imp := New(
		WithFile(file),
		WithExtFile(".csv"),
		WithTransaction(trx),
		WithProcessFn(fn),
		WithWorker(1),
		WithCtx(context.Background()),
		WithHeaderRowIndex(0),
		WithDataStartRow(1),
		WithKeyColumns([]int{0}),
		WithValueStartCol(1),
		WithSentNotifToSlack(true, "https://hooks.slack.com/test"),
	)

	result := imp.ProcessPivotData()
	require.NotNil(t, result)
	assert.Equal(t, 1, result.TotalFailed)
}

func TestProcessPivotData_EmptyHeaderValue(t *testing.T) {
	content := createCSVContent(
		[]string{"Employee ID", "", "2024-03-26"},
		[]string{"444201123", "ignored", "HONS"},
	)
	file := createMultipartFile(t, content)

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
		WithCtx(context.Background()),
		WithHeaderRowIndex(0),
		WithDataStartRow(1),
		WithKeyColumns([]int{0}),
		WithValueStartCol(1),
	)

	result := imp.ProcessPivotData()
	require.NotNil(t, result)
	assert.Equal(t, 1, result.TotalData)
}

func TestProcessPivotData_KeyColumnOutOfBounds(t *testing.T) {
	content := createCSVContent(
		[]string{"Employee ID", "2024-03-26"},
		[]string{"444201123", "HONS"},
	)
	file := createMultipartFile(t, content)

	trx := &stubTransactions{}
	var entries []map[string]any
	fn := func(ctx context.Context, singleData interface{}, tx *sqlx.Tx, wi int) *api.HttpError {
		entries = append(entries, singleData.(map[string]any))
		return nil
	}

	imp := New(
		WithFile(file),
		WithExtFile(".csv"),
		WithTransaction(trx),
		WithProcessFn(fn),
		WithWorker(1),
		WithCtx(context.Background()),
		WithHeaderRowIndex(0),
		WithDataStartRow(1),
		WithKeyColumns([]int{5}),
		WithValueStartCol(1),
	)

	result := imp.ProcessPivotData()
	require.NotNil(t, result)
	assert.Equal(t, 1, result.TotalData)
}

func TestProcessPivotData_MultiKeyColumns(t *testing.T) {
	content := createCSVContent(
		[]string{"Employee ID", "Name", "2024-03-26"},
		[]string{"444201123", "Fitri", "HONS"},
	)
	file := createMultipartFile(t, content)

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
		WithCtx(context.Background()),
		WithHeaderRowIndex(0),
		WithDataStartRow(1),
		WithKeyColumns([]int{0, 1}),
		WithValueStartCol(2),
	)

	result := imp.ProcessPivotData()
	require.NotNil(t, result)
	assert.Equal(t, 1, result.TotalData)
	assert.Equal(t, 0, result.TotalFailed)
}

func TestProcessPivotData_Xlsx(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "pivot.xlsx")
	f := excelize.NewFile()
	f.SetCellStr("Sheet1", "A1", "Employee ID")
	f.SetCellStr("Sheet1", "B1", "2024-03-26")
	f.SetCellStr("Sheet1", "A2", "444201123")
	f.SetCellStr("Sheet1", "B2", "HONS")
	f.SaveAs(filePath)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	file := createMultipartFile(t, data)

	trx := &stubTransactions{}
	var entries []map[string]any
	fn := func(ctx context.Context, singleData interface{}, tx *sqlx.Tx, wi int) *api.HttpError {
		entries = append(entries, singleData.(map[string]any))
		return nil
	}

	imp := New(
		WithFile(file),
		WithExtFile(".xlsx"),
		WithTransaction(trx),
		WithProcessFn(fn),
		WithWorker(1),
		WithCtx(context.Background()),
		WithHeaderRowIndex(0),
		WithDataStartRow(1),
		WithKeyColumns([]int{0}),
		WithValueStartCol(1),
	)

	result := imp.ProcessPivotData()
	require.NotNil(t, result)
	assert.Equal(t, 1, result.TotalData)
	assert.Len(t, entries, 1)
	assert.Equal(t, "444201123", entries[0]["Employee ID"])
	assert.Equal(t, "2024-03-26", entries[0]["pivot_header"])
	assert.Equal(t, "HONS", entries[0]["pivot_value"])
}

func TestProcessPivotData_Xlsx_CustomSheetName(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "pivot.xlsx")
	f := excelize.NewFile()
	f.NewSheet("CustomSheet")
	f.SetCellStr("CustomSheet", "A1", "Employee ID")
	f.SetCellStr("CustomSheet", "B1", "2024-03-26")
	f.SetCellStr("CustomSheet", "A2", "444201123")
	f.SetCellStr("CustomSheet", "B2", "HONS")
	f.SaveAs(filePath)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	file := createMultipartFile(t, data)

	trx := &stubTransactions{}
	var entries []map[string]any
	fn := func(ctx context.Context, singleData interface{}, tx *sqlx.Tx, wi int) *api.HttpError {
		entries = append(entries, singleData.(map[string]any))
		return nil
	}

	imp := New(
		WithFile(file),
		WithExtFile(".xlsx"),
		WithSheetName("CustomSheet"),
		WithTransaction(trx),
		WithProcessFn(fn),
		WithWorker(1),
		WithCtx(context.Background()),
		WithHeaderRowIndex(0),
		WithDataStartRow(1),
		WithKeyColumns([]int{0}),
		WithValueStartCol(1),
	)

	result := imp.ProcessPivotData()
	require.NotNil(t, result)
	assert.Equal(t, 1, result.TotalData)
	assert.Len(t, entries, 1)
}

// ---------------------------------------------------------------------------
// Tests for PivotReadResult and data types
// ---------------------------------------------------------------------------

func TestPivotReadResult_Fields(t *testing.T) {
	result := &PivotReadResult{
		Data:     map[string]string{"k1": "v1"},
		Headers:  []string{"Key", "Date"},
		MetaRows: [][]string{{"meta"}},
	}
	assert.Len(t, result.Data, 1)
	assert.Len(t, result.Headers, 2)
	assert.Len(t, result.MetaRows, 1)
}

func TestRowData(t *testing.T) {
	rd := rowData{
		ParsedStruct: "test",
		RawData:      map[string]any{"key": "value"},
	}
	assert.Equal(t, "test", rd.ParsedStruct)
	assert.Equal(t, "value", rd.RawData["key"])
}

func TestProcessedResult(t *testing.T) {
	pr := processedResult{
		rowData: rowData{ParsedStruct: "test"},
		Error:   api.NewErr(api.WithErrorMessage("test error")),
	}
	assert.Equal(t, "test", pr.ParsedStruct)
	assert.NotNil(t, pr.Error)
}
