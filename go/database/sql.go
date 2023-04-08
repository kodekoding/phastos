package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strings"
	"sync"
	"time"

	sgw "github.com/ashwanthkumar/slack-go-webhook"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // import postgre driver
	"github.com/pkg/errors"
	_ "gorm.io/driver/mysql" // import mysql driver

	context2 "github.com/kodekoding/phastos/go/context"
	"github.com/kodekoding/phastos/go/env"
	custerr "github.com/kodekoding/phastos/go/error"
)

func newSQL(master, follower *sqlx.DB, timeout int, slowThreshold float64) *SQL {
	sqlTimeOut := 3
	slowQueryThreshold := float64(1)
	if timeout > 0 {
		sqlTimeOut = timeout
	}
	if slowThreshold > 0 {
		slowQueryThreshold = slowThreshold
	}
	return &SQL{
		Master:             master,
		Follower:           follower,
		master:             master,
		follower:           follower,
		timeout:            time.Duration(sqlTimeOut) * time.Second,
		slowQueryThreshold: slowQueryThreshold,
	}
}

func Connect(cfg *SQLs) (*SQL, error) {

	masterDB, err := connectDB(&cfg.Master)
	if err != nil {
		return nil, errors.Wrap(err, "phastos.database.ConnectMaster")
	}

	followerDB, err := connectDB(&cfg.Follower)
	if err != nil {
		return nil, errors.Wrap(err, "phastos.database.ConnectFollower")
	}

	db := newSQL(masterDB, followerDB, cfg.Timeout, cfg.SlowQueryThreshold)
	return db, nil
}

func connectDB(cfg *SQLConfig) (*sqlx.DB, error) {
	generateConnString(cfg)

	db, err := sqlx.Connect(cfg.Engine, cfg.ConnString)
	if err != nil {
		return nil, errors.Wrap(err, "phastos.database.Connect")
	}

	maxLifetime := time.Duration(cfg.MaxConnLifetime) * time.Second
	db.SetConnMaxLifetime(maxLifetime)
	maxIddleTime := time.Duration(cfg.MaxIdleTime) * time.Second
	db.SetConnMaxIdleTime(maxIddleTime)

	db.SetMaxOpenConns(cfg.MaxOpenConn)
	db.SetMaxIdleConns(cfg.MaxIdleConn)
	return db, nil
}

func generateConnString(cfg *SQLConfig) {
	if cfg.ConnString == "" {
		// if ConnString config is empty, then build the connection string manually
		strFormat := ""
		switch cfg.Engine {
		case "mysql":
			if cfg.Port == "" {
				cfg.Port = "3306"
			}
			strFormat = "%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&timeout=60s&readTimeout=60s&writeTimeout=60s"
		case "postgres":
			if cfg.Port == "" {
				cfg.Port = "5432"
			}
			strFormat = "postgres://%s:%s@%s:%s/%s"
		}
		connString := fmt.Sprintf(
			strFormat,
			cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.DBName,
		)
		cfg.ConnString = connString
	}
}

func (this *SQL) GetMaster() *sqlx.DB {
	return this.master
}

func (this *SQL) GetFollower() *sqlx.DB {
	return this.follower
}

