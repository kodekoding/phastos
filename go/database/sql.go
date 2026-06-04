package database

import (
	"container/list"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	sgw "github.com/ashwanthkumar/slack-go-webhook"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // import postgre driver
	_ "github.com/newrelic/go-agent/v3/integrations/nrmysql"
	_ "github.com/newrelic/go-agent/v3/integrations/nrpq"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	_ "gorm.io/driver/mysql" // import mysql driver

	context2 "github.com/kodekoding/phastos/v2/go/context"
	"github.com/kodekoding/phastos/v2/go/env"
	custerr "github.com/kodekoding/phastos/v2/go/error"
	plog "github.com/kodekoding/phastos/v2/go/log"
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
	log := plog.Get()
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
	if cfgMaxConnLifeTime == 0 {
		// set default max conn lifetime to 5 minutes
		cfgMaxConnLifeTime = 300
	}
	maxLifetime := time.Duration(cfgMaxConnLifeTime) * time.Second
	db.SetConnMaxLifetime(maxLifetime)

	cfgMaxIdleTime, _ := strconv.Atoi(os.Getenv("DATABASE_CONN_MAX_IDLE_TIME"))

	maxIdleTime := time.Duration(cfgMaxIdleTime) * time.Second
	if maxIdleTime == 0 {
		// set default max iddle time to 45 seconds
		maxIdleTime = 45
	}
	db.SetConnMaxIdleTime(maxIdleTime)

	// set maximum open connection to DB
	maxOpenConn, _ := strconv.Atoi(os.Getenv("DATABASE_MAX_OPEN_CONN"))
	if maxOpenConn == 0 {
		maxOpenConn = 10
	}
	db.SetMaxOpenConns(maxOpenConn)

	maxIdleConn, _ := strconv.Atoi(os.Getenv("DATABASE_MAX_IDLE_CONN"))
	if maxIdleConn == 0 {
		maxIdleConn = 2
	}
	db.SetMaxIdleConns(maxIdleConn)
	return db, nil
}

func (this *SQL) Read(ctx context.Context, opts *QueryOpts, additionalParams ...interface{}) error {
	ctx, span := monitoring.StartSpan(ctx, "PhastosDB-Read")
	defer span.End()
	span.SetAttributes(attribute.String("db.system", this.engine))
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

	span.SetAttributes(attribute.String("db.operation", "Read"))
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
	span.SetAttributes(
		attribute.String("db.statement", opts.BaseQuery),
		attribute.String("db.params", string(byteParam)),
	)
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
		span.SetAttributes(attribute.String("db.statement", finalQuery))
		stmt, err := opts.Trx.PreparexContext(ctx, finalQuery)
		if err != nil {
			_, err = sendNilResponse(err, "phastos.database.ReadTrx.PrepareContext", finalQuery, params)
			return err
		}
		defer stmt.Close() //nolint:errcheck

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
		finalQuery = this.CachedRebind(query.String())
		opts.query = finalQuery

		span.SetAttributes(attribute.String("db.statement", finalQuery))

		// Try cached prepared statement for reads (O1).
		// For fixed queries (e.g. GetDetailById), this eliminates per-call
		// Prepare overhead. Fall back to direct DB call on cache miss or error.
		if stmt, stmtErr := this.getReadStmtx(ctx, finalQuery, opts.UseMaster); stmtErr == nil { //nolint:sqlclosecheck
			if opts.IsList {
				if err := stmt.SelectContext(ctx, opts.Result, params...); err != nil {
					evictReadStmt(finalQuery)
					_, err = sendNilResponse(err, "phastos.database.Read.SelectContext", finalQuery, params)
					return err
				}
			} else {
				if err := stmt.GetContext(ctx, opts.Result, params...); err != nil {
					evictReadStmt(finalQuery)
					_, err = sendNilResponse(err, "phastos.database.Read.GetContext", finalQuery, params)
					return err
				}
			}
		} else {
			// Fallback: no cached stmt, use direct DB call
			db := this.Follower
			if opts.UseMaster {
				db = this.Master
			}
			if opts.IsList {
				if err := db.SelectContext(ctx, opts.Result, finalQuery, params...); err != nil {
					_, err = sendNilResponse(err, "phastos.database.Read.SelectContext", finalQuery, params)
					return err
				}
			} else {
				if err := db.GetContext(ctx, opts.Result, finalQuery, params...); err != nil {
					_, err = sendNilResponse(err, "phastos.database.Read.GetContext", finalQuery, params)
					return err
				}
			}
		}
	}

	this.checkSQLWarning(ctx, finalQuery, start, params)
	return nil
}

