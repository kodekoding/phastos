package action

import (
	"context"
	"fmt"
	"strings"

	"github.com/kodekoding/phastos/v2/go/database"
	"github.com/kodekoding/phastos/v2/go/helper"
	"github.com/kodekoding/phastos/v2/go/monitoring"
)

type BaseRead struct {
	*baseAction
}

func NewBaseRead(db database.ISQL, tableName string, isSoftDelete ...bool) *BaseRead {
	sofDelete := true
	if isSoftDelete != nil && len(isSoftDelete) > 0 {
		sofDelete = isSoftDelete[0]
	}
	return &BaseRead{&baseAction{db, tableName, sofDelete}}
}

func (b *BaseRead) getBaseQuery(ctx context.Context, opts *database.QueryOpts) string {
	if opts.OptionalTableName != "" {
		originalTableName := b.tableName
		defer func() {
			b.tableName = originalTableName
		}()
		b.tableName = opts.OptionalTableName
	}
	newBaseQuery := opts.BaseQuery
	if newBaseQuery == "" {
		selectedColumnStr := "*"
		if opts.ExcludeColumns != "" {
			selectedCols := helper.GenerateSelectCols(
				ctx,
				opts.Result,
				helper.WithExcludedCols(opts.ExcludeColumns),
				helper.WithIncludedCols(opts.Columns),
			)
			if selectedCols != nil {
				selectedColumnStr = strings.Join(selectedCols, ", ")
			}
		} else if opts.Columns != "" {
			selectedColumnStr = opts.Columns
		}

		newBaseQuery = fmt.Sprintf("SELECT %s FROM %s", selectedColumnStr, b.tableName)
	}
	return newBaseQuery
}

func (b *BaseRead) GetList(ctx context.Context, opts *database.QueryOpts) error {
	txn := monitoring.BeginTrxFromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("PhastosDB-GetList")
		defer segment.End()
	}
	opts.IsList = true
	opts.BaseQuery = b.getBaseQuery(ctx, opts)
	return b.db.Read(ctx, opts)
}

// GetDetail - Query Detail with specific Query and return single data
func (b *BaseRead) GetDetail(ctx context.Context, opts *database.QueryOpts) error {
	txn := monitoring.BeginTrxFromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("PhastosDB-GetDetail")
		defer segment.End()
	}
	opts.BaseQuery = b.getBaseQuery(ctx, opts)
	return b.db.Read(ctx, opts)
}

// GetDetailById - Generate Query "SELECT * FROM <table_name | optional_table_name> WHERE id = ?"
func (b *BaseRead) GetDetailById(ctx context.Context, resultStruct interface{}, id interface{}, optionalTableName ...string) error {
	txn := monitoring.BeginTrxFromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("PhastosDB-GetDetailById")
		segment.AddAttribute("id", id)
		defer segment.End()
	}
	opts := &database.QueryOpts{
		Result: resultStruct,
	}

	if optionalTableName != nil && len(optionalTableName) > 0 {
		opts.OptionalTableName = optionalTableName[0]
	}

	opts.BaseQuery = fmt.Sprintf("%s WHERE id = ?", b.getBaseQuery(ctx, opts))
	return b.db.Read(ctx, opts, id)
}

func (b *BaseRead) Count(ctx context.Context, reqData *database.TableRequest, tableName ...string) (totalData, totalFiltered int, err error) {
	txn := monitoring.BeginTrxFromContext(ctx)
	if txn != nil {
		countSegment := txn.StartSegment("PhastosDB-Count")
		defer countSegment.End()
	}
	selectedTableName := b.tableName
	if tableName != nil && len(tableName) > 0 {
		selectedTableName = tableName[0]
	}
	queryTotal := fmt.Sprintf("SELECT COUNT(1) FROM %s ", selectedTableName)
	opts := &database.QueryOpts{
		BaseQuery:     queryTotal,
		Result:        &totalFiltered,
		IsList:        false,
		SelectRequest: reqData,
	}
	if err = b.db.Read(ctx, opts); err != nil {
		return 0, 0, err
	}

	opts.SelectRequest.Limit = 0
	opts.SelectRequest.Page = 0
	opts.Result = &totalData

	if err = b.db.Read(ctx, opts); err != nil {
		return 0, 0, err
	}

	return
}
