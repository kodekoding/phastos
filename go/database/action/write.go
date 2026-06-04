package action

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"

	"github.com/kodekoding/phastos/v2/go/common"
	"github.com/kodekoding/phastos/v2/go/database"
	"github.com/kodekoding/phastos/v2/go/helper"
	"github.com/kodekoding/phastos/v2/go/monitoring"
)

type BaseWrites interface {
	common.WriteRepo
}

type BaseWrite struct {
	*baseAction
}

func NewBaseWrite(db database.ISQL, tableName string, isSoftDelete ...bool) *BaseWrite {
	sofDelete := true
	if len(isSoftDelete) > 0 {
		sofDelete = isSoftDelete[0]
	}
	return &BaseWrite{&baseAction{db, tableName, sofDelete}}
}

func (b *BaseWrite) Insert(ctx context.Context, data interface{}, optTrx ...*sqlx.Tx) (*database.CUDResponse, error) {
	txn := monitoring.BeginTrxFromContext(ctx)
	if txn != nil {
		insertSegment := txn.StartSegment("PhastosDB-Insert")
		defer insertSegment.End()
	}
	var trx *sqlx.Tx
	if len(optTrx) > 0 {
		trx = optTrx[0]
	}
	return b.cudProcess(ctx, database.ActionInsert, data, nil, trx)
}

func (b *BaseWrite) BulkInsert(ctx context.Context, data interface{}, optTrx ...*sqlx.Tx) (*database.CUDResponse, error) {
	txn := monitoring.BeginTrxFromContext(ctx)
	if txn != nil {
		bulkInsertSegment := txn.StartSegment("PhastosDB-BulkInsert")
		defer bulkInsertSegment.End()
	}
	var trx *sqlx.Tx
	if len(optTrx) > 0 {
		trx = optTrx[0]
	}
	return b.cudProcess(ctx, database.ActionBulkInsert, data, nil, trx)
}

func (b *BaseWrite) BulkUpdate(ctx context.Context, data interface{}, condition map[string][]interface{}, optTrx ...*sqlx.Tx) (*database.CUDResponse, error) {
	cudRequestData, err := helper.ConstructColNameAndValueBulk(ctx, data, condition)
	if err != nil {
		return nil, err
	}

	action := database.ActionBulkUpdate
	cudRequestData.Action = action
	cudRequestData.TableName = b.tableName

	qOpts := &database.QueryOpts{
		CUDRequest: cudRequestData,
	}
	if len(optTrx) > 0 {
		trx := optTrx[0]
		qOpts.Trx = trx
	}

	result, err := b.db.Write(ctx, qOpts)
	if err != nil {
		return result, errors.Wrap(err, "phastos.database.action."+action+".Write")
	}

	return result, nil
}

func (b *BaseWrite) Update(ctx context.Context, data interface{}, condition map[string]interface{}, optTrx ...*sqlx.Tx) (*database.CUDResponse, error) {
	txn := monitoring.BeginTrxFromContext(ctx)
	if txn != nil {
		updateSegment := txn.StartSegment("PhastosDB-Update")
		defer updateSegment.End()
	}
	var trx *sqlx.Tx
	if len(optTrx) > 0 {
		trx = optTrx[0]
	}
	return b.cudProcess(ctx, database.ActionUpdate, data, condition, trx)
}

func (b *BaseWrite) UpdateById(ctx context.Context, data interface{}, id interface{}, optTrx ...*sqlx.Tx) (*database.CUDResponse, error) {
	txn := monitoring.BeginTrxFromContext(ctx)
	if txn != nil {
		updateByIdSegment := txn.StartSegment("PhastosDB-UpdateByID")
		defer updateByIdSegment.End()
	}

	condition := map[string]interface{}{
		"id = ?": id,
	}
	var trx *sqlx.Tx
	if len(optTrx) > 0 {
		trx = optTrx[0]
	}
	return b.cudProcess(ctx, database.ActionUpdateById, data, condition, trx)
}

