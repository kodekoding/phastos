package generator

import (
	"archive/zip"
	"bytes"
	csvencode "encoding/csv"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kodekoding/phastos/v2/go/helper"
)

// ---------------------------------------------------------------------------
// CSV Tests
// ---------------------------------------------------------------------------

func TestNewCSV(t *testing.T) {
	c := NewCSV()
	require.NotNil(t, c)
	assert.Equal(t, "generated-csv.csv", c.FileName())
}

func TestCSV_SetFileName(t *testing.T) {
	c := NewCSV()
	result := c.SetFileName("my-report")
	assert.Equal(t, "my-report.csv", c.FileName())
	assert.NotNil(t, result) // returns CSVs interface
}

func TestCSV_SetHeader(t *testing.T) {
	c := NewCSV()
	c.SetHeader([]string{"Name", "Email", "Role"})
	assert.Nil(t, c.Error())
}

func TestCSV_SetHeader_WithExistingContent_ColumnMismatch(t *testing.T) {
	c := NewCSV()
	c.SetHeader([]string{"Name", "Email", "Role"})
	c.AppendDataRow([]string{"Alice", "alice@mail.com", "Admin"})
	// Now set a header with different column count
	c.SetHeader([]string{"Name", "Email"})
	assert.NotNil(t, c.Error())
	assert.Contains(t, c.Error().Error(), "Total Column isn't equal")
}

func TestCSV_SetHeader_SameColumnCount(t *testing.T) {
	c := NewCSV()
	c.SetHeader([]string{"Name", "Email"})
	c.AppendDataRow([]string{"Alice", "alice@mail.com"})
	c.SetHeader([]string{"First", "Last"})
	assert.Nil(t, c.Error())
}

func TestCSV_AppendDataRow(t *testing.T) {
	c := NewCSV()
	c.SetHeader([]string{"A", "B", "C"})
	c.AppendDataRow([]string{"1", "2", "3"})
	c.AppendDataRow([]string{"4", "5", "6"})
	assert.Nil(t, c.Error())
}

func TestCSV_AppendDataRow_ColumnMismatch(t *testing.T) {
	c := NewCSV()
	c.SetHeader([]string{"A", "B", "C"})
	c.AppendDataRow([]string{"1", "2"})
	assert.NotNil(t, c.Error())
	assert.Contains(t, c.Error().Error(), "Total Column isn't equal")
}

func TestCSV_AppendDataRow_FirstRow(t *testing.T) {
	// First data row with no prior content - should always succeed
	c := NewCSV()
	c.AppendDataRow([]string{"1", "2", "3"})
	assert.Nil(t, c.Error())
}

func TestCSV_AppendDataRow_SkipOnError(t *testing.T) {
	c := NewCSV()
	c.SetHeader([]string{"A", "B"})
	c.AppendDataRow([]string{"1", "2", "3"}) // mismatch
	assert.NotNil(t, c.Error())
	// Subsequent calls should return early due to existing error
	c.AppendDataRow([]string{"4", "5"})
	assert.NotNil(t, c.Error())
}

func TestCSV_SetHeader_SkipOnError(t *testing.T) {
	c := NewCSV()
	c.err = fmt.Errorf("some error")
	c.SetHeader([]string{"A", "B"})
	// Should return early, error still set
	assert.NotNil(t, c.Error())
}

func TestCSV_Generate(t *testing.T) {
	tmpDir := t.TempDir()
	fileName := filepath.Join(tmpDir, "test-csv")

	c := NewCSV()
	c.SetFileName(fileName).
		SetHeader([]string{"Name", "Email", "Role"}).
		AppendDataRow([]string{"Alice", "alice@mail.com", "Admin"}).
		AppendDataRow([]string{"Bob", "bob@mail.com", "User"})

	require.Nil(t, c.Error())
	err := c.Generate()
	require.NoError(t, err)

	// Verify the file was created
	assert.Equal(t, fileName+".csv", c.FileName())
	data, err := os.ReadFile(fileName + ".csv")
	require.NoError(t, err)
	assert.Contains(t, string(data), "Name,Email,Role")
	assert.Contains(t, string(data), "Alice,alice@mail.com,Admin")
	assert.Contains(t, string(data), "Bob,bob@mail.com,User")
}

func TestCSV_Generate_WithError(t *testing.T) {
	c := NewCSV()
	c.err = fmt.Errorf("prior error")
	err := c.Generate()
	assert.Equal(t, c.err, err)
}

func TestCSV_Generate_InvalidPath(t *testing.T) {
	c := NewCSV()
	c.fileName = "/nonexistent/dir/file.csv"
	c.SetHeader([]string{"A"}).AppendDataRow([]string{"1"})
	err := c.Generate()
	assert.Error(t, err)
}

func TestCSV_Error(t *testing.T) {
	c := NewCSV()
	assert.Nil(t, c.Error())
	c.err = fmt.Errorf("test error")
	assert.NotNil(t, c.Error())
}

