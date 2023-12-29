package csv

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/gocarina/gocsv"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"github.com/kodekoding/phastos/go/database"
	"github.com/kodekoding/phastos/v2/go/api"
	contextinternal "github.com/kodekoding/phastos/v2/go/context"
	"github.com/kodekoding/phastos/v2/go/entity"
	"github.com/kodekoding/phastos/v2/go/helper"
	"github.com/kodekoding/phastos/v2/go/notifications"
)

type (
	processFn func(ctx context.Context, singleData interface{}, trx *sql.Tx) *api.HttpError
	importer  struct {
		ctx             context.Context
		csvRowsData     interface{}
		file            multipart.File
		trx             database.Transactions
		fn              processFn
		notif           notifications.Platforms
		jwtData         interface{}
		dataListReflVal reflect.Value
	}
	ImportOptions func(reader *importer)
)

func NewImport(opt ...ImportOptions) *importer {
	csvImporter := new(importer)
	for _, options := range opt {
		options(csvImporter)
	}

	if csvImporter.ctx != nil {
		csvImporter.notif = csvImporter.ctx.Value(entity.NotifPlatformContext{}).(notifications.Platforms)
		jwtCtx := contextinternal.GetJWT(csvImporter.ctx)
		if jwtCtx != nil {
			csvImporter.jwtData = jwtCtx.Data
		}
	}

	return csvImporter
}

func WithFile(file multipart.File) ImportOptions {
	return func(reader *importer) {
		reader.file = file
	}
}

func WithDataList(dataList interface{}) ImportOptions {
	return func(reader *importer) {
		reader.csvRowsData = dataList
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

func WithCtx(ctx context.Context) ImportOptions {
	return func(reader *importer) {
		reader.ctx = ctx
	}
}

func (r *importer) resetField() {
	r.file = nil
	r.trx = nil
	r.csvRowsData = nil
	r.fn = nil
}

func (r *importer) validateField() error {
	if r.file == nil {
		return errors.New("`File` is null, please provide the file")
	}

	if r.csvRowsData == nil {
		return errors.New("`Data List` Variable is null, please provide the variable")

	}

	if r.trx == nil {
		return errors.New("`Transaction` Variable is null, please provide the transactions")
	}

	reflectVal := reflect.ValueOf(r.csvRowsData)

	if reflectVal.Kind() == reflect.Ptr {
		reflectVal = reflectVal.Elem()
	}

	if reflectVal.Kind() != reflect.Slice {
		return errors.New("data list should be an array/slice")
	}
	r.dataListReflVal = reflectVal
	return nil
}

func (r *importer) ProcessData() {

	if err := r.validateField(); err != nil {
		log.Error().Msg(err.Error())
		return
	}

	start := time.Now()
	gocsv.SetCSVReader(func(reader io.Reader) gocsv.CSVReader {
		r := csv.NewReader(reader)
		r.LazyQuotes = true
		return r
	})
	if err := gocsv.Unmarshal(r.file, r.csvRowsData); err != nil {
		log.Error().Msgf("Failed to marshal the data: %s", err.Error())
		return
	}

	log.Info().Msg("will start import the data asynchronously")
	asyncContext := context.Background()
	if r.notif != nil {
		asyncContext = context.WithValue(asyncContext, entity.NotifPlatformContext{}, r.notif)
	}
	go r.processData(asyncContext, start)
}

func (r *importer) processData(asyncContext context.Context, start time.Time) {
	defer r.resetField()

	mtx := new(sync.Mutex)
	wg := new(sync.WaitGroup)

	trx, err := r.trx.Begin()
	defer r.trx.Finish(trx, err)
	errChan := make(chan *api.HttpError, 0)

	// main process each row
	go r.processEachData(asyncContext, r.dataListReflVal, r.fn, wg, mtx, trx, errChan)

	totalData := 0
	totalFailed := 0
	failedList := make(map[string][]interface{})
	for newErr := range errChan {
		if newErr.Message != "no error" {
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

	notifData := make(map[string]string)
	notifType := helper.NotifInfoType
	notifTitle := fmt.Sprintf("Your Data (%d data) Successfully Imported", totalData)
	if failedList != nil && totalFailed > 0 {
		for errGroup, errList := range failedList {
			errKey := fmt.Sprintf("-%s (%d data)", errGroup, len(errList))
			errData, _ := json.Marshal(errList)
			notifData[errKey] = string(errData)
		}

		notifType = helper.NotifErrorType
		notifTitle = "Your Import Data is something wrong"

	}

	if r.jwtData != nil {
		jwtData, _ := json.Marshal(r.jwtData)
		notifData["-jwt data"] = string(jwtData)
	}

	end := time.Since(start)
	notifData["total_data"] = fmt.Sprintf("%d", totalData)
	notifData["time_execution"] = fmt.Sprintf("%.2f second(s)", end.Seconds())
	_ = helper.SendSlackNotification(
		asyncContext,
		helper.NotifTitle(notifTitle),
		helper.NotifMsgType(notifType),
		helper.NotifData(notifData),
		helper.NotifChannel(os.Getenv("NOTIFICATION_SLACK_INFO_WEBHOOK")),
	)
	log.Printf("success inserted %d/%d rows in %.2f second(s)", totalData-totalFailed, totalData, end.Seconds())
}

func (r *importer) processEachData(ctx context.Context, rows reflect.Value, fn processFn, wait *sync.WaitGroup, mute *sync.Mutex, transc *sql.Tx, err chan<- *api.HttpError) {
	totalData := rows.Len()

	for i := 0; i < totalData; i++ {
		wait.Add(1)
		data := rows.Index(i).Interface()
		go func(dt interface{}, wg *sync.WaitGroup, mtx *sync.Mutex, trx *sql.Tx, errChan chan<- *api.HttpError) {
			mtx.Lock()
			defer func() {
				mtx.Unlock()
				wg.Done()
			}()
			errChan <- fn(ctx, dt, trx)
		}(data, wait, mute, transc, err)
	}

	wait.Wait()
	close(err)
}
