package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/pkg/errors"

	"github.com/kodekoding/phastos/v2/go/api"
	contextinternal "github.com/kodekoding/phastos/v2/go/context"
	"github.com/kodekoding/phastos/v2/go/database"
	"github.com/kodekoding/phastos/v2/go/env"
	"github.com/kodekoding/phastos/v2/go/helper"
	plog "github.com/kodekoding/phastos/v2/go/log"
	"github.com/kodekoding/phastos/v2/go/monitoring"
)

type (
	processFn func(ctx context.Context, singleData interface{}, trx *sqlx.Tx, wi int) *api.HttpError
	importer  struct {
		ctx               context.Context
		structDestination interface{}
		file              multipart.File
		trx               database.Transactions
		fn                processFn
		dataListReflVal   reflect.Value
		structDestReflVal reflect.Value
		sentNotifToSlack  bool
		slackNotifChannel string
		sheetName         string
		sourceType        string
		worker            int
		processName       string
		excel
		csv

		// pivot config
		headerRowIndex int
		dataStartRow   int
		keyColumns     []int
		keySeparator   string
		valueStartCol  int
		onPivotEntry   func(key, value string)
	}
	ImportOptions func(reader *importer)

	ImportResult struct {
		FailedList       map[string][]interface{} `json:"failed_list"`
		SuccessList      []any                    `json:"success_list"`
		TotalData        int                      `json:"total_data"`
		TotalFailed      int                      `json:"total_failed"`
		TotalSuccess     int                      `json:"total_success"`
		ExecutionTime    float64                  `json:"execution_time"`
		UniqueProcessKey string                   `json:"unique_process_key,omitempty"`
	}

	// rowData is the internal type sent through the channel from file readers to workers.
	rowData struct {
		ParsedStruct any
		RawData      map[string]any
	}

	// processedResult is the internal type sent from workers back to the aggregator.
	processedResult struct {
		rowData
		Error *api.HttpError
	}
)

const (
	ExcelFileType         = "excel"
	ExcelWorkbookFileType = "excel_workbook"
	CSVFileType           = "csv"
	UndefinedFileType     = ""

	ExcelExt         = ".xls"
	ExcelWorkbookExt = ".xlsx"
	CSVExt           = ".csv"
)

var mapFileExt = map[string]string{
	ExcelExt:         ExcelFileType,
	ExcelWorkbookExt: ExcelWorkbookFileType,
	CSVExt:           CSVFileType,
}

func New(opt ...ImportOptions) *importer {
	csvImporter := new(importer)

	// set default worker to 10
	csvImporter.worker = 10

	for _, options := range opt {
		options(csvImporter)
	}

	return csvImporter
}

func WithFile(file multipart.File) ImportOptions {
	return func(reader *importer) {
		reader.file = file
	}
}

func WithWorker(totalWorker int) ImportOptions {
	return func(reader *importer) {
		reader.worker = totalWorker
	}
}

func WithExtFile(ext string) ImportOptions {
	return func(reader *importer) {
		if val, exist := mapFileExt[ext]; !exist {
			reader.sourceType = UndefinedFileType
		} else {
			reader.sourceType = val
			reader.excel.fileType = val
		}
	}
}

func WithStructDestination(structDestination interface{}) ImportOptions {
	return func(reader *importer) {
		reader.structDestination = structDestination
	}
}

func WithTransaction(trx database.Transactions) ImportOptions {
	return func(reader *importer) {
		reader.trx = trx
	}
}

func WithProcessFn(fn processFn) ImportOptions {
	return func(reader *importer) {
		reader.fn = fn
	}
}

func WithProcessName(processName string) ImportOptions {
	return func(reader *importer) {
		reader.processName = processName
	}
}

func WithSentNotifToSlack(sent bool, channel ...string) ImportOptions {
	return func(reader *importer) {
		reader.sentNotifToSlack = sent
		reader.slackNotifChannel = os.Getenv("NOTIFICATION_SLACK_INFO_WEBHOOK")
		if channel != nil && len(channel) > 0 {
			reader.slackNotifChannel = channel[0]
		}
	}
}

func WithCtx(ctx context.Context) ImportOptions {
	return func(reader *importer) {
		reader.ctx = ctx
	}
}

func WithSheetName(sheetName string) ImportOptions {
	return func(reader *importer) {
		reader.excel.sheetName = sheetName
	}
}

func WithHeaderRowIndex(idx int) ImportOptions {
	return func(reader *importer) {
		reader.headerRowIndex = idx
	}
}

func WithDataStartRow(row int) ImportOptions {
	return func(reader *importer) {
		reader.dataStartRow = row
	}
}

