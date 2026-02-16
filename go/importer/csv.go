package importer

import (
	"context"
	csvencode "encoding/csv"
	"encoding/json"
	"io"
	"mime/multipart"
	"reflect"
	"strings"
)

type (
	csv struct{}
)

func (e excel) readFromCSV(structSource reflect.Value, file multipart.File, ctx ...context.Context) <-chan rowData {
	chanOut := make(chan rowData)
	go func() {
		defer close(chanOut)

		csvReader := csvencode.NewReader(file)

		// read header row first
		headers, err := csvReader.Read()
		if err != nil {
			return
		}

		// clean header names (remove "*" marker)
		for i, h := range headers {
			headers[i] = strings.Replace(h, "*", "", -1)
		}

		// stream row-by-row instead of loading entire file into memory
		for {
			row, err := csvReader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}

			// create a fresh map for each row (avoids stale data from previous rows)
			rowMap := buildRowMap(headers, row)

			destStruct := reflect.New(structSource.Type()).Interface()
			dt, _ := json.Marshal(rowMap)
			_ = json.Unmarshal(dt, destStruct)

			chanOut <- rowData{
				ParsedStruct: destStruct,
				RawData:      rowMap,
			}
		}
	}()
	return chanOut
}
