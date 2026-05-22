package importer

import (
	csvencode "encoding/csv"
	"io"
	"mime/multipart"
	"strings"

	"github.com/pkg/errors"
	"github.com/xuri/excelize/v2"

	plog "github.com/kodekoding/phastos/v2/go/log"
)

// pivotReadConfig configures how to read a pivot/custom-format file
// where column headers become part of the map key.
//
// Example for shift schedule:
//
//	Row 1: Location Name, PT. XYZ ...   (metadata)
//	Row 2: (empty)                       (metadata)
//	Row 3: Employee ID, Employee Name, 2024-03-26, 2024-03-27, ...  (header)
//	Row 4: 444201123,   Fitri,          HONS,       HONS, ...       (data)
//
// Config:
//
//	pivotReadConfig{
//	    HeaderRowIndex: 2,          // row 3 (0-indexed)
//	    DataStartRow:   3,          // row 4 (0-indexed)
//	    KeyColumns:     []int{0},   // Employee ID
//	    ValueStartCol:  2,          // date columns start at index 2
//	    KeySeparator:   ";",
//	}
//
// Result: map["444201123;2024-03-26"] = "HONS"
type pivotReadConfig struct {
	File           multipart.File
	FileType       string // "csv", "excel", "excel_workbook"
	SheetName      string // for excel/excel_workbook only
	HeaderRowIndex int    // 0-indexed row number for header
	DataStartRow   int    // 0-indexed row number where data begins
	KeyColumns     []int  // column indices to build the composite key
	KeySeparator   string // separator between key parts, default ";"
	ValueStartCol  int    // column index where values begin

	// OnEntry is an optional callback invoked for each pivot entry.
	// When set, entries are NOT stored in PivotReadResult.Data (the map will be nil),
	// reducing memory usage for very large files.
	// Signature: func(compositeKey, cellValue string)
	OnEntry func(key, value string)
}

// PivotReadResult holds the output of a pivot read operation.
type PivotReadResult struct {
	// Data is the pivot result: compositeKey -> cellValue.
	// Example: "444201123;2024-03-26" -> "HONS"
	Data map[string]string

	// Headers is the full header row.
	Headers []string

	// MetaRows contains all rows before the header row (metadata).
	MetaRows [][]string
}

// readPivot reads a CSV or Excel file with a custom/pivot layout and returns
// a flat map[string]string where each key is built from the key column values
// joined with the header of the current value column.
func readPivot(config pivotReadConfig) (*PivotReadResult, error) {
	if config.File == nil {
		return nil, errors.New("file is nil")
	}
	if config.KeySeparator == "" {
		config.KeySeparator = ";"
	}
	if config.DataStartRow <= config.HeaderRowIndex {
		config.DataStartRow = config.HeaderRowIndex + 1
	}

	switch config.FileType {
	case CSVFileType:
		return readPivotFromCSV(config)
	case ExcelWorkbookFileType:
		return readPivotFromXlsx(config)
	case ExcelFileType:
		return readPivotFromXls(config)
	default:
		return nil, errors.New("unsupported file type, use 'csv', 'excel', or 'excel_workbook'")
	}
}

// readPivotFromCSV streams CSV row-by-row and builds the pivot map.
func readPivotFromCSV(config pivotReadConfig) (*PivotReadResult, error) {
	csvReader := csvencode.NewReader(config.File)
	// allow variable number of fields per row (metadata rows often differ)
	csvReader.FieldsPerRecord = -1

	result := &PivotReadResult{}

	rowIndex := 0
	var headers []string

	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.Wrap(err, "phastos.importer.readPivot.CSV.Read")
		}

		if rowIndex < config.HeaderRowIndex {
			// metadata row
			result.MetaRows = append(result.MetaRows, row)
			rowIndex++
			continue
		}

		if rowIndex == config.HeaderRowIndex {
			headers = make([]string, len(row))
			copy(headers, row)
			result.Headers = headers
			// pre-allocate map with estimated capacity (streaming: row count unknown)
			if config.OnEntry == nil {
				numValueCols := len(headers) - config.ValueStartCol
				if numValueCols < 0 {
					numValueCols = 0
				}
				result.Data = make(map[string]string, numValueCols*100)
			}
			rowIndex++
			continue
		}

		// skip rows between header and data start
		if rowIndex < config.DataStartRow {
			rowIndex++
			continue
		}

		// data row — build pivot entries
		buildPivotEntries(result.Data, headers, row, config)
		rowIndex++
	}

	return result, nil
}

