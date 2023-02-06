package generator

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/xuri/excelize/v2"
)

type Excels interface {
	SetFileName(fileName string) Excels
	SetSheetName(sheetName string) Excels
	AppendDataRow(data []string) Excels
	SetHeader(data []string) Excels
	Generate() error
}

type Excel struct {
	content   [][]string
	sheetName string
	fileName  string
	err       error
}

// New - to initialize Excel struct object
// fileName: by default should be "generated.xlsx"
func NewExcel() *Excel {
	return &Excel{
		fileName:  "generated.xlsx",
		sheetName: "Sheet1",
	}
}

func (c *Excel) SetFileName(fileName string) Excels {
	c.fileName = fmt.Sprintf("%s.xlsx", fileName)
	return c
}

func (c *Excel) AppendDataRow(data []string) Excels {
	if c.content != nil && len(c.content) > 0 {
		totalColumnExisting := len(c.content[0])
		totalColumnData := len(data)
		if totalColumnData != totalColumnExisting {
			c.err = errors.Wrap(errors.New("Total Column isn't equal with total existing column"), "phastos.go.generator.excel.AppendDataRow.CheckTotalColumn")
		}
	}
	c.content = append(c.content, data)
	return c
}

func (c *Excel) SetHeader(data []string) Excels {
	if c.content != nil && len(c.content) > 0 {
		totalColumnExisting := len(c.content[0])
		totalColumnData := len(data)
		if totalColumnData != totalColumnExisting {
			c.err = errors.Wrap(errors.New("Total Column isn't equal with total existing column"), "phastos.go.generator.excel.SetHeader.CheckTotalColumn")
		}
	}
	c.content = append([][]string{data}, c.content...)
	return c
}

func (c *Excel) SetSheetName(sheetName string) Excels {
	c.sheetName = sheetName
	return c
}

func getColumnAlphabet(i int) string {
	return excelColumnAlphabet[i]
}

func (c *Excel) Generate() error {
	if c.err != nil {
		return c.err
	}
	excelFile := excelize.NewFile()

	sheetIndex, err := excelFile.NewSheet(c.sheetName)
	if err != nil {
		return errors.Wrap(err, "phastos.go.generator.excel.Generate.NewSheet")
	}

	excelFile.SetActiveSheet(sheetIndex)

	headerStyle, err := excelFile.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})

	// generate content of Data
	for row, column := range c.content {
		totalColumn := len(column)
		for i, data := range column {
			cellIndex := fmt.Sprintf("%s%d", getColumnAlphabet(i+1), row+1)
			if err = excelFile.SetCellStr(c.sheetName, cellIndex, data); err != nil {
				return errors.Wrap(err, "phastos.go.generator.excel.Generate.SetCellValue")
			}
		}
		if row == 0 {
			_ = excelFile.SetCellStyle(c.sheetName, "A1", fmt.Sprintf("%s1", getColumnAlphabet(totalColumn)), headerStyle)
		}
	}

	if err = excelFile.SaveAs(c.fileName); err != nil {
		return errors.Wrap(err, "phastos.go.generator.excel.Generate.SaveAsExcel")
	}
	return nil
}
