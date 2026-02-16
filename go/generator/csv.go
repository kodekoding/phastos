package generator

import (
	csvpkg "encoding/csv"
	"fmt"
	"os"

	"github.com/pkg/errors"
)

type CSVs interface {
	FileGenerator
	SetFileName(fileName string) CSVs
	AppendDataRow(data []string) CSVs
	SetHeader(data []string) CSVs
	Error() error
}

type csv struct {
	content  [][]string
	fileName string
	err      error
}

// New - to initialize CSV struct object
// fileName: by default should be "generated-csv.csv"
func NewCSV() *csv {
	return &csv{
		fileName: "generated-csv.csv",
	}
}

func (c *csv) SetFileName(fileName string) CSVs {
	c.fileName = fmt.Sprintf("%s.csv", fileName)
	return c
}

func (c *csv) AppendDataRow(data []string) CSVs {
	if c.err != nil {
		return c
	}
	if c.content != nil && len(c.content) > 0 {
		totalColumnExisting := len(c.content[0])
		totalColumnData := len(data)
		if totalColumnData != totalColumnExisting {
			c.err = errors.Wrap(errors.New("Total Column isn't equal with total existing column"), "phastos.go.generator.csv.AppendDataRow.CheckTotalColumn")
			return c
		}
	}
	c.content = append(c.content, data)
	return c
}

func (c *csv) SetHeader(data []string) CSVs {
	if c.err != nil {
		return c
	}
	if c.content != nil && len(c.content) > 0 {
		totalColumnExisting := len(c.content[0])
		totalColumnData := len(data)
		if totalColumnData != totalColumnExisting {
			c.err = errors.Wrap(errors.New("Total Column isn't equal with total existing column"), "phastos.go.generator.csv.SetHeader.CheckTotalColumn")
			return c
		}
	}
	c.content = append([][]string{data}, c.content...)
	return c
}

func (c *csv) Generate() error {
	if c.err != nil {
		return c.err
	}
	csvNewFile, err := os.Create(c.fileName)
	if err != nil {
		return errors.Wrap(err, "phastos.go.generator.csv.Generate.CreateCSVFile")
	}
	defer csvNewFile.Close()

	csvWriter := csvpkg.NewWriter(csvNewFile)
	if err = csvWriter.WriteAll(c.content); err != nil {
		return errors.Wrap(err, "phastos.go.generator.csv.Generate.WriteAllContent")
	}
	csvWriter.Flush()
	if err = csvWriter.Error(); err != nil {
		return errors.Wrap(err, "phastos.go.generator.csv.Generate.FlushContent")
	}

	return nil
}

func (c *csv) FileName() string {
	return c.fileName
}

func (c *csv) Error() error {
	return c.err
}