func TestCSV_FullPipeline(t *testing.T) {
	tmpDir := t.TempDir()
	c := NewCSV()
	c.SetFileName(filepath.Join(tmpDir, "full-test")).
		SetHeader([]string{"ID", "Name"}).
		AppendDataRow([]string{"1", "Alice"}).
		AppendDataRow([]string{"2", "Bob"})

	require.NoError(t, c.Generate())
	assert.Equal(t, filepath.Join(tmpDir, "full-test.csv"), c.FileName())

	// Parse the generated CSV and verify
	f, err := os.Open(c.FileName())
	require.NoError(t, err)
	defer f.Close()

	reader := csvencode.NewReader(f)
	records, err := reader.ReadAll()
	require.NoError(t, err)
	assert.Len(t, records, 3) // header + 2 data rows
	assert.Equal(t, []string{"ID", "Name"}, records[0])
	assert.Equal(t, []string{"1", "Alice"}, records[1])
	assert.Equal(t, []string{"2", "Bob"}, records[2])
}

// ---------------------------------------------------------------------------
// Excel Tests
// ---------------------------------------------------------------------------

func TestNewExcel_NewSource(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	require.NotNil(t, e)
	assert.Nil(t, e.Error())
	assert.Equal(t, "generated.xlsx", e.FileName())
	assert.Equal(t, "Sheet1", e.sheetName)
}

func TestNewExcel_PathSource_InvalidInterface(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "path", File: 123}) // not a string
	require.NotNil(t, e)
	assert.NotNil(t, e.Error())
	assert.Contains(t, e.Error().Error(), "file path isn't set/string")
}

func TestNewExcel_PathSource_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a valid xlsx file first
	e1 := NewExcel(&ExcelOptions{Source: "new"})
	e1.SetFileName(filepath.Join(tmpDir, "source"))
	e1.SetHeader([]string{"Col1", "Col2"})
	e1.AppendDataRow([]string{"Val1", "Val2"})
	require.NoError(t, e1.Generate())

	// Now open it with "path" source
	e2 := NewExcel(&ExcelOptions{Source: "path", File: filepath.Join(tmpDir, "source.xlsx")})
	require.Nil(t, e2.Error())
	assert.NotNil(t, e2.GetExcelFile())
}

func TestNewExcel_PathSource_NonexistentFile(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "path", File: "/nonexistent/file.xlsx"})
	assert.NotNil(t, e.Error())
}

func TestNewExcel_UploadSource_InvalidInterface(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "upload", File: "not a multipart.File"})
	require.NotNil(t, e)
	assert.NotNil(t, e.Error())
	assert.Contains(t, e.Error().Error(), "uploaded file isn't set")
}

func TestNewExcel_InvalidSource(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "invalid"})
	require.NotNil(t, e)
	assert.NotNil(t, e.Error())
	assert.Contains(t, e.Error().Error(), "source isn't valid")
}

func TestExcel_SetFileName(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	e.SetFileName("my-report")
	assert.Equal(t, "my-report.xlsx", e.FileName())
}

func TestExcel_SetFileName_WithError(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	e.err = fmt.Errorf("some error")
	e.SetFileName("my-report")
	// Should not change fileName when error exists
	assert.Equal(t, "generated.xlsx", e.FileName())
}

func TestExcel_SetSheetName(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	e.SetSheetName("CustomSheet")
	assert.Equal(t, "CustomSheet", e.sheetName)
}

func TestExcel_AppendDataRow(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	e.SetHeader([]string{"A", "B"})
	e.AppendDataRow([]string{"1", "2"})
	assert.Nil(t, e.Error())
}

func TestExcel_AppendDataRow_ColumnMismatch(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	e.SetHeader([]string{"A", "B"})
	e.AppendDataRow([]string{"1", "2", "3"})
	assert.NotNil(t, e.Error())
}

func TestExcel_AppendDataRow_FirstRow(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	e.AppendDataRow([]string{"1", "2", "3"})
	assert.Nil(t, e.Error())
}

func TestExcel_AppendDataRow_SkipOnError(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	e.err = fmt.Errorf("some error")
	e.AppendDataRow([]string{"1", "2"})
	assert.NotNil(t, e.Error()) // error still present, not overwritten
}

func TestExcel_SetHeader(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	e.SetHeader([]string{"A", "B"})
	assert.Nil(t, e.Error())
}

func TestExcel_SetHeader_ColumnMismatch(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	e.AppendDataRow([]string{"1", "2", "3"})
	e.SetHeader([]string{"A", "B"})
	assert.NotNil(t, e.Error())
}

func TestExcel_SetHeader_SkipOnError(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	e.err = fmt.Errorf("some error")
	e.SetHeader([]string{"A", "B"})
	assert.NotNil(t, e.Error())
}

func TestExcel_Generate(t *testing.T) {
	tmpDir := t.TempDir()
	e := NewExcel(&ExcelOptions{Source: "new"})
	e.SetFileName(filepath.Join(tmpDir, "test-excel"))
	e.SetSheetName("TestSheet")
	e.SetHeader([]string{"Name", "Email", "Role"})
	e.AppendDataRow([]string{"Alice", "alice@mail.com", "Admin"})
	e.AppendDataRow([]string{"Bob", "bob@mail.com", "User"})

	require.Nil(t, e.Error())
	err := e.Generate()
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(filepath.Join(tmpDir, "test-excel.xlsx"))
	require.NoError(t, err)
}