func WithKeyColumns(cols []int) ImportOptions {
	return func(reader *importer) {
		reader.keyColumns = cols
	}
}

func WithKeySeparator(sep string) ImportOptions {
	return func(reader *importer) {
		reader.keySeparator = sep
	}
}

func WithValueStartCol(col int) ImportOptions {
	return func(reader *importer) {
		reader.valueStartCol = col
	}
}

func WithOnPivotEntry(fn func(key, value string)) ImportOptions {
	return func(reader *importer) {
		reader.onPivotEntry = fn
	}
}

func (r *importer) resetField() {
	r.file = nil
	r.trx = nil
	r.structDestination = nil
	r.fn = nil
}

// buildRowMap creates a new map for a single row, mapping header names to cell values.
func buildRowMap(headers []string, row []string) map[string]any {
	rowMap := make(map[string]any, len(headers))
	for i, header := range headers {
		if i < len(row) {
			rowMap[header] = row[i]
		} else {
			rowMap[header] = ""
		}
	}
	return rowMap
}

func (r *importer) validateField() error {
	if r.file == nil {
		return errors.New("`File` is null, please provide the file")
	}

	if r.structDestination == nil {
		return errors.New("`Struct Destination` Variable is null, please provide the variable")

	}

	if r.trx == nil {
		return errors.New("`Transaction` Variable is null, please provide the transactions")
	}

	if r.sourceType == UndefinedFileType {
		return errors.New("File Type isn't set")
	}

	reflectVal := reflect.ValueOf(r.structDestination)

	if reflectVal.Kind() == reflect.Ptr {
		reflectVal = reflectVal.Elem()
	}

	if reflectVal.Kind() != reflect.Struct {
		return errors.New("data destination should be a struct")
	}

	r.structDestReflVal = reflectVal
	return nil
}

func (r *importer) ProcessData() *ImportResult {
	log := plog.Ctx(r.ctx)
	txn := monitoring.BeginTrxFromContext(r.ctx)
	var importProcessSegment *newrelic.Segment
	if txn != nil {
		importProcessSegment = txn.StartSegment("Importer-ProcessImportData")
		importProcessSegment.AddAttribute("process_name", r.processName)
		importProcessSegment.AddAttribute("file_type", r.sourceType)
		defer importProcessSegment.End()
	}
	if err := r.validateField(); err != nil {
		log.Error().Msg(err.Error())
		return nil
	}

	start := time.Now()

	asyncContext := contextinternal.CreateAsyncContext(r.ctx)

	result := r.processData(asyncContext, txn)

	totalData := result.TotalData
	totalFailed := result.TotalFailed

	importProcessSegment.AddAttribute("result", map[string]int{
		"total_data":   totalData,
		"total_failed": totalFailed,
	})

	notifData := make(map[string]string)
	notifType := helper.NotifInfoType
	notifTitle := fmt.Sprintf("Your Data (%d data) Successfully Imported", totalData)
	if totalFailed > 0 {
		for errGroup, errList := range result.FailedList {
			errKey := fmt.Sprintf("-%s (%d data)", errGroup, len(errList))
			errData, _ := json.Marshal(errList)
			notifData[errKey] = string(errData)
		}

		notifType = helper.NotifErrorType
		notifTitle = "Your Import Data is something wrong"
	}

	notifTitle = fmt.Sprintf("%s on %s", notifTitle, env.ServiceEnv())

	jwtCtx := contextinternal.GetJWT(asyncContext)
	if jwtCtx != nil {
		jwtData, _ := json.Marshal(jwtCtx.Data)
		notifData["-jwt data"] = string(jwtData)
		importProcessSegment.AddAttribute("executed_by", string(jwtData))
	}
	notifData["-process name"] = fmt.Sprintf("Import Data %s from %s", r.processName, r.sourceType)

	end := time.Since(start)
	notifData["total_data"] = fmt.Sprintf("%d", totalData)
	notifData["time_execution"] = fmt.Sprintf("%.2f second(s)", end.Seconds())
	go func() {
		if r.sentNotifToSlack {
			_ = helper.SendSlackNotification(
				asyncContext,
				helper.NotifTitle(notifTitle),
				helper.NotifMsgType(notifType),
				helper.NotifData(notifData),
				helper.NotifChannel(r.slackNotifChannel),
			)
		}
	}()
	executionTime := end.Seconds()
	result.ExecutionTime = executionTime
	log.Info().
		Int("success_processing_data", totalData-totalFailed).
		Int("total_data", totalData).
		Float64("processing_time", executionTime).
		Msg("Success Import Data")
	return result
}

