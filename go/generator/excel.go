package generator

import (
	"encoding/json"
	"fmt"
	"mime/multipart"
	"reflect"

	"github.com/pkg/errors"
	"github.com/xuri/excelize/v2"
)

type Excels interface {
	SetFileName(fileName string) Excels
	GetExcelFile() *excelize.File
	Error() error
	SetSheetName(sheetName string) Excels
	AppendDataRow(data []string) Excels
	SetHeader(data []string) Excels
	ScanContentToStruct(sheetName string, destinationStruct interface{}) error
	GetContents(sheetName string) ([]map[string]string, error)
	GetMergeCell(sheetName string) ([]excelize.MergeCell, error)
	Generate() error
}

type excel struct {
	content   [][]string
	sheetName string
	fileName  string
	err       error
	excelFile *excelize.File
}

type ExcelOptions struct {
	Source string
	File   interface{}
}

// New - to initialize Excel struct object
// fileName: by default should be "generated.xlsx"
func NewExcel(options *ExcelOptions) *excel {
	excelObj := &excel{
		fileName:  "generated.xlsx",
		sheetName: "Sheet1",
	}

	switch options.Source {
	case "new":
		excelObj.excelFile = excelize.NewFile()
	case "path":
		file, valid := options.File.(string)
		if !valid {
			excelObj.err = errors.Wrap(errors.New("file path isn't set/string"), "phastos.go.generator.excel.New.CheckFileInterface")
			return excelObj
		}
		excelObj.excelFile, excelObj.err = excelize.OpenFile(file)
	case "upload":
		file, valid := options.File.(multipart.File)
		if !valid {
			excelObj.err = errors.Wrap(errors.New("uploaded file isn't set"), "phastos.go.generator.excel.New.CheckFileInterface")
			return excelObj
		}
		excelObj.excelFile, excelObj.err = excelize.OpenReader(file)
	default:
		excelObj.err = errors.Wrap(errors.New("source isn't valid, should be 'new', 'path', and 'upload'"), "phastos.go.generator.excel.New.CheckSource")
	}
	return excelObj
}

func (c *excel) SetFileName(fileName string) Excels {
	if c.err == nil {
		c.fileName = fmt.Sprintf("%s.xlsx", fileName)
	}
	return c
}

func (c *excel) AppendDataRow(data []string) Excels {
	if c.err == nil {
		if c.content != nil && len(c.content) > 0 {
			totalColumnExisting := len(c.content[0])
			totalColumnData := len(data)
			if totalColumnData != totalColumnExisting {
				c.err = errors.Wrap(errors.New("Total Column isn't equal with total existing column"), "phastos.go.generator.excel.AppendDataRow.CheckTotalColumn")
			}
		}
	}
	c.content = append(c.content, data)
	return c
}

func (c *excel) SetHeader(data []string) Excels {
	if c.err == nil {
		if c.content != nil && len(c.content) > 0 {
			totalColumnExisting := len(c.content[0])
			totalColumnData := len(data)
			if totalColumnData != totalColumnExisting {
				c.err = errors.Wrap(errors.New("Total Column isn't equal with total existing column"), "phastos.go.generator.excel.SetHeader.CheckTotalColumn")
			}
		}
	}
	c.content = append([][]string{data}, c.content...)
	return c
}

func (c *excel) SetSheetName(sheetName string) Excels {
	c.sheetName = sheetName
	return c
}

func (c *excel) GetExcelFile() *excelize.File {
	return c.excelFile
}

func getColumnAlphabet(i int) string {
	return excelColumnAlphabet[i]
}

func (c *excel) Generate() error {
	if c.err != nil {
		return c.err
	}

	sheetIndex, err := c.excelFile.NewSheet(c.sheetName)
	if err != nil {
		return errors.Wrap(err, "phastos.go.generator.excel.Generate.NewSheet")
	}

	c.excelFile.SetActiveSheet(sheetIndex)

	headerStyle, err := c.excelFile.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})

	// generate content of Data
	for row, column := range c.content {
		totalColumn := len(column)
		for i, data := range column {
			cellIndex := fmt.Sprintf("%s%d", getColumnAlphabet(i+1), row+1)
			if err = c.excelFile.SetCellStr(c.sheetName, cellIndex, data); err != nil {
				return errors.Wrap(err, "phastos.go.generator.excel.Generate.SetCellValue")
			}
		}
		if row == 0 {
			_ = c.excelFile.SetCellStyle(c.sheetName, "A1", fmt.Sprintf("%s1", getColumnAlphabet(totalColumn)), headerStyle)
		}
	}

	if err = c.excelFile.SaveAs(c.fileName); err != nil {
		return errors.Wrap(err, "phastos.go.generator.excel.Generate.SaveAsExcel")
	}
	return nil
}

func (c *excel) GetMergeCell(sheetName string) ([]excelize.MergeCell, error) {
	return c.excelFile.GetMergeCells(sheetName)
}

func (c *excel) Error() error {
	return c.err
}

func (c *excel) ScanContentToStruct(sheetName string, destinationStruct interface{}) error {
	if reflect.ValueOf(destinationStruct).Kind() != reflect.Ptr {
		notPointerStruct := errors.New("destination struct must be pointer")
		return errors.Wrap(notPointerStruct, "pkg.util.excel.ScanContent.CheckStruct")
	}
	data, err := c.GetContents(sheetName)
	if err != nil {
		return errors.Wrap(err, "pkg.util.excel.ScanContent.GetContents")
	}
	marshalData, err := json.Marshal(data)
	if err != nil {
		return errors.Wrap(err, "pkg.util.excel.GetContents.MarshalData")
	}

	if err = json.Unmarshal(marshalData, destinationStruct); err != nil {
		return errors.Wrap(err, "pkg.util.excel.GetContents.UnmarshalToStruct")
	}
	return nil
}

func (c *excel) GetContents(sheetName string) ([]map[string]string, error) {
	rows, err := c.excelFile.GetRows(sheetName)
	if err != nil {
		return nil, errors.Wrap(err, "pkg.util.excel.GetContents.GetRows")
	}
	header := rows[0]

	rowsLen := len(rows)
	var data []map[string]string
	for i := 1; i < rowsLen; i++ {

		colsLen := len(rows[i])
		rowData := make(map[string]string)

		for x := 0; x < colsLen; x++ {
			rowData[header[x]] = rows[i][x]
		}

		data = append(data, rowData)
	}

	return data, nil
}