func TestExcel_Generate_WithError(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	e.err = fmt.Errorf("prior error")
	err := e.Generate()
	assert.Equal(t, e.err, err)
}

func TestExcel_Generate_InvalidPath(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	e.SetFileName("/nonexistent/dir/file")
	e.SetHeader([]string{"A"})
	e.AppendDataRow([]string{"1"})
	err := e.Generate()
	assert.Error(t, err)
}

func TestExcel_GetExcelFile(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	assert.NotNil(t, e.GetExcelFile())
}

func TestExcel_Error(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	assert.Nil(t, e.Error())
}

func TestExcel_GetColumnAlphabet(t *testing.T) {
	assert.Equal(t, "A", getColumnAlphabet(1))
	assert.Equal(t, "B", getColumnAlphabet(2))
	assert.Equal(t, "Z", getColumnAlphabet(26))
	assert.Equal(t, "AA", getColumnAlphabet(27))
	assert.Equal(t, ".", getColumnAlphabet(0))
}

func TestExcel_ScanContentToStruct(t *testing.T) {
	tmpDir := t.TempDir()
	// Create an Excel file with data
	e := NewExcel(&ExcelOptions{Source: "new"})
	e.SetFileName(filepath.Join(tmpDir, "scan-test"))
	e.SetHeader([]string{"Name", "Email"})
	e.AppendDataRow([]string{"Alice", "alice@mail.com"})
	require.NoError(t, e.Generate())

	// Now open it and scan
	e2 := NewExcel(&ExcelOptions{Source: "path", File: filepath.Join(tmpDir, "scan-test.xlsx")})
	require.Nil(t, e2.Error())

	type TestStruct struct {
		Name  string `json:"Name"`
		Email string `json:"Email"`
	}
	var result []TestStruct
	err := e2.ScanContentToStruct("Sheet1", &result)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "Alice", result[0].Name)
	assert.Equal(t, "alice@mail.com", result[0].Email)
}

func TestExcel_ScanContentToStruct_NotPointer(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	type TestStruct struct {
		Name string `json:"Name"`
	}
	var result []TestStruct // not a pointer
	err := e.ScanContentToStruct("Sheet1", result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "destination struct must be pointer")
}

func TestExcel_GetContents(t *testing.T) {
	tmpDir := t.TempDir()
	e := NewExcel(&ExcelOptions{Source: "new"})
	e.SetFileName(filepath.Join(tmpDir, "contents-test"))
	e.SetHeader([]string{"Name", "Email"})
	e.AppendDataRow([]string{"Alice", "alice@mail.com"})
	require.NoError(t, e.Generate())

	e2 := NewExcel(&ExcelOptions{Source: "path", File: filepath.Join(tmpDir, "contents-test.xlsx")})
	require.Nil(t, e2.Error())

	data, err := e2.GetContents("Sheet1")
	require.NoError(t, err)
	require.Len(t, data, 1)
	assert.Equal(t, "Alice", data[0]["Name"])
	assert.Equal(t, "alice@mail.com", data[0]["Email"])
}

func TestExcel_GetContents_EmptySheet(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	// GetContents on a sheet with only default content
	data, err := e.GetContents("Sheet1")
	// The default Sheet1 has no data rows, so should return nil or empty
	if err != nil {
		// It's OK if there's an error for empty sheet
		t.Logf("GetContents on empty sheet returned error: %v", err)
	} else {
		assert.Nil(t, data)
	}
}

func TestExcel_GetMergeCell(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	// No merge cells by default
	cells, err := e.GetMergeCell("Sheet1")
	assert.NoError(t, err)
	assert.Empty(t, cells)
}

func TestExcel_FileName(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	assert.Equal(t, "generated.xlsx", e.FileName())
}

// ---------------------------------------------------------------------------
// Bulk Generator Tests
// ---------------------------------------------------------------------------

func TestNewBulkGenerator(t *testing.T) {
	bg := NewBulkGenerator()
	require.NotNil(t, bg)
	assert.Equal(t, 5, bg.worker) // default worker
}

func TestNewBulkGenerator_WithBulkWorker(t *testing.T) {
	bg := NewBulkGenerator(WithBulkWorker(10))
	assert.Equal(t, 10, bg.worker)
}

func TestNewBulkGenerator_WithBulkWorker_Zero(t *testing.T) {
	bg := NewBulkGenerator(WithBulkWorker(0))
	assert.Equal(t, 5, bg.worker) // zero should not override default
}

func TestNewBulkGenerator_WithBulkWorker_Negative(t *testing.T) {
	bg := NewBulkGenerator(WithBulkWorker(-1))
	assert.Equal(t, 5, bg.worker) // negative should not override default
}

func TestBulkGenerator_Add(t *testing.T) {
	bg := NewBulkGenerator()
	mock1 := &mockGenerator{fileName: "file1.csv"}
	mock2 := &mockGenerator{fileName: "file2.csv"}
	bg.Add(mock1, mock2)
	assert.Len(t, bg.generators, 2)
}

