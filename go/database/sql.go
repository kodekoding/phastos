package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	sgw "github.com/ashwanthkumar/slack-go-webhook"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // import postgre driver
	_ "github.com/newrelic/go-agent/v3/integrations/nrmysql"
	_ "github.com/newrelic/go-agent/v3/integrations/nrpq"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	_ "gorm.io/driver/mysql" // import mysql driver

	context2 "github.com/kodekoding/phastos/v2/go/context"
	"github.com/kodekoding/phastos/v2/go/env"
	custerr "github.com/kodekoding/phastos/v2/go/error"
	"github.com/kodekoding/phastos/v2/go/monitoring"
)

func newSQL(master, follower *sqlx.DB) *SQL {
	slowQueryThreshold := float64(1)
	envSlowQuery, _ := strconv.ParseFloat(os.Getenv("DATABASE_SLOW_QUERY_THRESHOLD"), 32)
	if envSlowQuery > 0 {
		slowQueryThreshold = envSlowQuery
	}

	return &SQL{
		Master:             master,
		Follower:           follower,
		slowQueryThreshold: slowQueryThreshold,
	}
}

func Connect() (*SQL, error) {
	engine := os.Getenv("DATABASE_ENGINE")

	masterDB, err := connectDB(engine, "MASTER")
	if err != nil {
		return nil, errors.Wrap(err, "phastos.database.ConnectMaster")
	}

	followerDB, err := connectDB(engine, "FOLLOWER")
	if err != nil {
		return nil, errors.Wrap(err, "phastos.database.ConnectFollower")
	}

	db := newSQL(masterDB, followerDB)
	db.engine = engine

	if strings.HasPrefix(engine, "nr") {
		// using NR (New Relic) driver
		db.isNR = true
	}
	log.Info().Msg(fmt.Sprintf("Successful connect to DB %s", engine))
	return db, nil
}

func connectDB(engine string, dbType string) (*sqlx.DB, error) {

	connString := os.Getenv(fmt.Sprintf("DATABASE_CONN_STRING_%s", dbType))
	db, err := sqlx.Connect(engine, connString)
	if err != nil {
		return nil, errors.Wrap(err, "phastos.database.Connect")
	}

	cfgMaxConnLifeTime, _ := strconv.Atoi(os.Getenv("DATABASE_CONN_MAX_LIFETIME"))
	maxLifetime := time.Duration(cfgMaxConnLifeTime) * time.Second
	db.SetConnMaxLifetime(maxLifetime)

	cfgMaxIdleTime, _ := strconv.Atoi(os.Getenv("DATABASE_CONN_MAX_IDLE_TIME"))

	maxIdleTime := time.Duration(cfgMaxIdleTime) * time.Second
	db.SetConnMaxIdleTime(maxIdleTime)

	// set maximum open connection to DB
	maxOpenConn, _ := strconv.Atoi(os.Getenv("DATABASE_MAX_OPEN_CONN"))
	if maxOpenConn == 0 {
		maxOpenConn = 10
	}
	db.SetMaxOpenConns(maxOpenConn)

	maxIdleConn, _ := strconv.Atoi(os.Getenv("DATABASE_MAX_IDLE_CONN"))
	if maxIdleConn == 0 {
		maxIdleConn = 4
	}
	db.SetMaxIdleConns(maxIdleConn)
	return db, nil
}

func (this *SQL) GetTransaction() Transactions {
	return NewTransaction(this.Master)
}

func (this *SQL) QueryRow(query string, args ...interface{}) *sql.Row {
	return this.Master.QueryRow(query, args...)
}

func (this *SQL) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return this.Master.QueryRowContext(ctx, query, args...)
}

func (this *SQL) QueryRowx(query string, args ...interface{}) *sqlx.Row {
	return this.Master.QueryRowx(query, args...)
}

func (this *SQL) QueryRowxContext(ctx context.Context, query string, args ...interface{}) *sqlx.Row {
	return this.Master.QueryRowxContext(ctx, query, args...)
}

func (this *SQL) Rebind(sql string) string {
	return this.Master.Rebind(sql)
}