func (this *SQL) Read(ctx context.Context, opts *QueryOpts, additionalParams ...interface{}) error {
	if opts.BaseQuery == "" {
		return errors.New("Base Query cannot be empty, please defined the base query")
	}
	if opts.Result == nil {
		return errors.New("Result must be assigned")
	}

	reflectVal := reflect.ValueOf(opts.Result)
	if reflectVal.Kind() != reflect.Ptr {
		return errors.New("Result must be a pointer")
	}

	if opts.Conditions != nil {
		opts.Conditions(ctx)
	}

	var (
		addOnQuery string
		params     = additionalParams
		err        error
		query      = opts.BaseQuery
	)

	if opts.SelectRequest != nil {
		var addOnParams []interface{}
		addOnQuery, addOnParams, err = GenerateAddOnQuery(ctx, opts.SelectRequest)
		if err != nil {
			_, err = sendNilResponse(err, "phastos.database.db.Read.GenerateAddOnQuery", opts.SelectRequest)
			return err
		}

		params = append(params, addOnParams...)
	}

	query += addOnQuery
	query = this.Rebind(query)

	start := time.Now()
	if opts.IsList {
		if err = this.Follower.SelectContext(ctx, opts.Result, query, params...); err != nil {
			_, err = sendNilResponse(err, "phastos.database.Read.SelectContext", query, params)
			return err
		}
	} else {
		if err = this.Follower.GetContext(ctx, opts.Result, query, params...); err != nil {
			_, err = sendNilResponse(err, "phastos.database.Read.GetContext", query, params)
			return err
		}
	}

	this.checkSQLWarning(ctx, query, start, params)

	opts.query = query
	opts.params = params
	return nil
}
func (this *SQL) Write(ctx context.Context, opts *QueryOpts) (*CUDResponse, error) {
	if opts.CUDRequest == nil {
		return nil, errors.New("CUD Request Struct must be assigned")
	}
	var (
		exec sql.Result
		err  error
	)
	// tracing
	//trc, ctx := tracer.StartSQLSpanFromContext(ctx, "CommonRepo-ExecTransaction", query)
	//defer trc.Finish()
	//marshalParam, _ := json.Marshal(data.Values)
	//trc.SetTag("sqlQuery.params", string(marshalParam))
	var (
		addOnQuery string
	)
	data := opts.CUDRequest
	cols := strings.Join(data.Cols, ",")
	var query string
	tableName := data.TableName
	switch data.Action {
	case "insert":
		query = fmt.Sprintf(`INSERT INTO %s (%s) VALUES (?%s)`, tableName, cols, strings.Repeat(",?", len(data.Cols)-1))
	case "bulk_insert":
		query = fmt.Sprintf(`INSERT INTO %s (%s) VALUES %s`, tableName, data.ColsInsert, data.BulkValuesInsert)
	case "upsert":
		colsUpdate := strings.Join(data.Cols, ",")
		query = fmt.Sprintf(`INSERT INTO %s (%s) VALUES (?%s) ON DUPLICATE KEY UPDATE %s`,
			data.TableName,
			data.ColsInsert,
			strings.Repeat(",?", len(data.Cols)-1),
			colsUpdate)
	case "update_by_id":
		query = fmt.Sprintf(`UPDATE %s SET %s WHERE id = ?`, tableName, cols)
	case "delete_by_id":
		query = fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, tableName)
	case "update":
		query = fmt.Sprintf(`UPDATE %s SET %s`, tableName, cols)
	case "delete":
		query = fmt.Sprintf(`DELETE FROM %s`, tableName)
	default:
		return nil, errors.Wrap(errors.New("action exec is not defined"), "phastos.database.sql.Write.CheckAction")
	}

	if opts.SelectRequest != nil {
		var addOnParams []interface{}
		addOnQuery, addOnParams, err = GenerateAddOnQuery(ctx, opts.SelectRequest)
		if err != nil {
			_, err = sendNilResponse(err, "phastos.database.db.Write.GenerateAddOnQuery", opts.SelectRequest)
			return nil, errors.Wrap(err, "")
		}

		data.Values = append(data.Values, addOnParams...)
	}

	query += addOnQuery
	query = this.Rebind(query)

	trx := opts.Trx
	start := time.Now()
	if trx != nil {
		stmt, err := trx.PrepareContext(ctx, query)
		if err != nil {
			_, err = sendNilResponse(err, "phastos.database.Write.PrepareContext", query, data.Values)
			return nil, err
		}
		exec, err = stmt.ExecContext(ctx, data.Values...)
		if err != nil {
			_, err = sendNilResponse(err, "phastos.database.Write.ExecContext", query, data.Values)
			return nil, err
		}
	} else {
		exec, err = this.Master.ExecContext(ctx, query, data.Values...)
		if err != nil {
			_, err = sendNilResponse(err, "phastos.database.Write.WithoutTrx.ExecContext", query, data.Values)
			return nil, err
		}
	}

	this.checkSQLWarning(ctx, query, start, data.Values)

	result := new(CUDResponse)
	lastInsertID, err := exec.LastInsertId()
	if err == nil {
		result.LastInsertID = lastInsertID
	}

	rowsAffected, err := exec.RowsAffected()
	if err == nil {
		result.RowsAffected = rowsAffected
	}
	result.query = query
	result.params = data.Values
	result.Status = true
	return result, nil
}

func generateParamArgsForLike(data string) string {
	return fmt.Sprintf("%%%s%%", data)
}