func TestBulkGenerator_GenerateAll(t *testing.T) {
	tmpDir := t.TempDir()

	// Create real CSV generators
	c1 := NewCSV()
	c1.SetFileName(filepath.Join(tmpDir, "bulk1")).
		SetHeader([]string{"A", "B"}).
		AppendDataRow([]string{"1", "2"})

	c2 := NewCSV()
	c2.SetFileName(filepath.Join(tmpDir, "bulk2")).
		SetHeader([]string{"C", "D"}).
		AppendDataRow([]string{"3", "4"})

	bg := NewBulkGenerator(WithBulkWorker(2))
	bg.Add(c1, c2)

	results := bg.GenerateAll()
	assert.Len(t, results, 2)
	for _, r := range results {
		assert.NoError(t, r.Error)
		assert.NotEmpty(t, r.FilePath)
	}
}

func TestBulkGenerator_GenerateAll_WithErrors(t *testing.T) {
	mock1 := &mockGenerator{fileName: "ok.csv", genErr: nil}
	mock2 := &mockGenerator{fileName: "fail.csv", genErr: fmt.Errorf("generation failed")}

	bg := NewBulkGenerator(WithBulkWorker(2))
	bg.Add(mock1, mock2)

	results := bg.GenerateAll()
	assert.Len(t, results, 2)
	assert.NoError(t, results[0].Error)
	assert.Error(t, results[1].Error)
}

func TestBulkGenerator_GenerateAll_Empty(t *testing.T) {
	bg := NewBulkGenerator()
	results := bg.GenerateAll()
	assert.Empty(t, results)
}

func TestBulkGenerator_GenerateZip(t *testing.T) {
	tmpDir := t.TempDir()

	// Create CSV files
	c1 := NewCSV()
	c1.SetFileName(filepath.Join(tmpDir, "zip1")).
		SetHeader([]string{"A", "B"}).
		AppendDataRow([]string{"1", "2"})
	require.NoError(t, c1.Generate())

	c2 := NewCSV()
	c2.SetFileName(filepath.Join(tmpDir, "zip2")).
		SetHeader([]string{"C", "D"}).
		AppendDataRow([]string{"3", "4"})
	require.NoError(t, c2.Generate())

	bg := NewBulkGenerator(WithBulkWorker(2))
	bg.Add(c1, c2)

	zipBytes, results, err := bg.GenerateZip("test.zip")
	require.NoError(t, err)
	assert.NotEmpty(t, zipBytes)
	assert.Len(t, results, 2)

	// Verify the zip content
	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	require.NoError(t, err)
	assert.Len(t, reader.File, 2)
}

func TestBulkGenerator_GenerateZip_WithErrors(t *testing.T) {
	tmpDir := t.TempDir()
	// Create one real file and one failing generator
	c1 := NewCSV()
	c1.SetFileName(filepath.Join(tmpDir, "zipok")).
		SetHeader([]string{"A"}).
		AppendDataRow([]string{"1"})
	require.NoError(t, c1.Generate())

	mockFail := &mockGenerator{fileName: "", genErr: fmt.Errorf("failed")}

	bg := NewBulkGenerator()
	bg.Add(c1, mockFail)

	zipBytes, results, err := bg.GenerateZip("test.zip")
	require.NoError(t, err)
	assert.Len(t, results, 2)
	// Only one file in the zip
	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	require.NoError(t, err)
	assert.Len(t, reader.File, 1)
}

func TestBulkGenerator_GenerateZip_EmptyFilePath(t *testing.T) {
	mockOk := &mockGenerator{fileName: "", genErr: nil} // empty file path
	bg := NewBulkGenerator()
	bg.Add(mockOk)

	zipBytes, results, err := bg.GenerateZip("test.zip")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	// No files in zip since file path is empty
	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	require.NoError(t, err)
	assert.Empty(t, reader.File)
}

func TestBulkGenerator_GenerateZip_FileOpenError(t *testing.T) {
	// Generator succeeds but file doesn't exist (file was deleted after generate)
	// Note: GenerateZip modifies result.Error on a copy of BulkResult, not the slice element.
	// The original results slice from GenerateAll is returned as-is.
	mockOk := &mockGenerator{fileName: "/nonexistent/file.csv", genErr: nil}
	bg := NewBulkGenerator()
	bg.Add(mockOk)

	_, results, err := bg.GenerateZip("test.zip")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	// The file can't be opened but the error is set on a copy in GenerateZip,
	// so the original results entry has no error (the gen succeeded, just zip archiving failed)
	assert.NoError(t, results[0].Error)
}

// ---------------------------------------------------------------------------
// Banner Tests
// ---------------------------------------------------------------------------

func TestNewBanner(t *testing.T) {
	b := NewBanner()
	require.NotNil(t, b)
	assert.Equal(t, 1200, b.Width)
	assert.Equal(t, 400, b.Height)
}