func (this *SQL) Read(ctx context.Context, opts *QueryOpts, additionalParams ...interface{}) error {
	var segment *newrelic.Segment
	if this.isNR {
		txn := monitoring.BeginTrxFromContext(ctx)
		segment = txn.StartSegment("PhastosDB-Read")
		defer segment.End()
		ctx = monitoring.NewContext(ctx, txn)
	}
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
		params = additionalParams
		query  strings.Builder
	)

	query.WriteString(opts.BaseQuery)

	byteReqData, err := json.Marshal(opts)
	segment.AddAttribute("query_option_param", fmt.Sprintf("%s - %#v", string(byteReqData), err))

	if opts.SelectRequest != nil {
		var addOnParams []interface{}
		opts.SelectRequest.engine = this.engine
		addOnQuery, addOnParams, err := GenerateAddOnQuery(ctx, opts.SelectRequest)
		if err != nil {
			_, err = sendNilResponse(err, "phastos.database.db.Read.GenerateAddOnQuery", opts.SelectRequest)
			return err
		}

		query.WriteString(addOnQuery)
		params = append(params, addOnParams...)
	}

	opts.params = params
	start := time.Now()

	byteParam, _ := json.Marshal(params)
	segment.AddAttribute(NewRelicAttributeParams, string(byteParam))

	var finalQuery string
	if opts.Trx != nil {
		var lockingType string
		switch opts.LockingType {
		case LockShare:
			lockingType = " FOR SHARE"
			if active, valid := mySQLEngineGroup[this.engine]; valid && active {
				lockingType = " LOCK IN SHARE MODE"
			}
		case LockUpdate:
			lockingType = " FOR UPDATE"
		default:
			lockingType = ""
		}

		query.WriteString(lockingType)

		finalQuery = opts.Trx.Rebind(query.String())
		opts.query = finalQuery
		if segment != nil {
			segment.AddAttribute(NewRelicAttributeQuery, finalQuery)
		}
		stmt, err := opts.Trx.PreparexContext(ctx, finalQuery)
		if err != nil {
			_, err = sendNilResponse(err, "phastos.database.ReadTrx.PrepareContext", finalQuery, params)
			return err
		}

		if opts.IsList {
			if err = stmt.SelectContext(ctx, opts.Result, params...); err != nil {
				_, err = sendNilResponse(err, "phastos.database.ReadTrx.SelectContext", finalQuery, params)
				return err
			}
		} else {
			if err = stmt.GetContext(ctx, opts.Result, params...); err != nil {
				_, err = sendNilResponse(err, "phastos.database.ReadTrx.GetContext", finalQuery, params)
				return err
			}
		}
	} else {
		finalQuery = this.Follower.Rebind(query.String())
		opts.query = finalQuery

		if segment != nil {
			segment.AddAttribute(NewRelicAttributeQuery, finalQuery)
		}
		if opts.IsList {
			if err = this.Follower.SelectContext(ctx, opts.Result, finalQuery, params...); err != nil {
				_, err = sendNilResponse(err, "phastos.database.Read.SelectContext", finalQuery, params)
				return err
			}
		} else {
			if err = this.Follower.GetContext(ctx, opts.Result, finalQuery, params...); err != nil {
				_, err = sendNilResponse(err, "phastos.database.Read.GetContext", finalQuery, params)
				return err
			}
		}
	}

	this.checkSQLWarning(ctx, finalQuery, start, params)
	return nil
}
func (this *SQL) Write(ctx context.Context, opts *QueryOpts, isSoftDelete ...bool) (*CUDResponse, error) {
	var segment *newrelic.Segment
	if this.isNR {
		txn := monitoring.BeginTrxFromContext(ctx)

		segment = txn.StartSegment("PhastosDB-Write")
		defer segment.End()
		ctx = monitoring.NewContext(ctx, txn)
	}
	if opts.CUDRequest == nil {
		return nil, errors.New("CUD Request Struct must be assigned")
	}
	var (
		exec sql.Result
		err  error
	)

	var (
		addOnQuery string
	)

	softDelete := true
	if isSoftDelete != nil && len(isSoftDelete) > 0 {
		softDelete = isSoftDelete[0]
	}
	data := opts.CUDRequest
	cols := strings.Join(data.Cols, ",")
	var query strings.Builder
	tableName := data.TableName
	switch data.Action {
	case ActionInsert:
		query.WriteString(fmt.Sprintf(`INSERT INTO %s (%s) VALUES (?%s)`, tableName, cols, strings.Repeat(",?", len(data.Cols)-1)))
		if active, valid := postgresEngineGroup[this.engine]; valid && active {
			query.WriteString(" RETURNING *")
		}
	case ActionBulkInsert:
		query.WriteString(fmt.Sprintf(`INSERT INTO %s (%s) VALUES %s`, tableName, data.ColsInsert, data.BulkValues))
	case ActionBulkUpdate:
		query.WriteString(fmt.Sprintf(`UPDATE %s AS main_table JOIN (%s) AS join_table %s`, tableName, data.BulkValues, data.BulkQuery))
	case ActionUpsert:
		colsUpdate := strings.Join(data.Cols, ",")
		query.WriteString(fmt.Sprintf(`INSERT INTO %s (%s) VALUES (?%s) ON DUPLICATE KEY UPDATE %s`,
			data.TableName,
			data.ColsInsert,
			strings.Repeat(",?", len(data.Cols)-1),
			colsUpdate))
	case ActionUpdateById:
		query.WriteString(fmt.Sprintf(`UPDATE %s SET %s WHERE id = ?`, tableName, cols))
	case ActionDeleteById:
		query.WriteString(fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, tableName))
		if softDelete {
			query.Reset()
			query.WriteString(fmt.Sprintf("UPDATE %s SET deleted_at = now() WHERE id = ?", tableName))
		}
	case ActionUpdate:
		query.WriteString(fmt.Sprintf(`UPDATE %s SET %s`, tableName, cols))
	case ActionDelete:
		query.WriteString(fmt.Sprintf(`DELETE FROM %s`, tableName))
		if softDelete {
			query.Reset()
			query.WriteString(fmt.Sprintf("UPDATE %s SET deleted_at = now()", tableName))
		}
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

		query.WriteString(addOnQuery)
		data.Values = append(data.Values, addOnParams...)
	}

	finalQuery := this.Master.Rebind(query.String())
	// reset and replace the final query with rebind-ed query
	query.Reset()
	query.WriteString(finalQuery)
	result := new(CUDResponse)
	result.query = query.String()
	result.params = data.Values
	trx := opts.Trx
	start := time.Now()
	lastInsertID := int64(0)
	rowsAffected := int64(0)

	if segment != nil {
		segment.AddAttribute(NewRelicAttributeQuery, query.String())
		byteParam, _ := json.Marshal(data.Values)
		segment.AddAttribute(NewRelicAttributeParams, string(byteParam))
	}
	if trx != nil {
		if active, valid := postgresEngineGroup[this.engine]; valid && active && data.Action == ActionUpdate {
			query.WriteString(" RETURNING *")
		}
		stmt, err := trx.PreparexContext(ctx, query.String())
		if err != nil {
			_, err = sendNilResponse(err, "phastos.database.Write.PrepareContext", query.String(), data.Values)
			return result, err
		}

		if active, valid := postgresEngineGroup[this.engine]; valid && active {

			if err = stmt.QueryRowxContext(ctx, data.Values...).StructScan(opts.Result); err != nil {
				_, err = sendNilResponse(err, "phastos.database.Write.QueryRowContext", query.String(), data.Values)
				if err == nil {
					result.RowsAffected = 1
					result.Status = true
				}
				return result, err
			}
		} else {
			exec, err = stmt.ExecContext(ctx, data.Values...)
			if err != nil {
				_, err = sendNilResponse(err, "phastos.database.Write.ExecContext", query.String(), data.Values)
				return result, err
			}
		}
	} else {
		if active, valid := postgresEngineGroup[this.engine]; valid && active {
			if data.Action == ActionUpdate {
				query.WriteString(" RETURNING *")
			}
			if err = this.Master.QueryRowxContext(ctx, query.String(), data.Values...).StructScan(opts.Result); err != nil {
				_, err = sendNilResponse(err, "phastos.database.Write.QueryRowContext", query.String(), data.Values)
				if err == nil {
					result.RowsAffected = 1
					result.Status = true
				}
				return result, err
			}
		} else {
			exec, err = this.Master.ExecContext(ctx, query.String(), data.Values...)
			if err != nil {
				_, err = sendNilResponse(err, "phastos.database.Write.WithoutTrx.ExecContext", query.String(), data.Values)
				return result, err
			}
		}
	}
	rowsAffected++
	result.LastInsertID = lastInsertID
	result.RowsAffected = rowsAffected

	this.checkSQLWarning(ctx, query.String(), start, data.Values)

	if active, valid := mySQLEngineGroup[this.engine]; valid && active {
		lastInsertID, err = exec.LastInsertId()
		if err == nil {
			result.LastInsertID = lastInsertID
		}

		rowsAffected, err = exec.RowsAffected()
		if err == nil {
			result.RowsAffected = rowsAffected
		}
	}

	result.Status = true
	return result, nil
}

