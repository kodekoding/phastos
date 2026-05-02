package importer

import (
	"context"
	"encoding/json"
	"mime/multipart"
	"reflect"
	"strings"

	"github.com/extrame/xls"
	"github.com/rs/zerolog"
	"github.com/xuri/excelize/v2"

	plog "github.com/kodekoding/phastos/v2/go/log"
)

type (
	excel struct {
		sheetName string
		fileType  string
	}
)

func GetDataFromXlsx(file multipart.File, sheetName string) ([][]string, error) {
	log := plog.Get()
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
	log := plog.Get()
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

func (e excel) readFromExcel(structSource reflect.Value, file multipart.File, ctx ...context.Context) <-chan rowData {
	log := plog.Get()
	chanOut := make(chan rowData)
	go func() {
		defer close(chanOut)

		if e.fileType == ExcelFileType {
			// xls format: extrame/xls doesn't support streaming, process rows directly
			e.readFromXls(structSource, file, chanOut, log)
		} else {
			// xlsx format: use excelize streaming Rows() API
			e.readFromXlsxStream(structSource, file, chanOut, log)
		}
	}()
	return chanOut
}

// readFromXlsxStream uses excelize's Rows() streaming iterator to avoid loading
// the entire sheet into memory.
func (e excel) readFromXlsxStream(structSource reflect.Value, file multipart.File, chanOut chan<- rowData, log zerolog.Logger) {
	xlsFile, err := excelize.OpenReader(file)
	if err != nil {
		log.Err(err).Msg("[IMPORTER][PHASTOS] - Open xlsx file for streaming")
		return
	}
	defer func() {
		if err = xlsFile.Close(); err != nil {
			log.Err(err).Msg("[IMPORTER][PHASTOS] - Failed to close xlsx file")
		}
	}()

	sheetName := xlsFile.GetSheetName(0)
	if e.sheetName != "" {
		sheetName = e.sheetName
	}

	rows, err := xlsFile.Rows(sheetName)
	if err != nil {
		log.Err(err).Msg("[IMPORTER][PHASTOS] - Get streaming rows from xlsx")
		return
	}
	defer rows.Close() //nolint:errcheck

	var headers []string
	for rows.Next() {
		cols, err := rows.Columns()
		if err != nil {
			log.Err(err).Msg("[IMPORTER][PHASTOS] - Read columns from xlsx row")
			continue
		}

		// first row is the header
		if headers == nil {
			headers = make([]string, len(cols))
			for i, col := range cols {
				headers[i] = strings.ReplaceAll(col, "*", "")
			}
			continue
		}

		rowMap := buildRowMap(headers, cols)

		destStruct := reflect.New(structSource.Type()).Interface()
		dt, _ := json.Marshal(rowMap)
		_ = json.Unmarshal(dt, destStruct)

		chanOut <- rowData{
			ParsedStruct: destStruct,
			RawData:      rowMap,
		}
	}
}

// readFromXls processes xls rows directly without collecting into [][]string first.
func (e excel) readFromXls(structSource reflect.Value, file multipart.File, chanOut chan<- rowData, log zerolog.Logger) {
	xlsFile, err := xls.OpenReader(file, "utf-8")
	if err != nil {
		log.Err(err).Msg("[IMPORTER][PHASTOS] - Open xls file")
		return
	}

	sheetData := xlsFile.GetSheet(0)
	maxRow := int(sheetData.MaxRow)

	// read headers from first row
	var headers []string
	if maxRow > 0 {
		headerRow := sheetData.Row(0)
		lastCol := headerRow.LastCol()
		headers = make([]string, lastCol)
		for i := 0; i < lastCol; i++ {
			headers[i] = strings.ReplaceAll(headerRow.Col(i), "*", "")
		}
	}

	// process data rows directly (skip header at index 0)
	for rowIndex := 1; rowIndex < maxRow; rowIndex++ {
		row := sheetData.Row(rowIndex)
		lastCol := row.LastCol()

		rowCols := make([]string, lastCol)
		for colIndex := 0; colIndex < lastCol; colIndex++ {
			rowCols[colIndex] = row.Col(colIndex)
		}

		rowMap := buildRowMap(headers, rowCols)

		destStruct := reflect.New(structSource.Type()).Interface()
		dt, _ := json.Marshal(rowMap)
		_ = json.Unmarshal(dt, destStruct)

		chanOut <- rowData{
			ParsedStruct: destStruct,
			RawData:      rowMap,
		}
	}
}
