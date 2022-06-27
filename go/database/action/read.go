package action

import (
	"context"
	"fmt"

	"github.com/kodekoding/phastos/go/database"
)

type BaseRead struct {
	*baseAction
}

func NewBaseRead(db *database.SQL, tableName string, isSoftDelete ...bool) *BaseRead {
	sofDelete := true
	if isSoftDelete != nil && len(isSoftDelete) > 0 {
		sofDelete = isSoftDelete[0]
	}
	return &BaseRead{&baseAction{db, tableName, sofDelete}}
}

func (b *BaseRead) getBaseQuery(optionalTableName, baseQuery string) string {
	if optionalTableName != "" {
		originalTableName := b.tableName
		defer func() {
			b.tableName = originalTableName
		}()
		b.tableName = optionalTableName
	}
	newBaseQuery := baseQuery
	if newBaseQuery == "" {
		newBaseQuery = fmt.Sprintf("SELECT * FROM %s", b.tableName)
	}
	return newBaseQuery
}

func (b *BaseRead) GetList(ctx context.Context, opts *database.QueryOpts) error {
	opts.IsList = true
	opts.BaseQuery = b.getBaseQuery(opts.OptionalTableName, opts.BaseQuery)
	return b.db.Read(ctx, opts)
}

// GetDetail - Query Detail with specific Query and return single data
func (b *BaseRead) GetDetail(ctx context.Context, opts *database.QueryOpts) error {
	opts.BaseQuery = b.getBaseQuery(opts.OptionalTableName, opts.BaseQuery)
	return b.db.Read(ctx, opts)
}

// GetDetailById - Generate Query "SELECT * FROM <table_name | optional_table_name> WHERE id = ?"
func (b *BaseRead) GetDetailById(ctx context.Context, resultStruct interface{}, id int, optionalTableName ...string) error {
	opts := &database.QueryOpts{
		Result: resultStruct,
	}

	viewName := ""
	if optionalTableName != nil && len(optionalTableName) > 0 {
		viewName = optionalTableName[0]
	}

	opts.BaseQuery = fmt.Sprintf("%s WHERE id = ?", b.getBaseQuery(viewName, ""))
	return b.db.Read(ctx, opts, id)
}

func (b *BaseRead) Count(ctx context.Context, tableName string, reqData *database.TableRequest) (totalData, totalFiltered int, err error) {
	// TODO: ACTIVATE THIS WHEN USING TRACER
	//trc, ctx := tracer.StartSpanFromContext(ctx, "CommonRepo-CountAll")
	//defer trc.Finish()
	queryTotal := fmt.Sprintf("SELECT COUNT(1) FROM %s ", tableName)
	opts := &database.QueryOpts{
		BaseQuery: queryTotal,
		Result:    &totalData,
		IsList:    false,
	}

	// get total data
	// TODO: ACTIVATE THIS WHEN USING TRACER
	//sqlTrace, ctx := tracer.StartSQLSpanFromContext(ctx, "DB-GetCountQuery", queryTotal)
	//defer sqlTrace.Finish()
	//marshalParam, _ := json.Marshal(params)
	//sqlTrace.SetTag("sqlQuery.params", string(marshalParam))
	if err = b.db.Read(ctx, opts); err != nil {
		return 0, 0, err
	}

	// for get total filtered data
	reqData.Page = 0
	reqData.Limit = 0
	opts.SelectRequest = reqData
	opts.Result = &totalFiltered

	// get total filtered data
	// TODO: ACTIVATE THIS WHEN USING TRACER
	//sqlTrace, ctx := tracer.StartSQLSpanFromContext(ctx, "DB-GetCountQuery", queryTotal)
	//defer sqlTrace.Finish()
	//marshalParam, _ := json.Marshal(params)
	//sqlTrace.SetTag("sqlQuery.params", string(marshalParam))
	if err = b.db.Read(ctx, opts); err != nil {
		return 0, 0, err
	}

	return
}