// readPivotFromXlsx uses excelize streaming Rows() API for xlsx files.
func readPivotFromXlsx(config pivotReadConfig) (*PivotReadResult, error) {
	log := plog.Get()
	xlsFile, err := excelize.OpenReader(config.File)
	if err != nil {
		return nil, errors.Wrap(err, "phastos.importer.readPivot.Xlsx.OpenReader")
	}
	defer func() {
		if err = xlsFile.Close(); err != nil {
			log.Err(err).Msg("[IMPORTER][PHASTOS] - readPivot: Failed to close xlsx file")
		}
	}()

	sheetName := xlsFile.GetSheetName(0)
	if config.SheetName != "" {
		sheetName = config.SheetName
	}

	rows, err := xlsFile.Rows(sheetName)
	if err != nil {
		return nil, errors.Wrap(err, "phastos.importer.readPivot.Xlsx.Rows")
	}
	defer rows.Close() //nolint:errcheck

	result := &PivotReadResult{}

	rowIndex := 0
	var headers []string

	for rows.Next() {
		cols, err := rows.Columns()
		if err != nil {
			log.Err(err).Msg("[IMPORTER][PHASTOS] - readPivot: Read columns from xlsx row")
			rowIndex++
			continue
		}

		if rowIndex < config.HeaderRowIndex {
			result.MetaRows = append(result.MetaRows, cols)
			rowIndex++
			continue
		}

		if rowIndex == config.HeaderRowIndex {
			headers = make([]string, len(cols))
			copy(headers, cols)
			result.Headers = headers
			// pre-allocate map with estimated capacity (streaming: row count unknown)
			if config.OnEntry == nil {
				numValueCols := len(headers) - config.ValueStartCol
				if numValueCols < 0 {
					numValueCols = 0
				}
				result.Data = make(map[string]string, numValueCols*100)
			}
			rowIndex++
			continue
		}

		if rowIndex < config.DataStartRow {
			rowIndex++
			continue
		}

		buildPivotEntries(result.Data, headers, cols, config)
		rowIndex++
	}

	return result, nil
}

// readPivotFromXls reads old .xls format.
func readPivotFromXls(config pivotReadConfig) (*PivotReadResult, error) {
	// reuse existing GetDataFromXls which returns [][]string
	allRows, err := GetDataFromXls(config.File)
	if err != nil {
		return nil, errors.Wrap(err, "phastos.importer.readPivot.Xls.GetData")
	}

	result := &PivotReadResult{}

	for rowIndex, row := range allRows {
		if rowIndex < config.HeaderRowIndex {
			result.MetaRows = append(result.MetaRows, row)
			continue
		}

		if rowIndex == config.HeaderRowIndex {
			result.Headers = make([]string, len(row))
			copy(result.Headers, row)
			// pre-allocate map with exact capacity (xls: row count known)
			if config.OnEntry == nil {
				numDataRows := len(allRows) - config.DataStartRow
				numValueCols := len(row) - config.ValueStartCol
				if numDataRows < 0 {
					numDataRows = 0
				}
				if numValueCols < 0 {
					numValueCols = 0
				}
				result.Data = make(map[string]string, numDataRows*numValueCols)
			}
			continue
		}

		if rowIndex < config.DataStartRow {
			continue
		}

		buildPivotEntries(result.Data, result.Headers, row, config)
	}

	return result, nil
}

// buildPivotEntries builds key-value pairs from a single data row.
// Key format: "keyCol1Value<sep>keyCol2Value<sep>...<sep>headerOfValueCol"
// Value: the cell content at that column.
// When config.OnEntry is set, entries are passed to the callback instead of stored in the map.
func buildPivotEntries(data map[string]string, headers []string, row []string, config pivotReadConfig) {
	// build the key prefix from key columns using pooled strings.Builder (avoids []string alloc + Join)
	keyPrefix := getBuilder()
	for i, colIdx := range config.KeyColumns {
		if i > 0 {
			keyPrefix.WriteString(config.KeySeparator)
		}
		if colIdx < len(row) {
			keyPrefix.WriteString(strings.TrimSpace(row[colIdx]))
		}
	}
	prefix := keyPrefix.String()
	putBuilder(keyPrefix)

	// iterate value columns and build entries using pooled strings.Builder
	compositeKey := getBuilder()
	for colIdx := config.ValueStartCol; colIdx < len(row); colIdx++ {
		if colIdx >= len(headers) {
			break
		}
		headerName := strings.TrimSpace(headers[colIdx])
		if headerName == "" {
			continue
		}

		compositeKey.Reset()
		compositeKey.WriteString(prefix)
		compositeKey.WriteString(config.KeySeparator)
		compositeKey.WriteString(headerName)

		value := strings.TrimSpace(row[colIdx])
		if config.OnEntry != nil {
			config.OnEntry(compositeKey.String(), value)
		} else {
			data[compositeKey.String()] = value
		}
	}
	putBuilder(compositeKey)
}

