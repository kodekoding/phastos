package action

import (
	"context"
	"fmt"
	"reflect"
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

// updateByIdCacheKey identifies a unique UpdateById query template.
// For a given struct type + table name, the query is always the same.
type updateByIdCacheKey struct {
	Type      reflect.Type
	TableName string
}

// updateByIdCacheEntry holds the pre-computed, Rebind-ed query string
// and the update template for fast value extraction.
// R8: Template is now just a reference to the shared updateTemplateCache
// from struct_cache.go — no duplicate caching per reflect.Type.
type updateByIdCacheEntry struct {
	Query      string                     // full Rebind-ed query
	SetCols    string                     // comma-joined SET cols
	Template   *helper.UpdateTemplateInfo // reference to shared cache entry
	StructType reflect.Type
}

var updateByIdCache sync.Map

func getUpdateByIdCache(db database.ISQL, t reflect.Type, tableName string) *updateByIdCacheEntry {
	key := updateByIdCacheKey{Type: t, TableName: tableName}
	if cached, ok := updateByIdCache.Load(key); ok {
		return cached.(*updateByIdCacheEntry) //nolint:errcheck
	}

	// R8: GetUpdateTemplate returns the shared cache entry from struct_cache.go
	tmpl := helper.GetUpdateTemplate(t)
	setCols := strings.Join(tmpl.Cols, ",")

	// Build the query template
	var queryBuilder strings.Builder
	queryBuilder.WriteString("UPDATE ")
	queryBuilder.WriteString(tableName)
	queryBuilder.WriteString(" SET ")
	queryBuilder.WriteString(setCols)
	queryBuilder.WriteString(" WHERE id = ?")

	// Rebind once — cached across all calls
	reboundQuery := db.CachedRebind(queryBuilder.String())

	entry := &updateByIdCacheEntry{
		Query:      reboundQuery,
		SetCols:    setCols,
		Template:   tmpl,
		StructType: t,
	}
	actual, _ := updateByIdCache.LoadOrStore(key, entry)
	return actual.(*updateByIdCacheEntry) //nolint:errcheck
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

	// Fast path: try cached template for struct types
	reflectVal := reflect.ValueOf(data)
	if reflectVal.Kind() == reflect.Ptr {
		reflectVal = reflectVal.Elem()
	}
	if reflectVal.Kind() == reflect.Struct {
		entry := getUpdateByIdCache(b.db, reflectVal.Type(), b.tableName)
		// O4: Use fixed template — invariant query string so the prepared
		// stmt is always reused. This is critical for PG performance.
		info := helper.ExtractFixedUpdateValues(entry.Template, reflectVal, id)

		// No-trx fast path: use cached stmt directly, skip Write() entirely
		var trx *sqlx.Tx
		if len(optTrx) > 0 {
			trx = optTrx[0]
		}

		if trx == nil {
			// Direct execution with cached query + cached stmt
			sqlObj, ok := b.db.(*database.SQL)
			if ok {
				result := database.GetCUDResponse()
				start := time.Now()

				// PG: updated_at is in the template, use ExecContext via cached stmt.
				// No special handling needed — the stmt is invariant.
				_ = sqlObj.IsPostgres() && entry.Template.HaveUpdatedAt
				stmt, stmtErr := sqlObj.GetWriteStmt(ctx, entry.Query)
				if stmtErr != nil {
					return nil, errors.Wrap(stmtErr, "phastos.database.action.UpdateById.PrepareStmt")
				}
				exec, execErr := stmt.ExecContext(ctx, info.Values...)
				if execErr != nil {
					database.EvictWriteStmt(entry.Query)
					return nil, errors.Wrap(execErr, "phastos.database.action.UpdateById.ExecContext")
				}

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
				sqlObj.CheckSQLWarning(ctx, entry.Query, start, info.Values)
				return result, nil
			}
		}

		// Trx path: still go through Write() but with fixed template
		cudRequestData := database.GetCUDConstructData()
		cudRequestData.Cols = info.Cols
		cudRequestData.ColsInsert = entry.SetCols
		cudRequestData.Values = info.Values
		cudRequestData.Action = database.ActionUpdateById
		cudRequestData.TableName = b.tableName

		qOpts := &database.QueryOpts{
			CUDRequest: cudRequestData,
			Result:     data,
			Trx:        trx,
		}

		result, err := b.db.Write(ctx, qOpts)
		if err != nil {
			return result, errors.Wrap(err, "phastos.database.action.UpdateById.Write")
		}
		return result, nil
	}

	// Slow path: original implementation for non-struct inputs
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