func generateParamArgsForLike(data string) string {
	return fmt.Sprintf("%%%s%%", data)
}

func (this *SQL) checkSQLWarning(ctx context.Context, query string, start time.Time, params ...interface{}) {
	enabledSQLWarningEnv := os.Getenv("DATABASE_SLOW_QUERY_WARNING")
	enabledSQLWarning, _ := strconv.ParseBool(enabledSQLWarningEnv)
	if enabledSQLWarning {
		end := time.Since(start)

		endSecond := end.Seconds()
		if endSecond >= this.slowQueryThreshold {
			defaultWarnMsg := fmt.Sprintf(`
			[WARN] SLOW QUERY DETECTED (%s): %s (%#v)
			Process Query: %.2fs`, env.ServiceEnv(), query, params, end.Seconds())
			paramsString, _ := json.Marshal(params)
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
}

func GenerateAddOnQuery(ctx context.Context, reqData *TableRequest) (string, []interface{}, error) {
	txn := monitoring.BeginTrxFromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("PhastosDB-GeneratingAddOnQuery")
		byteReqData, _ := json.Marshal(reqData)
		segment.AddAttribute("requestData", string(byteReqData))
		segment.AddAttribute("engine", reqData.engine)
		defer segment.End()
	}
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

		if _, isPostgres := postgresEngineGroup[reqData.engine]; isPostgres {
			addOnBuilder.WriteString(" LIMIT ? OFFSET ?")
			addOnParams = append(addOnParams, reqData.Limit, offset)
		} else if _, isMySQL := mySQLEngineGroup[reqData.engine]; isMySQL {
			addOnBuilder.WriteString(" LIMIT ?,?")
			addOnParams = append(addOnParams, offset, reqData.Limit)
		} else {
			log.Warn().Str("engine", reqData.engine).Any("request_data", reqData).Msg("engine not defined !! please check your code again")
		}
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
		if reqData.CustomDateColFilter != "" {
			col = reqData.CustomDateColFilter
		}
		if reqData.MainTableAlias != "" {
			col = fmt.Sprintf("%s.%s", reqData.MainTableAlias, col)
		}
		startDate := fmt.Sprintf("%s 00:00:00", reqData.CreatedStart)

		if _, isMySQL := mySQLEngineGroup[reqData.engine]; isMySQL {
			addOnBuilder.WriteString(fmt.Sprintf("DATE_FORMAT(%s, '%%Y-%%m-%%d %%H:%%i:%%s') >= STR_TO_DATE(?, '%%Y-%%m-%%d %%H:%%i:%%s')", col))
		} else {
			addOnBuilder.WriteString(fmt.Sprintf("%s >= ?", col))
		}
		*addOnParams = append(*addOnParams, startDate)
	}

	if reqData.CreatedEnd != "" {
		if addOnBuilder.String() != "" {
			addOnBuilder.WriteString(" AND ")
		}

		col := "created_at"
		if reqData.CustomDateColFilter != "" {
			col = reqData.CustomDateColFilter
		}

		if reqData.MainTableAlias != "" {
			col = fmt.Sprintf("%s.%s", reqData.MainTableAlias, col)
		}
		endDate := fmt.Sprintf("%s 23:59:59", reqData.CreatedEnd)

		if _, isMySQL := mySQLEngineGroup[reqData.engine]; isMySQL {
			addOnBuilder.WriteString(fmt.Sprintf("DATE_FORMAT(%s, '%%Y-%%m-%%d %%H:%%i:%%s') <= STR_TO_DATE(?, '%%Y-%%m-%%d %%H:%%i:%%s')", col))
		} else {
			addOnBuilder.WriteString(fmt.Sprintf("%s <= ?", col))
		}
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
