package importer

import (
	"context"
	"encoding/json"
	"github.com/extrame/xls"
	"mime/multipart"
	"reflect"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/xuri/excelize/v2"
)

type (
	excel struct {
		sheetName string
	}
)

func GetDataFromXlsx(file multipart.File, sheetName string) ([][]string, error) {
	xlsFile, err := excelize.OpenReader(file)
	if err != nil {
		log.Err(err).Msg("err when open file")
		return nil, err
	}

	defer func(xlsFile *excelize.File) {
		if err = xlsFile.Close(); err != nil {
			log.Err(err).Msg("Failed to close xlsx file")
		}
	}(xlsFile)
	defaultSheetName := xlsFile.GetSheetName(0)
	if sheetName != "" {
		defaultSheetName = sheetName
	}
	return xlsFile.GetRows(defaultSheetName)
}

func GetDataFromXls(file multipart.File) ([][]string, error) {
	xlsFile, err := xls.OpenReader(file, "utf-8")
	if err != nil {
		log.Err(err).Msg("err when open file")
		return nil, err
	}

	sheetData := xlsFile.GetSheet(0)
	maxRow := int(sheetData.MaxRow)
	var dataList [][]string
	for rowIndex := 0; rowIndex < maxRow; rowIndex++ {
		var rowData []string
		row := sheetData.Row(rowIndex)
		lastCol := row.LastCol()
		for columnIndex := 0; columnIndex < lastCol; columnIndex++ {
			rowData = append(rowData, row.Col(columnIndex))
		}

		dataList = append(dataList, rowData)
	}
	return dataList, nil
}

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
				headerName := strings.Replace(rows[0][x], "*", "", -1)
				mapContent[headerName] = rowData
			}
			dt, _ := json.Marshal(mapContent)
			_ = json.Unmarshal(dt, destStruct)
			chanOut <- destStruct
		}
		close(chanOut)
	}()
	return chanOut
}
