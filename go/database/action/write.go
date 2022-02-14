package action

import (
	"context"
	"database/sql"

	"github.com/kodekoding/phastos/go/common"
	"github.com/kodekoding/phastos/go/database"
	"github.com/kodekoding/phastos/go/helper"

	"github.com/pkg/errors"
)

type BaseWrites interface {
	common.WriteRepo
	Upsert(ctx context.Context, data interface{}, trx ...*sql.Tx) (*database.CUDResponse, error)
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
	return b.cudProcess(ctx, "insert", data, 0, trx...)
}

func (b *BaseWrite) Update(ctx context.Context, data interface{}, id int, trx ...*sql.Tx) (*database.CUDResponse, error) {
	return b.cudProcess(ctx, "update", data, id, trx...)
}

func (b *BaseWrite) Delete(ctx context.Context, id int, trx ...*sql.Tx) (*database.CUDResponse, error) {
	// soft delete, just update the deleted_at to not null
	data := &database.CUDConstructData{
		Cols:   []string{"deleted_at = now()"},
		Values: []interface{}{id},
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
		return b.Update(ctx, data, id, trx...)
	}
	return b.db.Write(ctx, qOpts)
}

func (b *BaseWrite) Upsert(ctx context.Context, data interface{}, trx ...*sql.Tx) (*database.CUDResponse, error) {
	return b.cudProcess(ctx, "upsert", data, 0, trx...)
}

func (b *BaseWrite) cudProcess(ctx context.Context, action string, data interface{}, id int, trx ...*sql.Tx) (*database.CUDResponse, error) {
	var cudRequestData *database.CUDConstructData
	switch action {
	case "insert":
		cols, vals := helper.ConstructColNameAndValue(ctx, data)
		if cols == nil && vals == nil {
			return nil, errors.New("second parameter should be a pointer of struct")
		}
		cudRequestData = &database.CUDConstructData{
			Cols:      cols,
			Values:    vals,
			Action:    action,
			TableName: b.tableName,
		}
	case "update":
		cudRequestData = helper.ConstructColNameAndValueForUpdate(ctx, data, id)
	case "upsert":
		cudRequestData = helper.ConstructColNameAndValueForUpdate(ctx, data)
	case "delete":
		cudRequestData = data.(*database.CUDConstructData)
	default:
		return nil, errors.New("undefined action")
	}

	cudRequestData.Action = action

	qOpts := &database.QueryOpts{
		CUDRequest: cudRequestData,
	}
	if trx != nil && len(trx) > 0 {
		qOpts.Trx = trx[0]
	}
	result, err := b.db.Write(ctx, qOpts)
	if err != nil {
		return nil, errors.Wrap(err, "phastos.database.action."+action+".ExecTransation")
	}

	return result, nil
}