func (b *BaseWrite) Delete(ctx context.Context, condition map[string]interface{}, optTrx ...*sqlx.Tx) (*database.CUDResponse, error) {
	txn := monitoring.BeginTrxFromContext(ctx)
	if txn != nil {
		deleteSegment := txn.StartSegment("PhastosDB-Delete")
		defer deleteSegment.End()
	}
	// soft delete, just update the deleted_at to not null
	data := &database.CUDConstructData{
		Cols:      []string{"deleted_at = now()"},
		Action:    database.ActionDelete,
		TableName: b.tableName,
	}
	qOpts := &database.QueryOpts{
		CUDRequest: data,
	}
	if len(optTrx) > 0 {
		trx := optTrx[0]
		qOpts.Trx = trx
	}

	tableRequest := database.GetTableRequest()
	defer database.PutTableRequest(tableRequest)
	tableRequest.IncludeDeleted = true
	for cond, value := range condition {
		tableRequest.SetWhereCondition(cond, value)
	}
	qOpts.SelectRequest = tableRequest
	return b.db.Write(ctx, qOpts, b.isSoftDelete)
}

func (b *BaseWrite) DeleteById(ctx context.Context, id interface{}, optTrx ...*sqlx.Tx) (*database.CUDResponse, error) {
	txn := monitoring.BeginTrxFromContext(ctx)
	if txn != nil {
		deleteByIdSegment := txn.StartSegment("PhastosDB-DeleteByID")
		defer deleteByIdSegment.End()
	}

	// O5: Fast path — cache query + stmt per (tableName, isSoftDelete).
	// No CUDConstructData, no QueryOpts, no Write() builder.
	var trx *sqlx.Tx
	if len(optTrx) > 0 {
		trx = optTrx[0]
	}

	if trx == nil {
		sqlObj, ok := b.db.(*database.SQL)
		if ok {
			query := getDeleteByIdQuery(b.db, b.tableName, b.isSoftDelete)
			stmt, stmtErr := sqlObj.GetWriteStmt(ctx, query)
			if stmtErr != nil {
				return nil, errors.Wrap(stmtErr, "phastos.database.action.DeleteById.PrepareStmt")
			}
			start := time.Now()
			exec, execErr := stmt.ExecContext(ctx, id)
			if execErr != nil {
				database.EvictWriteStmt(query)
				return nil, errors.Wrap(execErr, "phastos.database.action.DeleteById.ExecContext")
			}
			result := database.GetCUDResponse()
			if ra, raErr := exec.RowsAffected(); raErr == nil {
				result.RowsAffected = ra
			} else {
				result.RowsAffected = 1
			}
			if active, valid := database.MySQLEngineGroupActive(sqlObj.Engine()); valid && active {
				if lastID, err := exec.LastInsertId(); err == nil {
					result.LastInsertID = lastID
				}
			}
			result.Status = true
			sqlObj.CheckSQLWarning(ctx, query, start, id)
			return result, nil
		}
	}

	// Trx path or non-SQL db: go through Write()
	data := &database.CUDConstructData{
		Action:    database.ActionDeleteById,
		TableName: b.tableName,
		Values:    []interface{}{id},
	}
	qOpts := &database.QueryOpts{
		CUDRequest: data,
	}
	if trx != nil {
		qOpts.Trx = trx
	}
	return b.db.Write(ctx, qOpts, b.isSoftDelete)
}

// deleteByIdCacheKey identifies a cached DeleteById query.
type deleteByIdCacheKey struct {
	TableName    string
	IsSoftDelete bool
}

// deleteByIdCache stores pre-computed, Rebind-ed DeleteById queries.
var deleteByIdCache sync.Map

// getDeleteByIdQuery returns the cached rebound DeleteById query for (tableName, isSoftDelete).
func getDeleteByIdQuery(db database.ISQL, tableName string, isSoftDelete bool) string {
	key := deleteByIdCacheKey{TableName: tableName, IsSoftDelete: isSoftDelete}
	if cached, ok := deleteByIdCache.Load(key); ok {
		return cached.(string) //nolint:errcheck
	}
	var query string
	if isSoftDelete {
		query = fmt.Sprintf("UPDATE %s SET deleted_at = now() WHERE id = ?", tableName)
	} else {
		query = fmt.Sprintf("DELETE FROM %s WHERE id = ?", tableName)
	}
	rebound := db.CachedRebind(query)
	actual, _ := deleteByIdCache.LoadOrStore(key, rebound)
	return actual.(string) //nolint:errcheck
}