// builderPool pools strings.Builder instances used in Read/Write to reduce
// heap allocations on the hot query-building path.
var builderPool = sync.Pool{
	New: func() any {
		return new(strings.Builder)
	},
}

func getBuilder() *strings.Builder {
	b := builderPool.Get().(*strings.Builder) //nolint:errcheck
	b.Reset()
	return b
}

func putBuilder(b *strings.Builder) {
	builderPool.Put(b)
}

const (
	defaultMaxStmtCacheSize   = 500
	defaultMaxRebindCacheSize = 2000
)

// boundedSyncMap wraps sync.Map with a capacity limit. When full, it
// evicts an arbitrary entry (via Range) to make room. The onEvict
// callback is invoked for evicted entries (e.g. to Close() a stmt).
type boundedSyncMap struct {
	m       sync.Map
	count   atomic.Int64
	cap     int64
	onEvict func(key, val any)
}

func newBoundedSyncMap(cap int, onEvict func(key, val any)) *boundedSyncMap {
	return &boundedSyncMap{cap: int64(cap), onEvict: onEvict}
}

func (c *boundedSyncMap) Load(key any) (any, bool) {
	return c.m.Load(key)
}

func (c *boundedSyncMap) LoadOrStore(key, val any) (actual any, loaded bool) {
	actual, loaded = c.m.LoadOrStore(key, val)
	if !loaded {
		if c.count.Add(1) > c.cap {
			c.evictOne()
		}
	}
	return
}

func (c *boundedSyncMap) LoadAndDelete(key any) (any, bool) {
	val, ok := c.m.LoadAndDelete(key)
	if ok {
		c.count.Add(-1)
	}
	return val, ok
}

func (c *boundedSyncMap) Delete(key any) {
	c.LoadAndDelete(key)
}

func (c *boundedSyncMap) Store(key, val any) {
	_, loaded := c.m.Swap(key, val)
	if !loaded {
		if c.count.Add(1) > c.cap {
			c.evictOne()
		}
	}
}

func (c *boundedSyncMap) Range(f func(key, val any) bool) {
	c.m.Range(f)
}

func (c *boundedSyncMap) Len() int64 {
	return c.count.Load()
}

func (c *boundedSyncMap) evictOne() {
	c.m.Range(func(key, val any) bool {
		if v, ok := c.m.LoadAndDelete(key); ok {
			c.count.Add(-1)
			if c.onEvict != nil {
				c.onEvict(key, v)
			}
		}
		return false
	})
}

// rebindCache caches Rebind results per query string to avoid repeated
// string scanning/replacement. For queries built from cached templates
// (e.g. UpdateById), the same query string is produced every time.
var rebindCache = newBoundedSyncMap(defaultMaxRebindCacheSize, nil)

// lruStmtCache is a bounded, LRU-evicting cache for prepared statements.
// Eviction does NOT call Close() — entries removed at capacity are simply
// discarded from the cache. DB-side prepared statements are cleaned up
// when the connection is recycled (DATABASE_CONN_MAX_LIFETIME).
// This guarantees no "sql: statement is closed" errors from eviction.
type lruStmtCache struct {
	mu    sync.Mutex
	ll    *list.List
	items map[string]*list.Element
	cap   int
}

