package importer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"github.com/kodekoding/phastos/go/database"
	"github.com/kodekoding/phastos/v2/go/api"
	contextinternal "github.com/kodekoding/phastos/v2/go/context"
	"github.com/kodekoding/phastos/v2/go/entity"
	"github.com/kodekoding/phastos/v2/go/env"
	"github.com/kodekoding/phastos/v2/go/helper"
	"github.com/kodekoding/phastos/v2/go/notifications"
)

type (
	processFn func(ctx context.Context, singleData interface{}, trx *sql.Tx, wi int) *api.HttpError
	importer  struct {
		ctx               context.Context
		structDestination interface{}
		file              multipart.File
		trx               database.Transactions
		fn                processFn
		notif             notifications.Platforms
		jwtData           *entity.JWTClaimData
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
	ExcelFileType     = "excel"
	CSVFileType       = "csv"
	UndefinedFileType = ""
)

func New(opt ...ImportOptions) *importer {
	csvImporter := new(importer)

	// set default worker to 10
	csvImporter.worker = 10
	csvImporter.excel.sheetName = "Sheet1"

	for _, options := range opt {
		options(csvImporter)
	}

	if csvImporter.ctx != nil {
		csvImporter.notif = csvImporter.ctx.Value(entity.NotifPlatformContext{}).(notifications.Platforms)
		csvImporter.jwtData = contextinternal.GetJWT(csvImporter.ctx)

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
		switch ext {
		case ".xlsx", ".xls":
			reader.sourceType = ExcelFileType
		case ".csv":
			reader.sourceType = CSVFileType
		default:
			reader.sourceType = UndefinedFileType
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

	if err := r.validateField(); err != nil {
		log.Error().Msg(err.Error())
		return nil
	}

	start := time.Now()

	asyncContext := context.Background()
	if r.notif != nil {
		asyncContext = context.WithValue(asyncContext, entity.NotifPlatformContext{}, r.notif)
	}

	if r.jwtData != nil {
		asyncContext = context.WithValue(asyncContext, contextinternal.JwtContext{}, r.jwtData)
	}

	result := r.processData(asyncContext)

	totalData := result.TotalData
	totalFailed := result.TotalFailed

	notifData := make(map[string]string)
	notifType := helper.NotifInfoType
	notifTitle := fmt.Sprintf("Your Data (%d data) Successfully Imported", totalData)
	if result != nil && totalFailed > 0 {
		for errGroup, errList := range result.FailedList {
			errKey := fmt.Sprintf("-%s (%d data)", errGroup, len(errList))
			errData, _ := json.Marshal(errList)
			notifData[errKey] = string(errData)
		}

		notifType = helper.NotifErrorType
		notifTitle = "Your Import Data is something wrong"
	}

	notifTitle = fmt.Sprintf("%s on %s", notifTitle, env.ServiceEnv())

	if r.jwtData != nil {
		jwtData, _ := json.Marshal(r.jwtData.Data)
		notifData["-jwt data"] = string(jwtData)
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

func (r *importer) processData(asyncContext context.Context) *ImportResult {
	var chanRowData = make(<-chan interface{})
	if r.sourceType == ExcelFileType {
		chanRowData = r.readFromExcel(r.structDestReflVal, r.file, r.mapContent)
	} else if r.sourceType == CSVFileType {
		chanRowData = r.readFromCSV(r.structDestReflVal, r.file, r.mapContent)
	}
	errChan := r.processEachData(asyncContext, chanRowData)

	totalData := 0
	totalFailed := 0
	result := new(ImportResult)
	failedList := make(map[string][]interface{})
	for newErr := range errChan {
		if newErr != nil {
			if _, exist := failedList[newErr.Message]; !exist {
				failedList[newErr.Message] = make([]interface{}, 0)
			}
			if newErr.Message != "Failed Parsed Single Data" {
				failedList[newErr.Message] = append(failedList[newErr.Message], newErr.Data.(map[string]interface{}))
			} else {
				failedList[newErr.Message] = append(failedList[newErr.Message], "failed")
			}
			totalFailed++
		}
		totalData++
	}

	result.FailedList = failedList
	result.TotalData = totalData
	result.TotalFailed = totalFailed

	return result
}

func (r importer) processEachData(ctx context.Context, data <-chan interface{}) <-chan *api.HttpError {
	errChan := make(chan *api.HttpError)
	wait := new(sync.WaitGroup)
	wait.Add(r.worker)
	go func() {
		for workerIndex := 0; workerIndex < r.worker; workerIndex++ {
			go func(wi int) {
				for dt := range data {
					if err := api.ValidateStruct(dt); err != nil {
						errChan <- api.NewErr(api.WithErrorData(err), api.WithErrorStatus(400))
					} else {
						trx, errTrx := r.trx.Begin()
						errTrx = r.fn(ctx, dt, trx, wi)
						r.trx.Finish(trx, errTrx)

						errChan <- errors.Cause(errTrx).(*api.HttpError)
					}
				}
				wait.Done()
			}(workerIndex)
		}
	}()

	go func() {
		wait.Wait()
		close(errChan)
	}()

	return errChan
}