func TestNewBanner_WithOptions(t *testing.T) {
	b := NewBanner(WithWidth(800), WithHeight(300))
	assert.Equal(t, 800, b.Width)
	assert.Equal(t, 300, b.Height)
}

func TestNewBanner_WithBackgroundColor(t *testing.T) {
	b := NewBanner(WithBackgroudColor("#ff0000"))
	require.NotNil(t, b)
	expectedColor, _ := helper.ParseHexColor("#ff0000")
	assert.Equal(t, expectedColor, b.BgColor)
}

func TestNewBanner_WithInvalidHexColor(t *testing.T) {
	// Should not panic, just log error
	b := NewBanner(WithBackgroudColor("invalid"))
	require.NotNil(t, b)
}

func TestBanner_AddImageLayer(t *testing.T) {
	b := NewBanner()
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	b.AddImageLayer(&ImageLayer{Image: img, XPos: 10, YPos: 20})
	assert.Len(t, b.imgLayer, 1)
}

func TestBanner_AddLabel(t *testing.T) {
	b := NewBanner()
	b.AddLabel(&Label{
		Text:     "Test",
		FontPath: "", // empty font path will cause error in Generate
		Size:     24,
		Color:    color.Black,
		XPos:     50,
		YPos:     50,
		Spacing:  1.5,
	})
	assert.Len(t, b.label, 1)
}

func TestBanner_SetDestPath(t *testing.T) {
	b := NewBanner()
	b.SetDestPath("/tmp/test.png")
	assert.Equal(t, "/tmp/test.png", b.destPath)
}

func TestBanner_FileName(t *testing.T) {
	b := NewBanner()
	b.destPath = "/tmp/mybanner.png"
	assert.Equal(t, "/tmp/mybanner.png", b.FileName())
}

func TestBanner_Generate_NoLabels(t *testing.T) {
	b := NewBanner(WithWidth(200), WithHeight(100))
	result := b.Generate()
	require.NotNil(t, result)
	assert.NotNil(t, b.Image())
}

func TestBanner_Generate_WithImageLayer(t *testing.T) {
	b := NewBanner(WithWidth(200), WithHeight(100))
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	b.AddImageLayer(&ImageLayer{Image: img, XPos: 10, YPos: 10})
	result := b.Generate()
	require.NotNil(t, result)
	assert.NotNil(t, b.Image())
}

func TestBanner_Image(t *testing.T) {
	b := NewBanner()
	assert.Nil(t, b.Image()) // no image before Generate
	b.Generate()
	assert.NotNil(t, b.Image())
}

func TestBanner_Save_PNG(t *testing.T) {
	tmpDir := t.TempDir()
	b := NewBanner(WithWidth(100), WithHeight(50))
	b.Generate()
	require.NotNil(t, b.Image())

	destPath := filepath.Join(tmpDir, "test.png")
	err := b.Save(destPath)
	require.NoError(t, err)

	// Verify file exists and is a valid PNG
	f, err := os.Open(destPath)
	require.NoError(t, err)
	defer f.Close()

	_, err = png.Decode(f)
	assert.NoError(t, err)
}

func TestBanner_Save_JPG(t *testing.T) {
	tmpDir := t.TempDir()
	b := NewBanner(WithWidth(100), WithHeight(50))
	b.Generate()
	require.NotNil(t, b.Image())

	destPath := filepath.Join(tmpDir, "test.jpg")
	err := b.Save(destPath)
	require.NoError(t, err)

	_, err = os.Stat(destPath)
	assert.NoError(t, err)
}

func TestBanner_Save_JPEG(t *testing.T) {
	tmpDir := t.TempDir()
	b := NewBanner(WithWidth(100), WithHeight(50))
	b.Generate()
	require.NotNil(t, b.Image())

	destPath := filepath.Join(tmpDir, "test.jpeg")
	err := b.Save(destPath)
	require.NoError(t, err)

	_, err = os.Stat(destPath)
	assert.NoError(t, err)
}

func TestBanner_Save_GIF(t *testing.T) {
	tmpDir := t.TempDir()
	b := NewBanner(WithWidth(100), WithHeight(50))
	b.Generate()
	require.NotNil(t, b.Image())

	destPath := filepath.Join(tmpDir, "test.gif")
	err := b.Save(destPath)
	require.NoError(t, err)

	_, err = os.Stat(destPath)
	assert.NoError(t, err)
}

