package importer

import (
	"context"
	"encoding/json"
	"mime/multipart"
	"reflect"

	"github.com/rs/zerolog/log"
	"github.com/xuri/excelize/v2"
)

type (
	excel struct {
		sheetName string
	}
)

func (e excel) readFromExcel(structSource reflect.Value, file multipart.File, mapContent map[string]interface{}, ctx ...context.Context) <-chan interface{} {
	chanOut := make(chan interface{})
	go func() {

		xlsFile, err := excelize.OpenReader(file)
		if err != nil {
			log.Error().Msgf("err when open file: %s", err.Error())
			return
		}

		defer func(xlsFile *excelize.File) {
			if err = xlsFile.Close(); err != nil {
				log.Error().Msgf("Failed to close xls file: %s", err.Error())
			}
		}(xlsFile)

		rows, err := xlsFile.GetRows(e.sheetName)

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