// ---------------------------------------------------------------------------
// Channel-based pivot streaming for ProcessPivotData pipeline
// ---------------------------------------------------------------------------

// readPivotChannel reads a pivot file and sends each entry as rowData to the returned channel.
// Each entry's ParsedStruct and RawData is a map[string]any containing:
//   - Key column values (keyed by their header names, e.g., "Employee ID": "444201123")
//   - "pivot_header": the header of the value column (e.g., "2024-03-26")
//   - "pivot_value":  the cell value (e.g., "HONS")
func readPivotChannel(config pivotReadConfig) <-chan rowData {
	if config.KeySeparator == "" {
		config.KeySeparator = ";"
	}
	if config.DataStartRow <= config.HeaderRowIndex {
		config.DataStartRow = config.HeaderRowIndex + 1
	}

	chanOut := make(chan rowData)
	go func() {
		defer close(chanOut)
		switch config.FileType {
		case CSVFileType:
			streamPivotCSV(config, chanOut)
		case ExcelWorkbookFileType:
			streamPivotXlsx(config, chanOut)
		case ExcelFileType:
			streamPivotXls(config, chanOut)
		}
	}()
	return chanOut
}

func streamPivotCSV(config pivotReadConfig, chanOut chan<- rowData) {
	csvReader := csvencode.NewReader(config.File)
	csvReader.FieldsPerRecord = -1

	rowIndex := 0
	var headers []string

	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		if rowIndex == config.HeaderRowIndex {
			headers = make([]string, len(row))
			copy(headers, row)
		}

		if rowIndex >= config.DataStartRow && headers != nil {
			sendPivotEntries(chanOut, headers, row, config)
		}

		rowIndex++
	}
}

func streamPivotXlsx(config pivotReadConfig, chanOut chan<- rowData) {
	log := plog.Get()
	xlsFile, err := excelize.OpenReader(config.File)
	if err != nil {
		log.Err(err).Msg("[IMPORTER][PHASTOS] - streamPivotXlsx: Open file")
		return
	}
	defer func() {
		if err = xlsFile.Close(); err != nil {
			log.Err(err).Msg("[IMPORTER][PHASTOS] - streamPivotXlsx: Close file")
		}
	}()

	sheetName := xlsFile.GetSheetName(0)
	if config.SheetName != "" {
		sheetName = config.SheetName
	}

	rows, err := xlsFile.Rows(sheetName)
	if err != nil {
		log.Err(err).Msg("[IMPORTER][PHASTOS] - streamPivotXlsx: Get rows")
		return
	}
	defer rows.Close() //nolint:errcheck

	rowIndex := 0
	var headers []string

	for rows.Next() {
		cols, err := rows.Columns()
		if err != nil {
			log.Err(err).Msg("[IMPORTER][PHASTOS] - streamPivotXlsx: Read columns")
			rowIndex++
			continue
		}

		if rowIndex == config.HeaderRowIndex {
			headers = make([]string, len(cols))
			copy(headers, cols)
		}

		if rowIndex >= config.DataStartRow && headers != nil {
			sendPivotEntries(chanOut, headers, cols, config)
		}

		rowIndex++
	}
}

func streamPivotXls(config pivotReadConfig, chanOut chan<- rowData) {
	allRows, err := GetDataFromXls(config.File)
	if err != nil {
		log := plog.Get()
		log.Err(err).Msg("[IMPORTER][PHASTOS] - streamPivotXls: Get data")
		return
	}

	var headers []string
	for rowIndex, row := range allRows {
		if rowIndex == config.HeaderRowIndex {
			headers = make([]string, len(row))
			copy(headers, row)
		}

		if rowIndex >= config.DataStartRow && headers != nil {
			sendPivotEntries(chanOut, headers, row, config)
		}
	}
}

// sendPivotEntries builds a rich map for each pivot entry and sends it to the channel.
// Each entry map contains key column values (using their header names), plus "pivot_header" and "pivot_value".
func sendPivotEntries(chanOut chan<- rowData, headers []string, row []string, config pivotReadConfig) {
	for colIdx := config.ValueStartCol; colIdx < len(row); colIdx++ {
		if colIdx >= len(headers) {
			break
		}
		headerName := strings.TrimSpace(headers[colIdx])
		if headerName == "" {
			continue
		}

		entryMap := make(map[string]any)
		for _, keyColIdx := range config.KeyColumns {
			if keyColIdx < len(headers) && keyColIdx < len(row) {
				entryMap[strings.TrimSpace(headers[keyColIdx])] = strings.TrimSpace(row[keyColIdx])
			}
		}
		entryMap["pivot_header"] = headerName
		entryMap["pivot_value"] = strings.TrimSpace(row[colIdx])

		chanOut <- rowData{
			ParsedStruct: entryMap,
			RawData:      entryMap,
		}
	}
}