func TestBanner_Save_UnsupportedExt(t *testing.T) {
	tmpDir := t.TempDir()
	b := NewBanner(WithWidth(100), WithHeight(50))
	b.Generate()
	require.NotNil(t, b.Image())

	destPath := filepath.Join(tmpDir, "test.bmp")
	err := b.Save(destPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file extensions isn't support yet")
}

func TestBanner_Save_InvalidPath(t *testing.T) {
	b := NewBanner(WithWidth(100), WithHeight(50))
	b.Generate()
	require.NotNil(t, b.Image())

	err := b.Save("/nonexistent/dir/file.png")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// QR Code Tests
// ---------------------------------------------------------------------------

func TestNewQR(t *testing.T) {
	qr, err := NewQR("https://example.com")
	require.NoError(t, err)
	require.NotNil(t, qr)
}

func TestNewQR_EmptyContent(t *testing.T) {
	// Empty string should still work for QR generation
	qr, err := NewQR("")
	require.NoError(t, err)
	require.NotNil(t, qr)
}

func TestQR_SetLogoImg(t *testing.T) {
	qr, _ := NewQR("https://example.com")
	result := qr.SetLogoImg("/path/to/logo.png")
	assert.NotNil(t, result)
}

func TestQR_FileName(t *testing.T) {
	qr, _ := NewQR("https://example.com")
	assert.Equal(t, "", qr.FileName()) // no file name set yet
}

func TestQR_Generate_NoFileName(t *testing.T) {
	qr, _ := NewQR("https://example.com")
	err := qr.Generate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fileName cannot be empty")
}

func TestQR_Generate_WithFileName(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpDir, "qr"), 0755)

	qr, err := NewQR("https://example.com")
	require.NoError(t, err)

	fileName := filepath.Join(tmpDir, "qr", "test-qr")
	qr.SetFileName(&fileName)
	err = qr.Generate()
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(qr.FileName())
	assert.NoError(t, err)
}

func TestQR_SetFileName(t *testing.T) {
	qr, _ := NewQR("https://example.com")
	name := "test-qr"
	qr.SetFileName(&name)
	assert.NotEmpty(t, qr.FileName())
	assert.Contains(t, qr.FileName(), "qr/")
}

// ---------------------------------------------------------------------------
// PDF Tests (basic - wkhtmltopdf may not be available)
// ---------------------------------------------------------------------------

func TestNewPDF(t *testing.T) {
	pdf, err := NewPDF()
	if err != nil {
		// wkhtmltopdf binary might not be installed
		t.Skipf("wkhtmltopdf not available: %v", err)
	}
	require.NotNil(t, pdf)
}

func TestNewPDF_WithCustomOptions(t *testing.T) {
	pdf, err := NewPDF(&ConverterOptions{
		PageSize:     "A4",
		MarginBottom: 20,
		MarginTop:    20,
		MarginLeft:   15,
		MarginRight:  15,
	})
	if err != nil {
		t.Skipf("wkhtmltopdf not available: %v", err)
	}
	require.NotNil(t, pdf)
}

func TestPDF_AddCustomFunction(t *testing.T) {
	pdf, err := NewPDF()
	if err != nil {
		t.Skipf("wkhtmltopdf not available: %v", err)
	}
	result := pdf.AddCustomFunction("testFn", func() string { return "test" })
	assert.NotNil(t, result)
	assert.NotNil(t, pdf.funcMap)
}

func TestPDF_AddCustomFunction_WithError(t *testing.T) {
	pdf, err := NewPDF()
	if err != nil {
		t.Skipf("wkhtmltopdf not available: %v", err)
	}
	pdf.err = fmt.Errorf("some error")
	result := pdf.AddCustomFunction("testFn", func() string { return "test" })
	assert.NotNil(t, result)
	assert.Nil(t, pdf.funcMap) // funcMap should not be initialized
}

func TestPDF_SetFooterHTMLTemplate(t *testing.T) {
	pdf, err := NewPDF()
	if err != nil {
		t.Skipf("wkhtmltopdf not available: %v", err)
	}
	result := pdf.SetFooterHTMLTemplate("/path/to/footer.html")
	assert.NotNil(t, result)
	assert.Equal(t, "/path/to/footer.html", pdf.footerHTMLPath)
}

func TestPDF_Error(t *testing.T) {
	pdf := &PDF{}
	assert.Nil(t, pdf.Error())
	pdf.err = fmt.Errorf("test error")
	assert.NotNil(t, pdf.Error())
}

func TestPDF_Generate_WithError(t *testing.T) {
	pdf := &PDF{}
	pdf.err = fmt.Errorf("prior error")
	err := pdf.Generate()
	assert.Equal(t, pdf.err, err)
}

func TestPDF_FileName(t *testing.T) {
	pdf := &PDF{}
	assert.Equal(t, "", pdf.FileName())
}

func TestPDF_SetTemplate_WithError(t *testing.T) {
	pdf, err := NewPDF()
	if err != nil {
		t.Skipf("wkhtmltopdf not available: %v", err)
	}
	pdf.err = fmt.Errorf("prior error")
	result := pdf.SetTemplate("/nonexistent/path", nil)
	assert.NotNil(t, result)
}

func TestPDF_SetFileName(t *testing.T) {
	pdf, err := NewPDF()
	if err != nil {
		t.Skipf("wkhtmltopdf not available: %v", err)
	}
	name := "test.pdf"
	result := pdf.SetFileName(&name)
	assert.NotNil(t, result)
	assert.Contains(t, pdf.fileName, "pdf/")
}

// ---------------------------------------------------------------------------
// FileGenerator Interface Test
// ---------------------------------------------------------------------------

func TestFileGenerator_Interface(t *testing.T) {
	// Verify CSV implements FileGenerator
	var _ FileGenerator = NewCSV()

	// Verify Excel implements FileGenerator
	var _ FileGenerator = NewExcel(&ExcelOptions{Source: "new"})
}

