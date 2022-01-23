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

func (b *BaseRead) GetList(ctx context.Context, opts *database.QueryOpts) error {
	opts.IsList = true
	if opts.OptionalTableName != "" {
		originalTableName := b.tableName
		defer func() {
			b.tableName = originalTableName
		}()
		b.tableName = opts.OptionalTableName
	}
	if opts.BaseQuery == "" {
		opts.BaseQuery = fmt.Sprintf("SELECT * FROM %s", b.tableName)
	}
	return b.db.Read(ctx, opts)
}

func (b *BaseRead) GetDetailById(ctx context.Context, resultStruct interface{}, id int) error {
	opts := &database.QueryOpts{
		ResultStruct: resultStruct,
	}

	if opts.OptionalTableName != "" {
		originalTableName := b.tableName
		defer func() {
			b.tableName = originalTableName
		}()
		b.tableName = opts.OptionalTableName
	}
	if opts.BaseQuery == "" {
		opts.BaseQuery = fmt.Sprintf("SELECT * FROM %s WHERE id = ?", b.tableName)
	}
	return b.db.Read(ctx, opts, id)
}