func (this *SQL) checkSQLWarning(ctx context.Context, query string, start time.Time, params ...interface{}) {
	end := time.Since(start)

	endSecond := end.Seconds()
	if endSecond >= this.slowQueryThreshold {
		defaultWarnMsg := fmt.Sprintf(`
			[WARN] SLOW QUERY DETECTED (%s): %s (%#v)
			Process Query: %.2fs`, env.ServiceEnv(), query, params, end.Seconds(),
		)
		paramsString, _ := json.Marshal(params)
		log.Printf(defaultWarnMsg)
		notif := context2.GetNotif(ctx)
		if notif != nil {
			var attachment interface{}
			color := "#e8dd0e"
			for _, platform := range notif.GetAllPlatform() {
				attachment = nil
				newWarnMsg := defaultWarnMsg
				if platform.Type() == "slack" {
					slackAttachment := &sgw.Attachment{
						Color: &color,
					}
					newWarnMsg = "SLOW QUERY DETECTED"
					slackAttachment.
						AddField(sgw.Field{
							Title: "Query",
							Value: query,
						}).AddField(
						sgw.Field{
							Short: true,
							Title: "Parameter",
							Value: string(paramsString),
						}).AddField(
						sgw.Field{
							Short: true,
							Title: "Process Time",
							Value: fmt.Sprintf("%.2f", endSecond),
						}).AddField(
						sgw.Field{
							Short: true,
							Title: "Environment",
							Value: env.ServiceEnv(),
						})
					attachment = slackAttachment
				}
				if platform.IsActive() {
					_ = platform.Send(ctx, newWarnMsg, attachment)
				}
			}
		}
	}
}

func GenerateAddOnQuery(ctx context.Context, reqData *TableRequest) (string, []interface{}, error) {
	// tracing
	//trc, ctx := tracer.StartSpanFromContext(ctx, "CommonRepo-GenerateAddOnQuery")
	//defer trc.Finish()
	var addOnBuilder strings.Builder
	var addOnParams []interface{}

	checkInitiateWhere(ctx, reqData, &addOnBuilder, &addOnParams)
	err := checkKeyword(ctx, reqData, &addOnBuilder, &addOnParams)
	if err != nil {
		return "", nil, err
	}

	checkCreatedDateParam(ctx, reqData, &addOnBuilder, &addOnParams)

	if addOnBuilder.String() != "" {
		whereString := fmt.Sprintf("WHERE %s", addOnBuilder.String())
		addOnBuilder.Reset()
		addOnBuilder.WriteString(whereString)
	}
	if reqData.GroupBy != "" {
		addOnBuilder.WriteString(fmt.Sprintf(" GROUP BY %s", reqData.GroupBy))
	}
	checkSortParam(ctx, reqData, &addOnBuilder)

	if reqData.Page > 0 && reqData.Limit > 0 {
		offset := (reqData.Page - 1) * reqData.Limit

		addOnBuilder.WriteString(" LIMIT ?,?")
		addOnParams = append(addOnParams, offset, reqData.Limit)
	}
	whereResult := strings.Replace(addOnBuilder.String(), " OR )", ")", -1)
	whereResult = " " + whereResult
	return whereResult, addOnParams, nil
}

func checkKeyword(_ context.Context, reqData *TableRequest, addOnBuilder *strings.Builder, addOnParams *[]interface{}) error {
	// tracing
	//trc, ctx := tracer.StartSpanFromContext(ctx, "CommonRepo-checkKeyword")
	//defer trc.Finish()
	if reqData.Keyword != "" {
		if reqData.SearchColsStr == "" {
			return errors.New("Keyword Cols is required when Keyword Field is filled")
		}
		reqData.SearchCols = strings.Split(reqData.SearchColsStr, ",")
		if reqData.InitiateWhere != nil {
			addOnBuilder.WriteString(" AND ")
		}
		addOnBuilder.WriteString("(")
		mtx := new(sync.Mutex)
		wg := new(sync.WaitGroup)
		for _, col := range reqData.SearchCols {
			wg.Add(1)
			go func(column string, mutex *sync.Mutex, wait *sync.WaitGroup) {
				mutex.Lock()
				addOnBuilder.WriteString(fmt.Sprintf("%s LIKE ? OR ", column))
				*addOnParams = append(*addOnParams, generateParamArgsForLike(reqData.Keyword))
				mutex.Unlock()
				wait.Done()
			}(col, mtx, wg)
		}
		wg.Wait()
		addOnBuilder.WriteString(")")
	}
	return nil
}

