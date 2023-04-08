package action

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kodekoding/phastos/go/common"
	"github.com/kodekoding/phastos/go/database"
	"github.com/kodekoding/phastos/go/helper"

	"github.com/pkg/errors"
)

type BaseWrites interface {
	common.WriteRepo
}

type BaseWrite struct {
	*baseAction
}

func NewBaseWrite(db *database.SQL, tableName string, isSoftDelete ...bool) *BaseWrite {
	sofDelete := true
	if isSoftDelete != nil && len(isSoftDelete) > 0 {
		sofDelete = isSoftDelete[0]
	}
	return &BaseWrite{&baseAction{db, tableName, sofDelete}}
}

func (b *BaseWrite) Insert(ctx context.Context, data interface{}, trx ...*sql.Tx) (*database.CUDResponse, error) {
	return b.cudProcess(ctx, "insert", data, nil, trx...)
}

func (b *BaseWrite) BulkInsert(ctx context.Context, data interface{}, trx ...*sql.Tx) (*database.CUDResponse, error) {
	return b.cudProcess(ctx, "bulk_insert", data, nil, trx...)
}

func (b *BaseWrite) Update(ctx context.Context, data interface{}, condition map[string]interface{}, trx ...*sql.Tx) (*database.CUDResponse, error) {
	return b.cudProcess(ctx, "update", data, condition, trx...)
}

func (b *BaseWrite) UpdateById(ctx context.Context, data interface{}, id int, trx ...*sql.Tx) (*database.CUDResponse, error) {
	condition := map[string]interface{}{
		"id = ?": id,
	}
	return b.cudProcess(ctx, "update_by_id", data, condition, trx...)
}

func (b *BaseWrite) Delete(ctx context.Context, condition map[string]interface{}, trx ...*sql.Tx) (*database.CUDResponse, error) {
	// soft delete, just update the deleted_at to not null
	data := &database.CUDConstructData{
		Cols: []string{"deleted_at = now()"},
	}
	if !b.isSoftDelete {
		data.Action = "delete"
		data.TableName = b.tableName
	}
	qOpts := &database.QueryOpts{
		CUDRequest: data,
	}
	if trx != nil && len(trx) > 0 {
		qOpts.Trx = trx[0]
	}

	if b.isSoftDelete {
		return b.Update(ctx, data, condition, trx...)
	}

	tableRequest := new(database.TableRequest)
	tableRequest.IncludeDeleted = true
	for cond, value := range condition {
		tableRequest.SetWhereCondition(cond, value)
	}
	qOpts.SelectRequest = tableRequest
	return b.db.Write(ctx, qOpts)
}

func (b *BaseWrite) DeleteById(ctx context.Context, id int, trx ...*sql.Tx) (*database.CUDResponse, error) {
	// soft delete, just update the deleted_at to not null
	data := &database.CUDConstructData{
		Cols:   []string{"deleted_at = now()"},
		Values: []interface{}{id},
	}
	if !b.isSoftDelete {
		data.Action = "delete_by_id"
		data.TableName = b.tableName
	}
	qOpts := &database.QueryOpts{
		CUDRequest: data,
	}
	if trx != nil && len(trx) > 0 {
		qOpts.Trx = trx[0]
	}

	if b.isSoftDelete {
		return b.UpdateById(ctx, data, id, trx...)
	}
	return b.db.Write(ctx, qOpts)
}

func (b *BaseWrite) Upsert(ctx context.Context, data interface{}, condition map[string]interface{}, trx ...*sql.Tx) (*database.CUDResponse, error) {
	var totalData int
	tableRequest := new(database.TableRequest)
	for cond, val := range condition {
		tableRequest.SetWhereCondition(cond, val)
	}
	if err := b.db.Read(ctx, &database.QueryOpts{
		BaseQuery:     fmt.Sprintf("SELECT COUNT(1) FROM %s", b.tableName),
		SelectRequest: tableRequest,
		Result:        &totalData,
	}); err != nil {
		return nil, errors.Wrap(err, "phastos.go.database.action.write.Upsert.GetData")
	}

	if totalData > 0 {
		return b.cudProcess(ctx, "update", data, condition, trx...)
	}
	return b.cudProcess(ctx, "insert", data, nil, trx...)
}

func (b *BaseWrite) cudProcess(ctx context.Context, action string, data interface{}, condition map[string]interface{}, trx ...*sql.Tx) (*database.CUDResponse, error) {
	var cudRequestData *database.CUDConstructData
	var err error
	switch action {
	case "insert":
		cols, vals := helper.ConstructColNameAndValue(ctx, data)
		if cols == nil && vals == nil {
			return nil, errors.New("second parameter should be a pointer of struct")
		}
		cudRequestData = &database.CUDConstructData{
			Cols:   cols,
			Values: vals,
		}
	case "bulk_insert":
		cudRequestData, err = helper.ConstructColNameAndValueBulk(ctx, data)
		if err != nil {
			return nil, err
		}
	case "update":
		cudRequestData = helper.ConstructColNameAndValueForUpdate(ctx, data)
	case "update_by_id":
		cudRequestData = helper.ConstructColNameAndValueForUpdate(ctx, data, condition["id = ?"])
		condition = nil
	case "upsert":
		cudRequestData = helper.ConstructColNameAndValueForUpdate(ctx, data)
		cudRequestData.Values = append(cudRequestData.Values, cudRequestData.Values...)
	case "delete":
		cudRequestData = data.(*database.CUDConstructData)
	default:
		return nil, errors.New("undefined action")
	}

	cudRequestData.Action = action
	cudRequestData.TableName = b.tableName

	qOpts := &database.QueryOpts{
		CUDRequest: cudRequestData,
	}
	if trx != nil && len(trx) > 0 {
		qOpts.Trx = trx[0]
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