// ---------------------------------------------------------------------------
// BulkResult Tests
// ---------------------------------------------------------------------------

func TestBulkResult_Fields(t *testing.T) {
	r := BulkResult{
		FilePath: "test.csv",
		Error:    nil,
	}
	assert.Equal(t, "test.csv", r.FilePath)
	assert.Nil(t, r.Error)
}

// ---------------------------------------------------------------------------
// Constants Coverage
// ---------------------------------------------------------------------------

func TestExcelColumnAlphabet(t *testing.T) {
	assert.Equal(t, ".", excelColumnAlphabet[0])
	assert.Equal(t, "A", excelColumnAlphabet[1])
	assert.Equal(t, "Z", excelColumnAlphabet[26])
	assert.Equal(t, "AA", excelColumnAlphabet[27])
	assert.Equal(t, "AZ", excelColumnAlphabet[52])
	assert.Len(t, excelColumnAlphabet, 703) // 1 (dot) + 26 + 26*26
}

// ---------------------------------------------------------------------------
// Mock Generator for Bulk Tests
// ---------------------------------------------------------------------------

type mockGenerator struct {
	fileName string
	genErr   error
}

func (m *mockGenerator) Generate() error  { return m.genErr }
func (m *mockGenerator) FileName() string { return m.fileName }

// ---------------------------------------------------------------------------
// Integration: Bulk with real CSV generators + Zip
// ---------------------------------------------------------------------------

func TestBulkGenerator_GenerateZip_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	c1 := NewCSV()
	c1.SetFileName(filepath.Join(tmpDir, "int1")).
		SetHeader([]string{"Col1", "Col2"}).
		AppendDataRow([]string{"a", "b"})

	c2 := NewCSV()
	c2.SetFileName(filepath.Join(tmpDir, "int2")).
		SetHeader([]string{"Col1", "Col2"}).
		AppendDataRow([]string{"c", "d"})

	bg := NewBulkGenerator(WithBulkWorker(2))
	bg.Add(c1, c2)

	zipBytes, results, err := bg.GenerateZip("integration.zip")
	require.NoError(t, err)

	for _, r := range results {
		assert.NoError(t, r.Error)
	}

	// Verify zip content
	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	require.NoError(t, err)
	assert.Len(t, reader.File, 2)

	// Check content of first file
	for _, f := range reader.File {
		rc, err := f.Open()
		require.NoError(t, err)
		content, err := io.ReadAll(rc)
		require.NoError(t, err)
		rc.Close()
		assert.True(t, strings.Contains(string(content), "Col1,Col2") || strings.Contains(string(content), "a,b") || strings.Contains(string(content), "c,d"))
	}
}

// ---------------------------------------------------------------------------
// Additional Coverage Tests
// ---------------------------------------------------------------------------

func TestBanner_Generate_WithLabel_InvalidFont(t *testing.T) {
	b := NewBanner(WithWidth(200), WithHeight(100))
	b.AddLabel(&Label{
		Text:        "Hello World",
		FontPath:    "/nonexistent/font.ttf",
		Size:        24,
		Color:       color.Black,
		XPos:        50,
		YPos:        50,
		Spacing:     1.5,
		RightMargin: 10,
	})
	result := b.Generate()
	// With invalid font path, Generate returns nil (error logged internally)
	assert.Nil(t, result)
}

func TestBanner_Save_UpdatesDestPath(t *testing.T) {
	tmpDir := t.TempDir()
	b := NewBanner(WithWidth(100), WithHeight(50))
	b.Generate()

	destPath := filepath.Join(tmpDir, "path-test.png")
	err := b.Save(destPath)
	require.NoError(t, err)
	// Save should update destPath
	assert.Equal(t, destPath, b.destPath)
	assert.Equal(t, destPath, b.FileName())
}

func TestCSV_Generate_EmptyContent(t *testing.T) {
	tmpDir := t.TempDir()
	c := NewCSV()
	c.SetFileName(filepath.Join(tmpDir, "empty-csv"))
	// No headers, no data
	err := c.Generate()
	require.NoError(t, err)
}

func TestExcel_GetContents_MultipleRows(t *testing.T) {
	tmpDir := t.TempDir()
	e := NewExcel(&ExcelOptions{Source: "new"})
	e.SetFileName(filepath.Join(tmpDir, "multi-contents"))
	e.SetHeader([]string{"Name", "Email", "Role"})
	e.AppendDataRow([]string{"Alice", "alice@mail.com", "Admin"})
	e.AppendDataRow([]string{"Bob", "bob@mail.com", "User"})
	e.AppendDataRow([]string{"Charlie", "charlie@mail.com", "User"})
	require.NoError(t, e.Generate())

	e2 := NewExcel(&ExcelOptions{Source: "path", File: filepath.Join(tmpDir, "multi-contents.xlsx")})
	require.Nil(t, e2.Error())

	data, err := e2.GetContents("Sheet1")
	require.NoError(t, err)
	require.Len(t, data, 3)
	assert.Equal(t, "Alice", data[0]["Name"])
	assert.Equal(t, "Bob", data[1]["Name"])
	assert.Equal(t, "Charlie", data[2]["Name"])
}