func checkSortParam(_ context.Context, reqData *TableRequest, addOnBuilder *strings.Builder) {
	// tracing
	//trc, ctx := tracer.StartSpanFromContext(ctx, "CommonRepo-checkSortParam")
	//defer trc.Finish()
	if reqData.OrderBy != "" {
		addOnBuilder.WriteString(fmt.Sprintf(" ORDER BY %s", reqData.OrderBy))
	}
}

func checkCreatedDateParam(_ context.Context, reqData *TableRequest, addOnBuilder *strings.Builder, addOnParams *[]interface{}) {
	// tracing
	//trc, ctx := tracer.StartSpanFromContext(ctx, "CommonRepo-checkCreatedDateParam")
	//defer trc.Finish()
	if reqData.CreatedStart != "" {
		if addOnBuilder.String() != "" {
			addOnBuilder.WriteString(" AND ")
		}

		col := "created_at"
		if reqData.MainTableAlias != "" {
			col = fmt.Sprintf("%s.%s", reqData.MainTableAlias, col)
		}
		startDate := fmt.Sprintf("%s 00:00:00", reqData.CreatedStart)

		addOnBuilder.WriteString(fmt.Sprintf("DATE_FORMAT(%s, '%%Y-%%m-%%d %%H:%%i:%%s') >= STR_TO_DATE(?, '%%Y-%%m-%%d %%H:%%i:%%s')", col))
		*addOnParams = append(*addOnParams, startDate)
	}

	if reqData.CreatedEnd != "" {
		if addOnBuilder.String() != "" {
			addOnBuilder.WriteString(" AND ")
		}

		col := "created_at"
		if reqData.MainTableAlias != "" {
			col = fmt.Sprintf("%s.%s", reqData.MainTableAlias, col)
		}
		endDate := fmt.Sprintf("%s 23:59:59", reqData.CreatedEnd)

		addOnBuilder.WriteString(fmt.Sprintf("DATE_FORMAT(%s, '%%Y-%%m-%%d %%H:%%i:%%s') <= STR_TO_DATE(?, '%%Y-%%m-%%d %%H:%%i:%%s')", col))
		*addOnParams = append(*addOnParams, endDate)
	}

	if reqData.NotContainsDeletedCol {
		return
	}
	if !reqData.IncludeDeleted {
		col := "deleted_at"
		if reqData.MainTableAlias != "" {
			col = fmt.Sprintf("%s.%s", reqData.MainTableAlias, col)
		}

		if addOnBuilder.String() != "" {
			addOnBuilder.WriteString(" AND ")
		}
		if reqData.IsDeleted != "1" {
			addOnBuilder.WriteString(fmt.Sprintf("(%s IS NULL OR CAST(%s AS CHAR(20)) = '0000-00-00 00:00:00') ", col, col))
		} else {
			addOnBuilder.WriteString(fmt.Sprintf("(%s IS NOT NULL) ", col))
		}
	}
}

func checkInitiateWhere(_ context.Context, reqData *TableRequest, addOnBuilder *strings.Builder, addOnParams *[]interface{}) {
	// tracing
	//trc, ctx := tracer.StartSpanFromContext(ctx, "CommonRepo-checkInitiateWhere")
	//defer trc.Finish()
	if reqData.InitiateWhere != nil {
		for _, condition := range reqData.InitiateWhere {
			addOnBuilder.WriteString(fmt.Sprintf("%s AND ", condition))
		}
		initWhere := addOnBuilder.String()
		initWhere = initWhere[:len(initWhere)-5]
		*addOnParams = append(*addOnParams, reqData.InitiateWhereValues...)

		addOnBuilder.Reset()
		addOnBuilder.WriteString(initWhere)
	}
}

func sendNilResponse(err error, ctxMsg string, params ...interface{}) (interface{}, error) {
	if strings.Contains(err.Error(), "no rows") {
		// return nil for result struct if no rows
		return nil, nil
	}

	customErr := custerr.New(err).SetCode(500)
	for i, paramValue := range params {
		keyParam := fmt.Sprintf("param %d", i+1)
		customErr.AppendData(keyParam, paramValue)
	}
	return nil, errors.Wrap(customErr, ctxMsg)
}