type lruEntry struct {
	key   string
	value any
}

func newLRUStmtCache(cap int) *lruStmtCache {
	return &lruStmtCache{
		ll:    list.New(),
		items: make(map[string]*list.Element),
		cap:   cap,
	}
}

func (c *lruStmtCache) Load(key any) (any, bool) {
	k, ok := key.(string)
	if !ok {
		return nil, false
	}
	c.mu.Lock()
	if el, ok := c.items[k]; ok {
		c.ll.MoveToFront(el)
		c.mu.Unlock()
		return el.Value.(*lruEntry).value, true //nolint:errcheck
	}
	c.mu.Unlock()
	return nil, false
}

func (c *lruStmtCache) LoadOrStore(key, val any) (actual any, loaded bool) {
	k, ok := key.(string)
	if !ok {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[k]; ok {
		c.ll.MoveToFront(el)
		return el.Value.(*lruEntry).value, true //nolint:errcheck
	}
	if c.ll.Len() >= c.cap {
		c.evictLocked()
	}
	entry := &lruEntry{key: k, value: val}
	el := c.ll.PushFront(entry)
	c.items[k] = el
	return val, false
}

func (c *lruStmtCache) LoadAndDelete(key any) (any, bool) {
	k, ok := key.(string)
	if !ok {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[k]; ok {
		c.ll.Remove(el)
		delete(c.items, k)
		return el.Value.(*lruEntry).value, true //nolint:errcheck
	}
	return nil, false
}

func (c *lruStmtCache) Delete(key any) {
	c.LoadAndDelete(key)
}

func (c *lruStmtCache) Store(key, val any) {
	k, ok := key.(string)
	if !ok {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[k]; ok {
		el.Value.(*lruEntry).value = val //nolint:errcheck
		c.ll.MoveToFront(el)
		return
	}
	if c.ll.Len() >= c.cap {
		c.evictLocked()
	}
	entry := &lruEntry{key: k, value: val}
	el := c.ll.PushFront(entry)
	c.items[k] = el
}

func (c *lruStmtCache) Range(f func(key, val any) bool) {
	c.mu.Lock()
	keys := make([]string, 0, len(c.items))
	for k := range c.items {
		keys = append(keys, k)
	}
	c.mu.Unlock()
	for _, k := range keys {
		c.mu.Lock()
		el, ok := c.items[k]
		if !ok {
			c.mu.Unlock()
			continue
		}
		val := el.Value.(*lruEntry).value //nolint:errcheck
		c.mu.Unlock()
		if !f(k, val) {
			return
		}
	}
}

func (c *lruStmtCache) Len() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return int64(c.ll.Len())
}

func (c *lruStmtCache) evictLocked() {
	back := c.ll.Back()
	if back == nil {
		return
	}
	entry := back.Value.(*lruEntry) //nolint:errcheck
	c.ll.Remove(back)
	delete(c.items, entry.key)
}

// readStmtCache caches *sql.Stmt per query string for non-transaction
// read paths.
var readStmtCache = newLRUStmtCache(defaultMaxStmtCacheSize)

// writeStmtCache caches *sql.Stmt per query string for non-transaction
// write paths.
var writeStmtCache = newLRUStmtCache(defaultMaxStmtCacheSize)

// getReadStmtx returns a cached *sqlx.Stmt for the given query on the
// follower (or master) DB, preparing and caching it on first access.
func (this *SQL) getReadStmtx(ctx context.Context, query string, useMaster bool) (*sqlx.Stmt, error) {
	if stmt, ok := readStmtCache.Load(query); ok {
		return stmt.(*sqlx.Stmt), nil //nolint:errcheck
	}
	db := this.Follower
	if useMaster {
		db = this.Master
	}
	followerDB, ok := db.(*sqlx.DB)
	if !ok {
		return nil, errors.New("follower DB is not *sqlx.DB")
	}
	stmt, err := followerDB.PreparexContext(ctx, query)
	if err != nil {
		return nil, err
	}
	actual, loaded := readStmtCache.LoadOrStore(query, stmt)
	if loaded {
		stmt.Close() //nolint:errcheck,sqlclosecheck
	}
	return actual.(*sqlx.Stmt), nil //nolint:errcheck
}

// evictReadStmt removes a cached read prepared statement.
func evictReadStmt(query string) {
	if stmt, ok := readStmtCache.LoadAndDelete(query); ok {
		stmt.(*sqlx.Stmt).Close() //nolint:errcheck
	}
}

// EvictReadStmt is the exported version for use by action package fast paths.
func EvictReadStmt(query string) {
	evictReadStmt(query)
}

// getWriteStmt returns a cached *sql.Stmt for the given query, preparing
// and caching it on first access. Callers should NOT close the returned
// stmt — it is owned by the cache.
//
// We use the underlying *sqlx.DB directly because the Master interface
// does not expose PrepareContext. At runtime, SQL.Master is always a
// *sqlx.DB which embeds *sql.DB.
func (this *SQL) getWriteStmt(ctx context.Context, query string) (*sql.Stmt, error) {
	if stmt, ok := writeStmtCache.Load(query); ok {
		return stmt.(*sql.Stmt), nil //nolint:errcheck
	}
	// Access the underlying *sqlx.DB to call PreparexContext.
	// The prepared sqlx.Stmt wraps a *sql.Stmt; we use the raw stmt
	// for direct ExecContext calls.
	masterDB, ok := this.Master.(*sqlx.DB)
	if !ok {
		// Fallback: can't cache, just prepare inline
		return nil, errors.New("master DB is not *sqlx.DB")
	}
	stmt, err := masterDB.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	actual, loaded := writeStmtCache.LoadOrStore(query, stmt)
	if loaded {
		stmt.Close() //nolint:errcheck,sqlclosecheck
	}
	return actual.(*sql.Stmt), nil //nolint:errcheck

}

// EvictWriteStmt removes a cached prepared statement (called on execution failure).
// Exported for use by action package fast paths.
func EvictWriteStmt(query string) {
	evictWriteStmt(query)
}

// evictWriteStmt is the internal version.
func evictWriteStmt(query string) {
	if stmt, ok := writeStmtCache.LoadAndDelete(query); ok {
		stmt.(*sql.Stmt).Close() //nolint:errcheck
	}
}

// IsPostgres returns true if the engine is a PostgreSQL variant.
func (this *SQL) IsPostgres() bool {
	active, valid := postgresEngineGroup[this.engine]
	return valid && active
}

// Engine returns the database engine string (e.g. "mysql", "postgres", "nrmysql").
func (this *SQL) Engine() string {
	return this.engine
}

// MySQLEngineGroupActive returns whether the given engine is a MySQL variant.
func MySQLEngineGroupActive(engine string) (bool, bool) {
	active, valid := mySQLEngineGroup[engine]
	return active, valid
}

// CheckSQLWarning is the exported version of checkSQLWarning for use by
// action package fast paths.
func (this *SQL) CheckSQLWarning(ctx context.Context, query string, start time.Time, params ...interface{}) {
	this.checkSQLWarning(ctx, query, start, params...)
}

// GetWriteStmt is the exported version of getWriteStmt for use by action
// package fast paths that bypass Write() entirely.
func (this *SQL) GetWriteStmt(ctx context.Context, query string) (*sql.Stmt, error) {
	return this.getWriteStmt(ctx, query)
}

// CachedRebind returns the cached Rebind result for the given query string,
// computing and caching it on first access.
func (this *SQL) CachedRebind(query string) string {
	if cached, ok := rebindCache.Load(query); ok {
		return cached.(string) //nolint:errcheck
	}
	result := this.Master.Rebind(query)
	actual, _ := rebindCache.LoadOrStore(query, result)
	return actual.(string) //nolint:errcheck
}

func (this *SQL) Write(ctx context.Context, opts *QueryOpts, isSoftDelete ...bool) (*CUDResponse, error) {
	ctx, span := monitoring.StartSpan(ctx, "PhastosDB-Write")
	defer span.End()
	span.SetAttributes(attribute.String("db.system", this.engine))
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
	if len(isSoftDelete) > 0 {
		softDelete = isSoftDelete[0]
	}
	data := opts.CUDRequest
	cols := strings.Join(data.Cols, ",")
	query := getBuilder()
	defer putBuilder(query)
	tableName := data.TableName
	switch data.Action {
	case ActionInsert:
		query.WriteString("INSERT INTO ")
		query.WriteString(tableName)
		query.WriteString(" (")
		query.WriteString(cols)
		query.WriteString(") VALUES (?" + strings.Repeat(",?", len(data.Cols)-1) + ")")
		if active, valid := postgresEngineGroup[this.engine]; valid && active {
			query.WriteString(" RETURNING id")
		}
	case ActionBulkInsert:
		query.WriteString("INSERT INTO ")
		query.WriteString(tableName)
		query.WriteString(" (")
		query.WriteString(data.ColsInsert)
		query.WriteString(") VALUES ")
		query.WriteString(data.BulkValues)
	case ActionBulkUpdate:
		query.WriteString("UPDATE ")
		query.WriteString(tableName)
		query.WriteString(" AS main_table JOIN (")
		query.WriteString(data.BulkValues)
		query.WriteString(") AS join_table ")
		query.WriteString(data.BulkQuery)
	case ActionUpsert:
		colsUpdate := strings.Join(data.Cols, ",")
		query.WriteString("INSERT INTO ")
		query.WriteString(data.TableName)
		query.WriteString(" (")
		query.WriteString(data.ColsInsert)
		query.WriteString(") VALUES (?" + strings.Repeat(",?", len(data.Cols)-1) + ") ON DUPLICATE KEY UPDATE ")
		query.WriteString(colsUpdate)
	case ActionUpdateById:
		query.WriteString("UPDATE ")
		query.WriteString(tableName)
		query.WriteString(" SET ")
		query.WriteString(cols)
		query.WriteString(" WHERE id = ?")
	case ActionDeleteById:
		if softDelete {
			query.WriteString("UPDATE ")
			query.WriteString(tableName)
			query.WriteString(" SET deleted_at = now() WHERE id = ?")
		} else {
			query.WriteString("DELETE FROM ")
			query.WriteString(tableName)
			query.WriteString(" WHERE id = ?")
		}
	case ActionUpdate:
		query.WriteString("UPDATE ")
		query.WriteString(tableName)
		query.WriteString(" SET ")
		query.WriteString(cols)
	case ActionDelete:
		if softDelete {
			query.WriteString("UPDATE ")
			query.WriteString(tableName)
			query.WriteString(" SET deleted_at = now()")
		} else {
			query.WriteString("DELETE FROM ")
			query.WriteString(tableName)
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

	finalQuery := this.CachedRebind(query.String())
	// reset and replace the final query with rebind-ed query
	query.Reset()
	query.WriteString(finalQuery)
	result := GetCUDResponse()
	result.query = query.String()
	result.params = data.Values
	trx := opts.Trx
	start := time.Now()
	lastInsertID := int64(0)
	rowsAffected := int64(0)

	byteParam, _ := json.Marshal(data.Values)
	span.SetAttributes(
		attribute.String("db.statement", query.String()),
		attribute.String("db.params", string(byteParam)),
		attribute.String("db.operation", data.Action),
	)
	if trx != nil {
		isPostgres := false
		if active, valid := postgresEngineGroup[this.engine]; valid && active {
			isPostgres = true
			// RETURNING id is already appended in the ActionInsert case above.
			// No need to append it again here.
		}
		stmt, err := trx.PreparexContext(ctx, query.String()) //nolint:govet // shadow
		if err != nil {
			_, err = sendNilResponse(err, "phastos.database.Write.PrepareContext", query.String(), data.Values)
			return result, err
		}
		defer stmt.Close() //nolint:errcheck

		if isPostgres && data.Action == ActionInsert {
			if err = stmt.QueryRowContext(ctx, data.Values...).Scan(&lastInsertID); err != nil {
				_, err = sendNilResponse(err, "phastos.database.Write.QueryRowContext", query, data.Values)
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
			// For Insert: RETURNING id already appended in switch case above.
			// For Insert on PG: use QueryRowContext+Scan (needs RETURNING).
			// For Update/UpdateById/Delete: use cached prepared statement.
			if data.Action == ActionInsert {
				if err = this.Master.QueryRowContext(ctx, query.String(), data.Values...).Scan(&lastInsertID); err != nil {
					_, err = sendNilResponse(err, "phastos.database.Write.QueryRowContext", query, data.Values)
					if err == nil {
						result.RowsAffected = 1
						result.Status = true
					}
					return result, err
				}
			} else {
				stmt, stmtErr := this.getWriteStmt(ctx, query.String()) //nolint:sqlclosecheck
				if stmtErr != nil {
					_, _ = sendNilResponse(stmtErr, "phastos.database.Write.PrepareStmt", query.String(), data.Values)
					return result, stmtErr
				}
				exec, err = stmt.ExecContext(ctx, data.Values...)
				if err != nil {
					evictWriteStmt(query.String())
					_, err = sendNilResponse(err, "phastos.database.Write.ExecContext", query.String(), data.Values)
					return result, err
				}
			}
		} else {
			stmt, stmtErr := this.getWriteStmt(ctx, query.String()) //nolint:sqlclosecheck
			if stmtErr != nil {
				_, _ = sendNilResponse(stmtErr, "phastos.database.Write.PrepareStmt", query.String(), data.Values)
				return result, stmtErr
			}
			exec, err = stmt.ExecContext(ctx, data.Values...)
			if err != nil {
				evictWriteStmt(query.String())
				_, err = sendNilResponse(err, "phastos.database.Write.WithoutTrx.ExecContext", query.String(), data.Values)
				return result, err
			}
		}
	}
	// Determine rowsAffected from sql.Result when available.
	// Previously this was hardcoded to rowsAffected++ (always 1) and only
	// overwritten for MySQL, which gave incorrect results for PostgreSQL.
	if exec != nil {
		if ra, raErr := exec.RowsAffected(); raErr == nil {
			rowsAffected = ra
		} else {
			rowsAffected = 1 // fallback when driver doesn't support it
		}
	} else {
		// PostgreSQL path: uses QueryRowContext+Scan instead of Exec,
		// so exec is nil. Set rowsAffected to 1 as a safe default.
		rowsAffected = 1
	}
	result.LastInsertID = lastInsertID
	result.RowsAffected = rowsAffected

	this.checkSQLWarning(ctx, query.String(), start, data.Values)

	if active, valid := mySQLEngineGroup[this.engine]; valid && active {
		lastInsertID, err = exec.LastInsertId()
		if err == nil {
			result.LastInsertID = lastInsertID
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
	log := plog.Ctx(ctx)
	_, span := monitoring.StartSpan(ctx, "PhastosDB-GeneratingAddOnQuery")
	defer span.End()
	span.SetAttributes(attribute.String("engine", reqData.engine))
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
	whereResult := strings.ReplaceAll(addOnBuilder.String(), " OR )", ")")
	whereResult = " " + whereResult
	return whereResult, addOnParams, nil
}

func checkKeyword(_ context.Context, reqData *TableRequest, addOnBuilder *strings.Builder, addOnParams *[]interface{}) error {
	if reqData.Keyword != "" {
		if reqData.SearchColsStr == "" {
			return errors.New("Keyword Cols is required when Keyword Field is filled")
		}
		reqData.SearchCols = strings.Split(reqData.SearchColsStr, ",")
		if reqData.InitiateWhere != nil {
			addOnBuilder.WriteString(" AND ")
		}
		addOnBuilder.WriteString("(")
		// Sequential loop is faster than goroutine+mutex for trivial string operations.
		// Goroutine scheduling + mutex contention overhead exceeds the cost of
		// a simple fmt.Fprintf + append, especially with typical 2-5 search columns.
		for _, col := range reqData.SearchCols {
			fmt.Fprintf(addOnBuilder, "%s LIKE ? OR ", col)
			*addOnParams = append(*addOnParams, generateParamArgsForLike(reqData.Keyword))
		}
		addOnBuilder.WriteString(")")
	}
	return nil
}

func checkSortParam(_ context.Context, reqData *TableRequest, addOnBuilder *strings.Builder) {
	// tracing
	//trc, ctx := tracer.StartSpanFromContext(ctx, "CommonRepo-checkSortParam")
	//defer trc.Finish()
	if reqData.OrderBy != "" {
		fmt.Fprintf(addOnBuilder, " ORDER BY %s", reqData.OrderBy)
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
			fmt.Fprintf(addOnBuilder, "DATE_FORMAT(%s, '%%Y-%%m-%%d %%H:%%i:%%s') >= STR_TO_DATE(?, '%%Y-%%m-%%d %%H:%%i:%%s')", col)
		} else {
			fmt.Fprintf(addOnBuilder, "%s >= ?", col)
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
			fmt.Fprintf(addOnBuilder, "DATE_FORMAT(%s, '%%Y-%%m-%%d %%H:%%i:%%s') <= STR_TO_DATE(?, '%%Y-%%m-%%d %%H:%%i:%%s')", col)
		} else {
			fmt.Fprintf(addOnBuilder, "%s <= ?", col)
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
			if _, isMySQL := mySQLEngineGroup[reqData.engine]; isMySQL {
				fmt.Fprintf(addOnBuilder, "(%s IS NULL OR CAST(%s AS CHAR(20)) = '0000-00-00 00:00:00') ", col, col)
			} else {
				fmt.Fprintf(addOnBuilder, "(%s IS NULL) ", col)
			}
		} else {
			fmt.Fprintf(addOnBuilder, "(%s IS NOT NULL) ", col)
		}
	}
}

func checkInitiateWhere(_ context.Context, reqData *TableRequest, addOnBuilder *strings.Builder, addOnParams *[]interface{}) {
	// tracing
	//trc, ctx := tracer.StartSpanFromContext(ctx, "CommonRepo-checkInitiateWhere")
	//defer trc.Finish()
	if reqData.InitiateWhere != nil {
		for _, condition := range reqData.InitiateWhere {
			fmt.Fprintf(addOnBuilder, "%s AND ", condition)
		}
		initWhere := addOnBuilder.String()
		initWhere = initWhere[:len(initWhere)-5]
		*addOnParams = append(*addOnParams, reqData.InitiateWhereValues...)

		addOnBuilder.Reset()
		addOnBuilder.WriteString(initWhere)
	}
}

func sendNilResponse(err error, ctxMsg string, params ...interface{}) (interface{}, error) { //nolint:unparam
	if strings.Contains(err.Error(), "no rows") {
		// return nil for result struct if no rows
		return nil, nil
	}

	customErr := custerr.New(err).SetCode(500)
	for i, paramValue := range params {
		keyParam := fmt.Sprintf("param %d", i+1)
		customErr.AppendData(keyParam, paramValue) //nolint:errcheck
	}
	return nil, errors.Wrap(customErr, ctxMsg)
}