// ReadPivotData reads a file with pivot/custom layout and returns
// a flat key-value map without processing through the worker pipeline.
// Use pivot options to configure the layout:
// WithHeaderRowIndex, WithDataStartRow, WithKeyColumns, WithValueStartCol.
// Optional: WithSheetName, WithKeySeparator, WithOnPivotEntry.
func (r *importer) ReadPivotData() (*PivotReadResult, error) {
	log := plog.Get()
	if r.file == nil {
		return nil, errors.New("`File` is null, please provide the file")
	}
	if r.sourceType == UndefinedFileType {
		return nil, errors.New("File Type isn't set")
	}

	start := time.Now()

	config := pivotReadConfig{
		File:           r.file,
		FileType:       r.sourceType,
		SheetName:      r.excel.sheetName,
		HeaderRowIndex: r.headerRowIndex,
		DataStartRow:   r.dataStartRow,
		KeyColumns:     r.keyColumns,
		KeySeparator:   r.keySeparator,
		ValueStartCol:  r.valueStartCol,
		OnEntry:        r.onPivotEntry,
	}

	result, err := readPivot(config)
	if err != nil {
		log.Err(err).Msg("[IMPORTER][PHASTOS] - ReadPivotData failed")
		return nil, err
	}

	end := time.Since(start)
	log.Info().
		Int("total_entries", len(result.Data)).
		Int("total_headers", len(result.Headers)).
		Float64("processing_time", end.Seconds()).
		Msg("Success Read Pivot Data")

	return result, nil
}

// ProcessPivotData reads a file with pivot/custom layout and processes each
// entry through the worker pool, just like ProcessData does for regular imports.
// Each entry passed to processFn is a map[string]any with:
//   - Key column values (e.g., "Employee ID": "444201123")
//   - "pivot_header": the value column header (e.g., "2024-03-26")
//   - "pivot_value":  the cell value (e.g., "HONS")
//
// Requires: WithFile, WithExtFile, WithTransaction, WithProcessFn, and pivot options.
func (r *importer) ProcessPivotData() *ImportResult {
	log := plog.Get()
	if r.file == nil {
		log.Error().Msg("`File` is null, please provide the file")
		return nil
	}
	if r.sourceType == UndefinedFileType {
		log.Error().Msg("File Type isn't set")
		return nil
	}
	if r.fn == nil {
		log.Error().Msg("`ProcessFn` is null, please provide the process function")
		return nil
	}

	start := time.Now()

	config := pivotReadConfig{
		File:           r.file,
		FileType:       r.sourceType,
		SheetName:      r.excel.sheetName,
		HeaderRowIndex: r.headerRowIndex,
		DataStartRow:   r.dataStartRow,
		KeyColumns:     r.keyColumns,
		KeySeparator:   r.keySeparator,
		ValueStartCol:  r.valueStartCol,
	}

	ctx := r.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	asyncContext := contextinternal.CreateAsyncContext(ctx)

	chanPivotData := readPivotChannel(config)
	resultChan := r.processEachData(asyncContext, chanPivotData, nil)

	totalData := 0
	totalFailed := 0
	result := new(ImportResult)
	var failedList map[string][]any
	var successList []any
	for pr := range resultChan {
		if pr.Error != nil {
			if failedList == nil {
				failedList = make(map[string][]any)
			}
			if _, exist := failedList[pr.Error.Message]; !exist {
				failedList[pr.Error.Message] = make([]any, 0)
			}
			if pr.Error.Data != nil {
				failedList[pr.Error.Message] = append(failedList[pr.Error.Message], pr.Error.Data)
			} else {
				failedList[pr.Error.Message] = append(failedList[pr.Error.Message], "failed")
			}
			totalFailed++
		} else {
			successList = append(successList, pr.RawData)
		}
		totalData++
	}

	if failedList != nil {
		log.Error().Any("error_data", failedList).Msgf("Error when processing pivot data from %s", r.sourceType)
	}

	result.FailedList = failedList
	result.SuccessList = successList
	result.TotalData = totalData
	result.TotalFailed = totalFailed
	result.TotalSuccess = totalData - totalFailed

	notifData := make(map[string]string)
	notifType := helper.NotifInfoType
	notifTitle := fmt.Sprintf("Your Pivot Data (%d data) Successfully Processed", totalData)
	if totalFailed > 0 {
		for errGroup, errList := range result.FailedList {
			errKey := fmt.Sprintf("-%s (%d data)", errGroup, len(errList))
			errData, _ := json.Marshal(errList)
			notifData[errKey] = string(errData)
		}
		notifType = helper.NotifErrorType
		notifTitle = "Your Pivot Import Data is something wrong"
	}
	notifTitle = fmt.Sprintf("%s on %s", notifTitle, env.ServiceEnv())
	notifData["-process name"] = fmt.Sprintf("Import Pivot Data %s from %s", r.processName, r.sourceType)

	end := time.Since(start)
	notifData["total_data"] = fmt.Sprintf("%d", totalData)
	notifData["time_execution"] = fmt.Sprintf("%.2f second(s)", end.Seconds())
	go func() {
		if r.sentNotifToSlack {
			_ = helper.SendSlackNotification(
				asyncContext,
				helper.NotifTitle(notifTitle),
				helper.NotifMsgType(notifType),
				helper.NotifData(notifData),
				helper.NotifChannel(r.slackNotifChannel),
			)
		}
	}()

	executionTime := end.Seconds()
	result.ExecutionTime = executionTime
	log.Info().
		Int("success_processing_data", totalData-totalFailed).
		Int("total_data", totalData).
		Float64("processing_time", executionTime).
		Msg("Success Process Pivot Data")
	return result
}

