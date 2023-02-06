package generator

import (
	"encoding/csv"
	"fmt"
	"os"

	"github.com/pkg/errors"
)

type CSVs interface {
	SetFileName(fileName string) CSVs
	AppendDataRow(data []string) CSVs
	SetHeader(data []string) CSVs
	Generate() error
}

type CSV struct {
	content  [][]string
	fileName string
	err      error
}

// New - to initialize CSV struct object
// fileName: by default should be "generated-csv.csv"
func NewCSV() *CSV {
	return &CSV{
		fileName: "generated-csv.csv",
	}
}

func (c *CSV) SetFileName(fileName string) CSVs {
	c.fileName = fmt.Sprintf("%s.csv", fileName)
	return c
}

func (c *CSV) AppendDataRow(data []string) CSVs {
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

func (c *CSV) SetHeader(data []string) CSVs {
	if c.content != nil && len(c.content) > 0 {
		totalColumnExisting := len(c.content[0])
		totalColumnData := len(data)
		if totalColumnData != totalColumnExisting {
			c.err = errors.Wrap(errors.New("Total Column isn't equal with total existing column"), "phastos.go.generator.excel.AppendDataRow.CheckTotalColumn")
		}
	}
	c.content = append([][]string{data}, c.content...)
	return c
}

func (c *CSV) Generate() error {
	if c.err != nil {
		return c.err
	}
	csvNewFile, err := os.Create(c.fileName)
	if err != nil {
		return errors.Wrap(err, "phastos.go.generator.csv.Generate.CreateCSVFile")
	}

	csvWriter := csv.NewWriter(csvNewFile)
	defer csvWriter.Flush()
	if err = csvWriter.WriteAll(c.content); err != nil {
		return errors.Wrap(err, "phastos.go.generator.csv.Generate.WriteAllContent")
	}

	return nil
}
