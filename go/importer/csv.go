package importer

import (
	"context"
	csvencode "encoding/csv"
	"encoding/json"
	"io"
	"mime/multipart"
	"reflect"
)

type (
	csv struct{}
)

func (e excel) readFromCSV(structSource reflect.Value, file multipart.File, mapContent map[string]interface{}, ctx ...context.Context) <-chan interface{} {
	chanOut := make(chan interface{})
	go func() {

		csvReader := csvencode.NewReader(file)

		rows, err := csvReader.ReadAll()
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}
		for rowIndex, row := range rows {
			if rowIndex == 0 {
				continue
			}
			destStruct := reflect.New(structSource.Type()).Interface()

			for x, rowData := range row {
				mapContent[rows[0][x]] = rowData
			}
			dt, _ := json.Marshal(mapContent)
			_ = json.Unmarshal(dt, destStruct)
			chanOut <- destStruct
		}
		close(chanOut)
	}()
	return chanOut
}