func (r *importer) processData(asyncContext context.Context, nrTrx *newrelic.Transaction) *ImportResult {
	log := plog.Ctx(asyncContext)
	var chanRowData = make(<-chan rowData)
	switch r.sourceType {
	case ExcelFileType, ExcelWorkbookFileType:
		chanRowData = r.readFromExcel(r.structDestReflVal, r.file)
	case CSVFileType:
		chanRowData = r.readFromCSV(r.structDestReflVal, r.file)
	}
	resultChan := r.processEachData(asyncContext, chanRowData, nrTrx)

	totalData := 0
	totalFailed := 0
	result := new(ImportResult)
	var failedList map[string][]any
	var successList []any
	for pr := range resultChan {
		if pr.Error != nil {
			if failedList == nil {
				failedList = make(map[string][]any)
			}
			if _, exist := failedList[pr.Error.Message]; !exist {
				failedList[pr.Error.Message] = make([]any, 0)
			}
			if pr.Error.Message != "Failed Parsed Single Data" {
				failedList[pr.Error.Message] = append(failedList[pr.Error.Message], pr.Error.Data.(map[string]any))
			} else {
				failedList[pr.Error.Message] = append(failedList[pr.Error.Message], "failed")
			}
			totalFailed++
		} else {
			successList = append(successList, pr.ParsedStruct)
		}
		totalData++
	}

	if failedList != nil {
		log.Error().Any("error_data", failedList).Msgf("Error when import data from %s", r.sourceType)
	}

	result.FailedList = failedList
	result.SuccessList = successList
	result.TotalData = totalData
	result.TotalFailed = totalFailed
	result.TotalSuccess = totalData - totalFailed

	return result
}

func (r importer) processEachData(ctx context.Context, data <-chan rowData, nrTx *newrelic.Transaction) <-chan *processedResult {
	resultChan := make(chan *processedResult)
	wait := new(sync.WaitGroup)
	wait.Add(r.worker)
	// for backward compatibility, if the user didn't use new relic as monitoring
	var newAsyncTrx *newrelic.Transaction
	if nrTx != nil {
		newAsyncTrx = nrTx.NewGoroutine()
	}
	go func(txn *newrelic.Transaction) {
		// for backward compatibility
		var workerTrx *newrelic.Transaction
		if nrTx != nil {
			workerTrx = txn.NewGoroutine()
		}
		for workerIndex := 0; workerIndex < r.worker; workerIndex++ {
			go func(wi int, txnWorker *newrelic.Transaction) {
				// for backward compatibility
				if txnWorker != nil {
					workerSegment := txnWorker.StartSegment(fmt.Sprintf("ImporterProcessEachData-Worker-%d", wi+1))
					defer workerSegment.End()
				}
				for dt := range data {
					pr := &processedResult{rowData: dt}
					skipProcess := false
					// only validate struct for regular import (not pivot)
					if r.structDestination != nil {
						if err := api.ValidateStruct(dt.ParsedStruct); err != nil {
							errData := map[string]interface{}{
								"validation_error": err,
								"data":             dt.ParsedStruct,
							}
							pr.Error = api.NewErr(api.WithErrorData(errData), api.WithErrorStatus(400), api.WithErrorMessage("Error Validation Struct"))
							skipProcess = true
						}
					}
					if !skipProcess {
						trx, errTrx := r.trx.Begin()
						errFn := r.fn(ctx, dt.ParsedStruct, trx, wi)
						if errFn != nil {
							// set `errTrx` to rollback the transaction
							errTrx = errors.New("something went wrong")
							pr.Error = errFn
						}
						r.trx.Finish(trx, errTrx)
					}
					resultChan <- pr
				}
				wait.Done()
			}(workerIndex, workerTrx)
		}
	}(newAsyncTrx)

	go func() {
		wait.Wait()
		close(resultChan)
	}()

	return resultChan
}
