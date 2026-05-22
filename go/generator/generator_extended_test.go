package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetColumnAlphabet(t *testing.T) {
	tests := []struct {
		index    int
		expected string
	}{
		{0, "."},
		{1, "A"},
		{2, "B"},
		{26, "Z"},
		{27, "AA"},
		{28, "AB"},
		{52, "AZ"},
		{53, "BA"},
	}
	for _, tt := range tests {
		result := getColumnAlphabet(tt.index)
		assert.Equal(t, tt.expected, result)
	}
}

func TestNewCSVDefaults(t *testing.T) {
	c := NewCSV()
	assert.NotNil(t, c)
	assert.Equal(t, "generated-csv.csv", c.fileName)
	assert.Nil(t, c.err)
}

func TestCSVSetFileName(t *testing.T) {
	c := NewCSV()
	c.SetFileName("report")
	assert.Equal(t, "report.csv", c.fileName)
}

func TestCSVSetHeader(t *testing.T) {
	c := NewCSV()
	c.SetHeader([]string{"Name", "Email"})
	assert.NoError(t, c.err)
	assert.Len(t, c.content, 1)
}

func TestCSVAppendDataRow(t *testing.T) {
	c := NewCSV()
	c.SetHeader([]string{"Name", "Email"})
	c.AppendDataRow([]string{"Alice", "alice@example.com"})
	assert.NoError(t, c.err)
	assert.Len(t, c.content, 2)
}

func TestCSVAppendDataRowColumnMismatch(t *testing.T) {
	c := NewCSV()
	c.SetHeader([]string{"Name", "Email"})
	c.AppendDataRow([]string{"Alice"})
	assert.Error(t, c.err)
}

func TestCSVSetHeaderColumnMismatch(t *testing.T) {
	c := NewCSV()
	c.SetHeader([]string{"Name", "Email"})
	c.SetHeader([]string{"Name"})
	assert.Error(t, c.err)
}

func TestCSVError(t *testing.T) {
	c := NewCSV()
	c.err = assert.AnError
	assert.Equal(t, assert.AnError, c.Error())
}

func TestCSVFileName(t *testing.T) {
	c := NewCSV()
	assert.Equal(t, "generated-csv.csv", c.FileName())
}

func TestCSVAppendAfterError(t *testing.T) {
	c := NewCSV()
	c.err = assert.AnError
	c.AppendDataRow([]string{"test"})
	// Should return early without modifying content
	assert.Equal(t, assert.AnError, c.err)
}

func TestNewExcelWithNewSource(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	assert.NotNil(t, e)
	assert.NotNil(t, e.excelFile)
	assert.Equal(t, "generated.xlsx", e.fileName)
	assert.Equal(t, "Sheet1", e.sheetName)
}

func TestNewExcelInvalidSource(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "invalid"})
	assert.Error(t, e.err)
}

func TestNewExcelPathSourceNotString(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "path", File: 123})
	assert.Error(t, e.err)
}

func TestNewExcelUploadSourceNotFile(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "upload", File: "not-a-file"})
	assert.Error(t, e.err)
}

func TestExcelSetFileName(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	e.SetFileName("report")
	assert.Equal(t, "report.xlsx", e.fileName)
}

func TestExcelSetFileNameAfterError(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "invalid"})
	e.SetFileName("report")
	// Should not change fileName when err is set
	assert.NotEqual(t, "report.xlsx", e.fileName)
}

func TestExcelSetSheetName(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	e.SetSheetName("Data")
	assert.Equal(t, "Data", e.sheetName)
}

func TestExcelGetExcelFile(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	assert.NotNil(t, e.GetExcelFile())
}

func TestExcelError(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "invalid"})
	assert.Error(t, e.Error())
}

func TestExcelFileName(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	assert.Equal(t, "generated.xlsx", e.FileName())
}

func TestExcelAppendDataRow(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	e.AppendDataRow([]string{"A", "B"})
	assert.NoError(t, e.err)
}

func TestExcelAppendDataRowMismatch(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	e.AppendDataRow([]string{"A", "B"})
	e.AppendDataRow([]string{"A"})
	assert.Error(t, e.err)
}

func TestExcelSetHeader(t *testing.T) {
	e := NewExcel(&ExcelOptions{Source: "new"})
	e.SetHeader([]string{"Col1", "Col2"})
	assert.NoError(t, e.err)
}

func TestBulkGeneratorDefaults(t *testing.T) {
	bg := NewBulkGenerator()
	assert.Equal(t, 5, bg.worker)
}

func TestBulkGeneratorWithWorker(t *testing.T) {
	bg := NewBulkGenerator(WithBulkWorker(10))
	assert.Equal(t, 10, bg.worker)
}

func TestBulkGeneratorWithZeroWorker(t *testing.T) {
	bg := NewBulkGenerator(WithBulkWorker(0))
	// Zero should not override the default
	assert.Equal(t, 5, bg.worker)
}

func TestBulkGeneratorAdd(t *testing.T) {
	bg := NewBulkGenerator()
	bg.Add(&stubGenerator{})
	assert.Len(t, bg.generators, 1)
}

func TestBulkResult(t *testing.T) {
	br := BulkResult{
		FilePath: "/test/file.csv",
		Error:    nil,
	}
	assert.Equal(t, "/test/file.csv", br.FilePath)
	assert.Nil(t, br.Error)
}

func TestConverterOptions(t *testing.T) {
	co := ConverterOptions{
		PageSize:     "A4",
		MarginBottom: 10,
		MarginTop:    10,
		MarginLeft:   11,
		MarginRight:  11,
	}
	assert.Equal(t, "A4", co.PageSize)
}

// stubGenerator implements FileGenerator for testing
type stubGenerator struct{}

func (s *stubGenerator) Generate() error  { return nil }
func (s *stubGenerator) FileName() string { return "stub.txt" }