func (b *BaseWrite) Upsert(ctx context.Context, data interface{}, condition map[string]interface{}, opts ...interface{}) (*database.CUDResponse, error) {
	txn := monitoring.BeginTrxFromContext(ctx)
	if txn != nil {
		upsertSegment := txn.StartSegment("PhastosDB-Upsert")
		defer upsertSegment.End()
	}
	var existingId int64
	tableRequest := database.GetTableRequest()
	defer database.PutTableRequest(tableRequest)
	pointerCondition := &condition
	for cond, val := range *pointerCondition {
		if val != nil {
			if !strings.Contains(cond, "?") {
				cond = fmt.Sprintf("%s = ?", cond)
			}

			tableRequest.SetWhereCondition(cond, val)
		} else {
			tableRequest.SetWhereCondition(cond)
		}
	}

	var trx *sqlx.Tx
	var includeDeleted bool
	if len(opts) > 0 {
		trx = opts[0].(*sqlx.Tx) //nolint:errcheck
		if len(opts) > 1 {
			includeDeleted = opts[1].(bool) //nolint:errcheck
		}
	}

	tableRequest.IncludeDeleted = includeDeleted
	if err := b.db.Read(ctx, &database.QueryOpts{
		BaseQuery:     fmt.Sprintf("SELECT count(1) FROM %s", b.tableName),
		SelectRequest: tableRequest,
		Result:        &existingId,
	}); err != nil {
		return nil, errors.Wrap(err, "phastos.go.database.action.write.Upsert.GetData")
	}

	if existingId > 0 {
		return b.cudProcess(ctx, database.ActionUpdate, data, *pointerCondition, trx, existingId)
	}
	return b.cudProcess(ctx, database.ActionInsert, data, nil, trx)
}

func (b *BaseWrite) cudProcess(ctx context.Context, action string, data interface{}, condition map[string]interface{}, opts ...interface{}) (*database.CUDResponse, error) {
	var cudRequestData *database.CUDConstructData
	var err error
	switch action {
	case database.ActionInsert:
		cols, vals := helper.ConstructColNameAndValue(ctx, data)
		if cols == nil && vals == nil {
			return nil, errors.New("second parameter should be a pointer of struct")
		}
		cudRequestData = &database.CUDConstructData{
			Cols:   cols,
			Values: vals,
		}
	case database.ActionBulkInsert:
		cudRequestData, err = helper.ConstructColNameAndValueBulk(ctx, data)
		if err != nil {
			return nil, err
		}
	case database.ActionUpdate:
		cudRequestData = helper.ConstructColNameAndValueForUpdate(ctx, data)
	case database.ActionUpdateById:
		cudRequestData = helper.ConstructColNameAndValueForUpdate(ctx, data, condition["id = ?"])
		condition = nil
	case database.ActionUpsert:
		cudRequestData = helper.ConstructColNameAndValueForUpdate(ctx, data)
		cudRequestData.Values = append(cudRequestData.Values, cudRequestData.Values...)
	case database.ActionDelete:
		cudRequestData = data.(*database.CUDConstructData) //nolint:errcheck
	default:
		return nil, errors.New("undefined action")
	}

	cudRequestData.Action = action
	cudRequestData.TableName = b.tableName

	qOpts := &database.QueryOpts{
		CUDRequest: cudRequestData,
		Result:     data,
	}

	totalOpts := len(opts)
	if opts != nil && totalOpts > 0 {
		qOpts.Trx = opts[0].(*sqlx.Tx) //nolint:errcheck
		if totalOpts > 1 {
			qOpts.UpsertInsertId = opts[1].(int64) //nolint:errcheck
		}
	}

	if condition != nil {
		tableRequest := database.GetTableRequest()
		defer database.PutTableRequest(tableRequest)
		tableRequest.IncludeDeleted = true
		for cond, value := range condition {
			tableRequest.SetWhereCondition(cond, value)
		}
		qOpts.SelectRequest = tableRequest
	}
	result, err := b.db.Write(ctx, qOpts)
	if err != nil {
		return result, errors.Wrap(err, "phastos.database.action."+action+".ExecTransation")
	}

	return result, nil
}
