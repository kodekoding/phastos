package action

import (
	"context"
	"fmt"
	"strings"

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
	if isSoftDelete != nil && len(isSoftDelete) > 0 {
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
	if optTrx != nil && len(optTrx) > 0 {
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
	if optTrx != nil && len(optTrx) > 0 {
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
	if optTrx != nil && len(optTrx) > 0 {
		var trx *sqlx.Tx
		trx = optTrx[0]
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
	if optTrx != nil && len(optTrx) > 0 {
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
	if optTrx != nil && len(optTrx) > 0 {
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
	if optTrx != nil && len(optTrx) > 0 {
		var trx *sqlx.Tx
		trx = optTrx[0]
		qOpts.Trx = trx
	}

	tableRequest := new(database.TableRequest)
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
	// soft delete, just update the deleted_at to not null
	data := &database.CUDConstructData{
		Action:    database.ActionDeleteById,
		TableName: b.tableName,
		Values:    []interface{}{id},
	}
	qOpts := &database.QueryOpts{
		CUDRequest: data,
	}
	if optTrx != nil && len(optTrx) > 0 {
		var trx *sqlx.Tx
		trx = optTrx[0]
		qOpts.Trx = trx
	}
	return b.db.Write(ctx, qOpts, b.isSoftDelete)
}

func (b *BaseWrite) Upsert(ctx context.Context, data interface{}, condition map[string]interface{}, opts ...interface{}) (*database.CUDResponse, error) {
	txn := monitoring.BeginTrxFromContext(ctx)
	if txn != nil {
		upsertSegment := txn.StartSegment("PhastosDB-Upsert")
		defer upsertSegment.End()
	}
	var existingId int64
	tableRequest := new(database.TableRequest)
	pointerCondition := &condition
	for cond, val := range *pointerCondition {
		if !strings.Contains(cond, "?") {
			cond = fmt.Sprintf("%s = ?", cond)
		}
		tableRequest.SetWhereCondition(cond, val)
	}

	var trx *sqlx.Tx
	var includeDeleted bool
	if opts != nil && len(opts) > 0 {
		trx = opts[0].(*sqlx.Tx)
		if len(opts) > 1 {
			includeDeleted = opts[1].(bool)
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
		cudRequestData = data.(*database.CUDConstructData)
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
		qOpts.Trx = opts[0].(*sqlx.Tx)
		if totalOpts > 1 {
			qOpts.UpsertInsertId = opts[1].(int64)
		}
	}

	if condition != nil {
		tableRequest := new(database.TableRequest)
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
