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
	"github.com/rs/zerolog/log"

	"github.com/kodekoding/phastos/v2/go/api"
	contextinternal "github.com/kodekoding/phastos/v2/go/context"
	"github.com/kodekoding/phastos/v2/go/database"
	"github.com/kodekoding/phastos/v2/go/env"
	"github.com/kodekoding/phastos/v2/go/helper"
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
		mapContent        map[string]interface{}
		processName       string
		excel
		csv
	}
	ImportOptions func(reader *importer)

	ImportResult struct {
		FailedList    map[string][]interface{} `json:"failed_list"`
		TotalData     int                      `json:"total_data"`
		TotalFailed   int                      `json:"total_failed"`
		ExecutionTime float64                  `json:"execution_time"`
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
		reader.mapContent = helper.ConvertStructToMap(structDestination)
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

func (r *importer) resetField() {
	r.file = nil
	r.trx = nil
	r.structDestination = nil
	r.fn = nil
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
	log.Info().Msgf("success inserted %d/%d rows in %.2f second(s)", totalData-totalFailed, totalData, executionTime)
	return result
}

func (r *importer) processData(asyncContext context.Context, nrTrx *newrelic.Transaction) *ImportResult {
	var chanRowData = make(<-chan interface{})
	switch r.sourceType {
	case ExcelFileType, ExcelWorkbookFileType:
		chanRowData = r.readFromExcel(r.structDestReflVal, r.file, r.mapContent)
	case CSVFileType:
		chanRowData = r.readFromCSV(r.structDestReflVal, r.file, r.mapContent)
	}
	errChan := r.processEachData(asyncContext, chanRowData, nrTrx)

	totalData := 0
	totalFailed := 0
	result := new(ImportResult)
	var failedList map[string][]any
	for newErr := range errChan {
		if newErr != nil {
			if failedList == nil {
				failedList = make(map[string][]any)
			}
			if _, exist := failedList[newErr.Message]; !exist {
				failedList[newErr.Message] = make([]any, 0)
			}
			if newErr.Message != "Failed Parsed Single Data" {
				failedList[newErr.Message] = append(failedList[newErr.Message], newErr.Data.(map[string]any))
			} else {
				failedList[newErr.Message] = append(failedList[newErr.Message], "failed")
			}
			totalFailed++
		}
		totalData++
	}

	if failedList != nil {
		log.Error().Interface("error_data", failedList).Msgf("Error when import data from %s", r.sourceType)
	}

	result.FailedList = failedList
	result.TotalData = totalData
	result.TotalFailed = totalFailed

	return result
}

func (r importer) processEachData(ctx context.Context, data <-chan interface{}, nrTx *newrelic.Transaction) <-chan *api.HttpError {
	errChan := make(chan *api.HttpError)
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
					if err := api.ValidateStruct(dt); err != nil {
						errData := map[string]interface{}{
							"validation_error": err,
							"data":             dt,
						}
						errChan <- api.NewErr(api.WithErrorData(errData), api.WithErrorStatus(400), api.WithErrorMessage("Error Validation Struct"))
					} else {
						trx, errTrx := r.trx.Begin()
						errFn := r.fn(ctx, dt, trx, wi)
						if errFn != nil {
							// set `errTrx` to rollback the transaction
							errTrx = errors.New("something went wrong")
						}
						r.trx.Finish(trx, errTrx)

						errChan <- errFn
					}
				}
				wait.Done()
			}(workerIndex, workerTrx)
		}
	}(newAsyncTrx)

	go func() {
		wait.Wait()
		close(errChan)
	}()

	return errChan
}
