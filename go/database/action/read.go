package action

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/kodekoding/phastos/v2/go/database"
	"github.com/kodekoding/phastos/v2/go/helper"
	"github.com/kodekoding/phastos/v2/go/monitoring"
)

// readQueryCacheKey identifies a cached SELECT query template.
type readQueryCacheKey struct {
	Type      reflect.Type
	TableName string
}

// readQueryCache stores pre-computed SELECT base queries per (type, table).
// The base query "SELECT cols FROM table" is invariant for a given struct type
// and table name, so we compute it once.
var readQueryCache sync.Map

// selectByIdCache stores pre-computed "SELECT ... FROM table WHERE id = ?" queries.
// O2: Now also stores the Rebind-ed (final) query string so GetDetailById
// can skip the builder + CachedRebind overhead entirely.
var selectByIdCache sync.Map

// selectByIdCacheEntry holds the pre-computed base query and the final
// rebound query for GetDetailById.
type selectByIdCacheEntry struct {
	BaseQuery    string // "SELECT cols FROM table WHERE id = ?"
	ReboundQuery string // Rebind-ed version (e.g. $1, $2... for PG)
}

// getBaseQueryCached returns the cached base SELECT query for a struct type + table name.
func getBaseQueryCached(t reflect.Type, tableName string, excludeCols, includeCols string) string {
	key := readQueryCacheKey{Type: t, TableName: tableName}

	// Build a lookup key that includes column filters
	type queryKey struct {
		readQueryCacheKey
		ExcludeCols string
		IncludeCols string
	}
	fullKey := queryKey{
		readQueryCacheKey: key,
		ExcludeCols:       excludeCols,
		IncludeCols:       includeCols,
	}

	if cached, ok := readQueryCache.Load(fullKey); ok {
		return cached.(string) //nolint:errcheck
	}

	var selectedColumnStr string
	if excludeCols != "" || includeCols != "" {
		// We still need to compute cols per-call since col selection depends on options.
		// This uses the selectColsCache in struct_cache.go internally.
		selectedCols := helper.GenerateSelectCols(context.TODO(), reflect.New(t).Elem().Interface(),
			helper.WithExcludedCols(excludeCols),
			helper.WithIncludedCols(includeCols),
		)
		if selectedCols != nil {
			selectedColumnStr = strings.Join(selectedCols, ", ")
		}
	}
	if selectedColumnStr == "" {
		selectedColumnStr = "*"
	}

	query := fmt.Sprintf("SELECT %s FROM %s", selectedColumnStr, tableName)
	actual, _ := readQueryCache.LoadOrStore(fullKey, query)
	return actual.(string) //nolint:errcheck
}

// getSelectByIdQueryCached returns the cached selectByIdCacheEntry for a struct type + table name.
func getSelectByIdQueryCached(db database.ISQL, t reflect.Type, tableName string) *selectByIdCacheEntry {
	key := readQueryCacheKey{Type: t, TableName: tableName}
	if cached, ok := selectByIdCache.Load(key); ok {
		return cached.(*selectByIdCacheEntry) //nolint:errcheck
	}
	cols := helper.GenerateSelectCols(context.TODO(), reflect.New(t).Elem().Interface())
	baseQuery := fmt.Sprintf("SELECT %s FROM %s WHERE id = ?", strings.Join(cols, ", "), tableName)
	reboundQuery := db.CachedRebind(baseQuery)
	entry := &selectByIdCacheEntry{
		BaseQuery:    baseQuery,
		ReboundQuery: reboundQuery,
	}
	actual, _ := selectByIdCache.LoadOrStore(key, entry)
	return actual.(*selectByIdCacheEntry) //nolint:errcheck
}

type BaseRead struct {
	*baseAction
}

func NewBaseRead(db database.ISQL, tableName string, isSoftDelete ...bool) *BaseRead {
	sofDelete := true
	if len(isSoftDelete) > 0 {
		sofDelete = isSoftDelete[0]
	}
	return &BaseRead{&baseAction{db, tableName, sofDelete}}
}

func (b *BaseRead) getBaseQuery(ctx context.Context, opts *database.QueryOpts) string {
	// User-specified base query takes precedence
	if opts.BaseQuery != "" {
		return opts.BaseQuery
	}

	tableName := b.tableName
	if opts.OptionalTableName != "" {
		tableName = opts.OptionalTableName
	}

	// Try cached path: needs a Result struct with a known type
	if opts.Result != nil {
		resultVal := reflect.ValueOf(opts.Result)
		if resultVal.Kind() == reflect.Ptr {
			elem := resultVal.Type().Elem()
			if elem.Kind() == reflect.Struct || (elem.Kind() == reflect.Slice && elem.Elem().Kind() == reflect.Struct) {
				structType := elem
				if structType.Kind() == reflect.Slice {
					structType = structType.Elem()
				}
				if structType.Kind() == reflect.Ptr {
					structType = structType.Elem()
				}
				return getBaseQueryCached(structType, tableName, opts.ExcludeColumns, opts.Columns)
			}
		}
	}

	// Slow path: compute on each call (fallback for map/simple types)
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

	return fmt.Sprintf("SELECT %s FROM %s", selectedColumnStr, tableName)
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

	// O2: Fast path for struct types — use pre-computed rebound query,
	// pooled QueryOpts, skip builder + Rebind in Read().
	reflectVal := reflect.ValueOf(resultStruct)
	if reflectVal.Kind() == reflect.Ptr {
		reflectVal = reflectVal.Elem()
	}
	if reflectVal.Kind() == reflect.Struct {
		tableName := b.tableName
		if len(optionalTableName) > 0 {
			tableName = optionalTableName[0]
		}
		entry := getSelectByIdQueryCached(b.db, reflectVal.Type(), tableName)

		// O3: Use pooled QueryOpts
		opts := database.GetQueryOpts()
		opts.Result = resultStruct
		opts.BaseQuery = entry.ReboundQuery
		err := b.db.Read(ctx, opts, id)
		database.PutQueryOpts(opts)
		return err
	}

	// Slow path: original implementation for non-struct inputs
	opts := &database.QueryOpts{
		Result: resultStruct,
	}
	if len(optionalTableName) > 0 {
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
	if len(tableName) > 0 {
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