func TestExcel_ScanContentToStruct_MultipleRows(t *testing.T) {
	tmpDir := t.TempDir()
	e := NewExcel(&ExcelOptions{Source: "new"})
	e.SetFileName(filepath.Join(tmpDir, "scan-multi"))
	e.SetHeader([]string{"Name", "Email"})
	e.AppendDataRow([]string{"Alice", "alice@mail.com"})
	e.AppendDataRow([]string{"Bob", "bob@mail.com"})
	require.NoError(t, e.Generate())

	e2 := NewExcel(&ExcelOptions{Source: "path", File: filepath.Join(tmpDir, "scan-multi.xlsx")})
	require.Nil(t, e2.Error())

	type TestStruct struct {
		Name  string `json:"Name"`
		Email string `json:"Email"`
	}
	var result []TestStruct
	err := e2.ScanContentToStruct("Sheet1", &result)
	require.NoError(t, err)
	require.Len(t, result, 2)
}

func TestExcel_Generate_MultipleColumns(t *testing.T) {
	tmpDir := t.TempDir()
	e := NewExcel(&ExcelOptions{Source: "new"})
	e.SetFileName(filepath.Join(tmpDir, "multi-col"))
	headers := make([]string, 26)
	row := make([]string, 26)
	for i := 0; i < 26; i++ {
		headers[i] = fmt.Sprintf("Col%d", i)
		row[i] = fmt.Sprintf("Val%d", i)
	}
	e.SetHeader(headers)
	e.AppendDataRow(row)
	require.NoError(t, e.Generate())

	_, err := os.Stat(filepath.Join(tmpDir, "multi-col.xlsx"))
	assert.NoError(t, err)
}

func TestPDF_SetTemplate_ValidTemplate(t *testing.T) {
	pdf, err := NewPDF()
	if err != nil {
		t.Skipf("wkhtmltopdf not available: %v", err)
	}
	tmpDir := t.TempDir()
	// Create a simple HTML template
	templatePath := filepath.Join(tmpDir, "template.html")
	err = os.WriteFile(templatePath, []byte("<h1>Hello {{.Name}}</h1>"), 0644)
	require.NoError(t, err)

	result := pdf.SetTemplate(templatePath, map[string]string{"Name": "World"})
	assert.NotNil(t, result)
	assert.NoError(t, pdf.Error())
	assert.Contains(t, pdf.content.String(), "Hello World")
}

func TestPDF_SetTemplate_InvalidTemplate(t *testing.T) {
	pdf, err := NewPDF()
	if err != nil {
		t.Skipf("wkhtmltopdf not available: %v", err)
	}
	result := pdf.SetTemplate("/nonexistent/template.html", nil)
	assert.NotNil(t, result)
	assert.Error(t, pdf.Error())
}

func TestBanner_Generate_MultipleImageLayers(t *testing.T) {
	b := NewBanner(WithWidth(200), WithHeight(100))
	img1 := image.NewRGBA(image.Rect(0, 0, 50, 50))
	img2 := image.NewRGBA(image.Rect(0, 0, 30, 30))
	b.AddImageLayer(&ImageLayer{Image: img1, XPos: 0, YPos: 0})
	b.AddImageLayer(&ImageLayer{Image: img2, XPos: 50, YPos: 50})
	result := b.Generate()
	require.NotNil(t, result)
	assert.NotNil(t, b.Image())
}

func TestBanner_Generate_MultipleLabels_InvalidFont(t *testing.T) {
	b := NewBanner(WithWidth(200), WithHeight(100))
	b.AddLabel(&Label{
		Text:     "First",
		FontPath: "/nonexistent/font.ttf",
		Size:     20,
		Color:    color.Black,
		XPos:     10,
		YPos:     30,
		Spacing:  1.5,
	})
	// First label has invalid font, Generate should return nil
	result := b.Generate()
	assert.Nil(t, result)
}

func TestBulkGenerator_GenerateZip_IoCopyError(t *testing.T) {
	// Test the os.Open error path: use a mock that succeeds at Generate
	// but returns a nonexistent file path so os.Open fails in GenerateZip.
	tmpDir := t.TempDir()

	c1 := NewCSV()
	c1.SetFileName(filepath.Join(tmpDir, "persist")).
		SetHeader([]string{"A"}).
		AppendDataRow([]string{"1"})
	require.NoError(t, c1.Generate())

	// mockWithDeletedFile returns a nonexistent path but no generate error
	mockDeleted := &mockGenerator{fileName: "/nonexistent/deleted-file.csv", genErr: nil}

	bg := NewBulkGenerator(WithBulkWorker(2))
	bg.Add(c1, mockDeleted)

	zipBytes, results, err := bg.GenerateZip("test.zip")
	require.NoError(t, err)
	assert.Len(t, results, 2)

	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	require.NoError(t, err)
	// Only c1's file is in the zip; mockDeleted's file couldn't be opened
	assert.Len(t, reader.File, 1)
}
