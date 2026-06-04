package action

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kodekoding/phastos/v2/go/database"
	"github.com/kodekoding/phastos/v2/go/monitoring"
)

// Fake driver for testing the *database.SQL fast path in UpdateById/DeleteById.
type actionFakeDriver struct{}
type actionFakeConn struct{}
type actionFakeStmt struct{}
type actionFakeTx struct{}
type actionFakeResult struct {
	lastInsertID int64
	rowsAffected int64
}

func (d actionFakeDriver) Open(name string) (driver.Conn, error) {
	return &actionFakeConn{}, nil
}
func (c *actionFakeConn) Prepare(query string) (driver.Stmt, error) {
	return &actionFakeStmt{}, nil
}
func (c *actionFakeConn) Close() error  { return nil }
func (c *actionFakeConn) Begin() (driver.Tx, error) { return &actionFakeTx{}, nil }
func (s *actionFakeStmt) Close() error                                    { return nil }
func (s *actionFakeStmt) NumInput() int                                    { return -1 }
func (s *actionFakeStmt) Exec(args []driver.Value) (driver.Result, error) {
	return &actionFakeResult{rowsAffected: 1, lastInsertID: 42}, nil
}
func (s *actionFakeStmt) Query(args []driver.Value) (driver.Rows, error)   { return nil, driver.ErrSkip }
func (t *actionFakeTx) Commit() error   { return nil }
func (t *actionFakeTx) Rollback() error { return nil }
func (r *actionFakeResult) LastInsertId() (int64, error)  { return r.lastInsertID, nil }
func (r *actionFakeResult) RowsAffected() (int64, error)  { return r.rowsAffected, nil }

// actionFakeErrorDriver returns an error-returning Stmt for testing fast-path error handling.
type actionFakeErrorDriver struct{}
type actionFakeErrorConn struct{}
type actionFakeErrorStmt struct{}

func (d actionFakeErrorDriver) Open(name string) (driver.Conn, error) {
	return &actionFakeErrorConn{}, nil
}
func (c *actionFakeErrorConn) Prepare(query string) (driver.Stmt, error) {
	return &actionFakeErrorStmt{}, nil
}
func (c *actionFakeErrorConn) Close() error  { return nil }
func (c *actionFakeErrorConn) Begin() (driver.Tx, error) { return nil, errors.New("txn error") }
func (s *actionFakeErrorStmt) Close() error                                    { return nil }
func (s *actionFakeErrorStmt) NumInput() int                                    { return -1 }
func (s *actionFakeErrorStmt) Exec(args []driver.Value) (driver.Result, error) {
	return nil, errors.New("exec error")
}
func (s *actionFakeErrorStmt) Query(args []driver.Value) (driver.Rows, error)   { return nil, driver.ErrSkip }
func (r *actionFakeErrorResult) LastInsertId() (int64, error)  { return 0, errors.New("no last insert id") }
func (r *actionFakeErrorResult) RowsAffected() (int64, error)  { return 0, errors.New("no rows affected") }

type actionFakeErrorResult struct{}

func init() {
	sql.Register("action_fake_driver", &actionFakeDriver{})
	sql.Register("action_fake_error_driver", &actionFakeErrorDriver{})
}

// newSQLForErrorAction creates a *database.SQL with an error-returning fake driver.
func newSQLForErrorAction() *database.SQL {
	db, _ := sqlx.Open("action_fake_error_driver", "")
	return &database.SQL{
		Master:   db,
		Follower: db,
	}
}

// newSQLForAction creates a *database.SQL with a fake *sqlx.DB for fast path testing.
func newSQLForAction() *database.SQL {
	db, _ := sqlx.Open("action_fake_driver", "")
	return &database.SQL{
		Master:   db,
		Follower: db,
	}
}

// testStruct is a test entity with db tags for reflection-based helper functions.
type testStruct struct {
	ID        int64  `db:"id"`
	Name      string `db:"name"`
	Email     string `db:"email"`
	Status    string `db:"status"`
}

func (t testStruct) TableName() string { return "test_table" }

// stubSQL implements database.ISQL manually without gomock matchers.
type stubSQL struct {
	t                *testing.T
	readFn           func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error
	writeFn          func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error)
	cachedRebindFn   func(query string) string
	rebindFn         func(sql string) string
	masterDB         interface{}
}

func newStubSQL(t *testing.T) *stubSQL {
	return &stubSQL{
		t: t,
		cachedRebindFn: func(query string) string {
			return query
		},
		rebindFn: func(sql string) string {
			return sql
		},
	}
}

func (s *stubSQL) Read(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
	if s.readFn != nil {
		return s.readFn(ctx, opts, additionalParams...)
	}
	return nil
}

func (s *stubSQL) Write(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
	if s.writeFn != nil {
		return s.writeFn(ctx, opts, isSoftDelete...)
	}
	return &database.CUDResponse{Status: true, RowsAffected: 1}, nil
}

func (s *stubSQL) CachedRebind(query string) string {
	if s.cachedRebindFn != nil {
		return s.cachedRebindFn(query)
	}
	return query
}

func (s *stubSQL) Rebind(sql string) string {
	if s.rebindFn != nil {
		return s.rebindFn(sql)
	}
	return sql
}

func (s *stubSQL) GetTransaction() database.Transactions { return nil }
func (s *stubSQL) Begin() (*sql.Tx, error) { return nil, nil }
func (s *stubSQL) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) { return nil, nil }
func (s *stubSQL) Beginx() (*sqlx.Tx, error) { return nil, nil }
func (s *stubSQL) BindNamed(query string, arg interface{}) (string, []interface{}, error) {
	return query, nil, nil
}
func (s *stubSQL) Exec(query string, args ...interface{}) (sql.Result, error) { return nil, nil }
func (s *stubSQL) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return nil, nil
}
func (s *stubSQL) Get(dest interface{}, query string, args ...interface{}) error { return nil }
func (s *stubSQL) GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return nil
}
func (s *stubSQL) NamedExec(query string, arg interface{}) (sql.Result, error) { return nil, nil }
func (s *stubSQL) NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error) {
	return nil, nil
}
func (s *stubSQL) NamedQuery(query string, arg interface{}) (*sqlx.Rows, error) { return nil, nil }
func (s *stubSQL) NamedQueryContext(ctx context.Context, query string, arg interface{}) (*sqlx.Rows, error) {
	return nil, nil
}
func (s *stubSQL) Query(query string, args ...interface{}) (*sql.Rows, error) { return nil, nil }
func (s *stubSQL) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return nil, nil
}
func (s *stubSQL) QueryRow(query string, args ...interface{}) *sql.Row { return nil }
func (s *stubSQL) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return nil
}
func (s *stubSQL) QueryRowx(query string, args ...interface{}) *sqlx.Row { return nil }
func (s *stubSQL) QueryRowxContext(ctx context.Context, query string, args ...interface{}) *sqlx.Row {
	return nil
}
func (s *stubSQL) QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error) {
	return nil, nil
}
func (s *stubSQL) Select(dest interface{}, query string, args ...interface{}) error { return nil }
func (s *stubSQL) SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return nil
}

// --- Tests for NewBase, NewBaseRead, NewBaseWrite ---

func TestNewBase(t *testing.T) {
	t.Run("default soft delete", func(t *testing.T) {
		db := newStubSQL(t)
		base := NewBase(db, "test_table")
		require.NotNil(t, base)
		assert.NotNil(t, base.ReadRepo)
		assert.NotNil(t, base.BaseWrites)
	})

	t.Run("with soft delete true", func(t *testing.T) {
		db := newStubSQL(t)
		base := NewBase(db, "test_table", true)
		require.NotNil(t, base)
	})

	t.Run("with soft delete false", func(t *testing.T) {
		db := newStubSQL(t)
		base := NewBase(db, "test_table", false)
		require.NotNil(t, base)
	})
}

func TestNewBaseRead(t *testing.T) {
	t.Run("default soft delete", func(t *testing.T) {
		db := newStubSQL(t)
		br := NewBaseRead(db, "test_table")
		require.NotNil(t, br)
		assert.True(t, br.isSoftDelete)
		assert.Equal(t, "test_table", br.tableName)
	})

	t.Run("with soft delete false", func(t *testing.T) {
		db := newStubSQL(t)
		br := NewBaseRead(db, "test_table", false)
		require.NotNil(t, br)
		assert.False(t, br.isSoftDelete)
	})
}

func TestNewBaseWrite(t *testing.T) {
	t.Run("default soft delete", func(t *testing.T) {
		db := newStubSQL(t)
		bw := NewBaseWrite(db, "test_table")
		require.NotNil(t, bw)
		assert.True(t, bw.isSoftDelete)
	})

	t.Run("with soft delete false", func(t *testing.T) {
		db := newStubSQL(t)
		bw := NewBaseWrite(db, "test_table", false)
		require.NotNil(t, bw)
		assert.False(t, bw.isSoftDelete)
	})
}

// --- Tests for BaseRead ---

// newRelicCtx creates a context with a New Relic transaction for testing monitoring segments.
// The returned context should be used in tests that need to verify txn.StartSegment paths.
func newRelicCtx() context.Context {
	nr := monitoring.InitNewRelic(
		monitoring.WithAppName("test-action"),
		monitoring.WithLicenseKey("0123456789012345678901234567890123456789"),
	)
	if nr == nil || nr.GetApp() == nil {
		return context.Background()
	}
	txn := nr.GetApp().StartTransaction("test")
	return monitoring.NewContext(context.Background(), txn)
}

func TestBaseRead_GetList(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db := newStubSQL(t)
		calledWithOpts := false
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			calledWithOpts = true
			assert.True(t, opts.IsList)
			assert.Contains(t, opts.BaseQuery, "SELECT")
			return nil
		}
		br := NewBaseRead(db, "test_table")
		opts := &database.QueryOpts{
			Result: &testStruct{},
		}
		err := br.GetList(context.Background(), opts)
		assert.NoError(t, err)
		assert.True(t, calledWithOpts)
	})

	t.Run("error from db", func(t *testing.T) {
		db := newStubSQL(t)
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			return errors.New("db error")
		}
		br := NewBaseRead(db, "test_table")
		err := br.GetList(context.Background(), &database.QueryOpts{})
		assert.Error(t, err)
	})

	t.Run("with monitoring transaction", func(t *testing.T) {
		db := newStubSQL(t)
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			return nil
		}
		br := NewBaseRead(db, "test_table")
		ctx := newRelicCtx()
		err := br.GetList(ctx, &database.QueryOpts{})
		assert.NoError(t, err)
	})
}

func TestBaseRead_GetDetail(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db := newStubSQL(t)
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			assert.False(t, opts.IsList)
			assert.Contains(t, opts.BaseQuery, "SELECT")
			return nil
		}
		br := NewBaseRead(db, "test_table")
		err := br.GetDetail(context.Background(), &database.QueryOpts{
			Result: &testStruct{},
		})
		assert.NoError(t, err)
	})

	t.Run("error from db", func(t *testing.T) {
		db := newStubSQL(t)
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			return errors.New("db error")
		}
		br := NewBaseRead(db, "test_table")
		err := br.GetDetail(context.Background(), &database.QueryOpts{})
		assert.Error(t, err)
	})

	t.Run("with monitoring transaction", func(t *testing.T) {
		db := newStubSQL(t)
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			return nil
		}
		br := NewBaseRead(db, "test_table")
		ctx := newRelicCtx()
		err := br.GetDetail(ctx, &database.QueryOpts{})
		assert.NoError(t, err)
	})
}

func TestBaseRead_GetDetailById(t *testing.T) {
	// Reset caches to ensure clean state
	readQueryCache = sync.Map{}
	selectByIdCache = sync.Map{}

	t.Run("with struct result", func(t *testing.T) {
		db := newStubSQL(t)
		db.cachedRebindFn = func(query string) string {
			return query
		}
		var capturedID interface{}
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			if len(additionalParams) > 0 {
				capturedID = additionalParams[0]
			}
			assert.Contains(t, opts.BaseQuery, "SELECT")
			assert.Contains(t, opts.BaseQuery, "WHERE id =")
			return nil
		}
		br := NewBaseRead(db, "test_table")
		result := &testStruct{}
		err := br.GetDetailById(context.Background(), result, 42)
		assert.NoError(t, err)
		assert.Equal(t, 42, capturedID)
	})

	t.Run("with struct result and optional table name", func(t *testing.T) {
		db := newStubSQL(t)
		db.cachedRebindFn = func(query string) string {
			return query
		}
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			return nil
		}
		br := NewBaseRead(db, "test_table")
		result := &testStruct{}
		err := br.GetDetailById(context.Background(), result, 99, "view_table")
		assert.NoError(t, err)
	})

	t.Run("with non-struct result (slow path)", func(t *testing.T) {
		db := newStubSQL(t)
		queryOptsUsed := false
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			queryOptsUsed = true
			assert.Contains(t, opts.BaseQuery, "WHERE id = ?")
			return nil
		}
		br := NewBaseRead(db, "test_table")
		var result int
		err := br.GetDetailById(context.Background(), &result, 42)
		assert.NoError(t, err)
		assert.True(t, queryOptsUsed)
	})

	t.Run("with non-struct result and optional table name", func(t *testing.T) {
		db := newStubSQL(t)
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			return nil
		}
		br := NewBaseRead(db, "test_table")
		var result int
		err := br.GetDetailById(context.Background(), &result, 42, "view_table")
		assert.NoError(t, err)
	})

	t.Run("error from db", func(t *testing.T) {
		db := newStubSQL(t)
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			return errors.New("db error")
		}
		br := NewBaseRead(db, "test_table")
		err := br.GetDetailById(context.Background(), &testStruct{}, 42)
		assert.Error(t, err)
	})

	t.Run("with monitoring transaction (struct path)", func(t *testing.T) {
		db := newStubSQL(t)
		db.cachedRebindFn = func(query string) string { return query }
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			return nil
		}
		br := NewBaseRead(db, "test_table")
		ctx := newRelicCtx()
		err := br.GetDetailById(ctx, &testStruct{}, 42)
		assert.NoError(t, err)
	})

	t.Run("with monitoring transaction (non-struct path)", func(t *testing.T) {
		db := newStubSQL(t)
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			return nil
		}
		br := NewBaseRead(db, "test_table")
		ctx := newRelicCtx()
		var result int
		err := br.GetDetailById(ctx, &result, 42)
		assert.NoError(t, err)
	})
}

func TestBaseRead_Count(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db := newStubSQL(t)
		callCount := 0
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			callCount++
			if callCount == 1 {
				assert.Contains(t, opts.BaseQuery, "SELECT COUNT(1)")
				assert.False(t, opts.IsList)
			} else {
				assert.Equal(t, 0, opts.SelectRequest.Limit)
				assert.Equal(t, 0, opts.SelectRequest.Page)
			}
			return nil
		}
		br := NewBaseRead(db, "test_table")
		total, filtered, err := br.Count(context.Background(), &database.TableRequest{})
		assert.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.Equal(t, 0, filtered)
		assert.Equal(t, 2, callCount)
	})

	t.Run("with custom table name", func(t *testing.T) {
		db := newStubSQL(t)
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			return nil
		}
		br := NewBaseRead(db, "test_table")
		_, _, err := br.Count(context.Background(), &database.TableRequest{}, "custom_table")
		assert.NoError(t, err)
	})

	t.Run("first read error", func(t *testing.T) {
		db := newStubSQL(t)
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			return errors.New("first error")
		}
		br := NewBaseRead(db, "test_table")
		_, _, err := br.Count(context.Background(), &database.TableRequest{})
		assert.Error(t, err)
	})

	t.Run("second read error", func(t *testing.T) {
		db := newStubSQL(t)
		callCount := 0
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			callCount++
			if callCount == 1 {
				return nil
			}
			return errors.New("second error")
		}
		br := NewBaseRead(db, "test_table")
		_, _, err := br.Count(context.Background(), &database.TableRequest{})
		assert.Error(t, err)
	})

	t.Run("with monitoring transaction", func(t *testing.T) {
		db := newStubSQL(t)
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			return nil
		}
		br := NewBaseRead(db, "test_table")
		ctx := newRelicCtx()
		_, _, err := br.Count(ctx, &database.TableRequest{})
		assert.NoError(t, err)
	})
}

func TestBaseRead_getBaseQuery(t *testing.T) {
	t.Run("with custom base query", func(t *testing.T) {
		br := NewBaseRead(&stubSQL{t: t}, "test_table")
		opts := &database.QueryOpts{
			BaseQuery: "SELECT 1 FROM test_table",
		}
		q := br.getBaseQuery(context.Background(), opts)
		assert.Equal(t, "SELECT 1 FROM test_table", q)
	})

	t.Run("with optional table name", func(t *testing.T) {
		br := NewBaseRead(&stubSQL{t: t}, "test_table")
		opts := &database.QueryOpts{
			OptionalTableName: "other_table",
		}
		q := br.getBaseQuery(context.Background(), opts)
		assert.Equal(t, "SELECT * FROM other_table", q)
	})

	t.Run("with struct result (cached path)", func(t *testing.T) {
		br := NewBaseRead(&stubSQL{t: t}, "test_table")
		opts := &database.QueryOpts{
			Result: &testStruct{},
		}
		q := br.getBaseQuery(context.Background(), opts)
		assert.Contains(t, q, "SELECT")
		assert.Contains(t, q, "test_table")
	})

	t.Run("with slice of struct result", func(t *testing.T) {
		br := NewBaseRead(&stubSQL{t: t}, "test_table")
		var results []*testStruct
		opts := &database.QueryOpts{
			Result: &results,
		}
		q := br.getBaseQuery(context.Background(), opts)
		assert.Contains(t, q, "SELECT")
		assert.Contains(t, q, "test_table")
	})

	t.Run("with exclude columns", func(t *testing.T) {
		br := NewBaseRead(&stubSQL{t: t}, "test_table")
		opts := &database.QueryOpts{
			Result:         &testStruct{},
			ExcludeColumns: "id",
		}
		q := br.getBaseQuery(context.Background(), opts)
		assert.Contains(t, q, "SELECT")
		assert.NotContains(t, q, "id")
	})

	t.Run("with specified columns", func(t *testing.T) {
		br := NewBaseRead(&stubSQL{t: t}, "test_table")
		opts := &database.QueryOpts{
			Result:  &testStruct{},
			Columns: "id, name",
		}
		q := br.getBaseQuery(context.Background(), opts)
		assert.Contains(t, q, "id, name")
	})

	t.Run("slice of struct (not pointers)", func(t *testing.T) {
		br := NewBaseRead(&stubSQL{t: t}, "test_table")
		var results []testStruct
		opts := &database.QueryOpts{
			Result: &results,
		}
		q := br.getBaseQuery(context.Background(), opts)
		assert.Contains(t, q, "SELECT")
		assert.Contains(t, q, "test_table")
	})

	t.Run("slow path with exclude columns (non-pointer result)", func(t *testing.T) {
		br := NewBaseRead(&stubSQL{t: t}, "test_table")
		opts := &database.QueryOpts{
			Result:         testStruct{},
			ExcludeColumns: "id",
		}
		q := br.getBaseQuery(context.Background(), opts)
		assert.Contains(t, q, "SELECT")
		assert.NotContains(t, q, "id")
	})

	t.Run("slow path with columns only (non-pointer result)", func(t *testing.T) {
		br := NewBaseRead(&stubSQL{t: t}, "test_table")
		opts := &database.QueryOpts{
			Result:  testStruct{},
			Columns: "id, name",
		}
		q := br.getBaseQuery(context.Background(), opts)
		assert.Equal(t, "SELECT id, name FROM test_table", q)
	})
}

// --- Tests for BaseWrite ---

func TestBaseWrite_Insert(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db := newStubSQL(t)
		var capturedOpts *database.QueryOpts
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			capturedOpts = opts
			return &database.CUDResponse{Status: true, RowsAffected: 1, LastInsertID: 1}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		data := &testStruct{Name: "test", Email: "test@test.com", Status: "active"}
		resp, err := bw.Insert(context.Background(), data)
		require.NoError(t, err)
		assert.True(t, resp.Status)
		assert.Equal(t, int64(1), resp.RowsAffected)
		assert.NotNil(t, capturedOpts)
		assert.Equal(t, database.ActionInsert, capturedOpts.CUDRequest.Action)
	})

	t.Run("error from db", func(t *testing.T) {
		db := newStubSQL(t)
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			return nil, errors.New("db error")
		}
		bw := NewBaseWrite(db, "test_table")
		_, err := bw.Insert(context.Background(), &testStruct{Name: "test"})
		assert.Error(t, err)
	})

	t.Run("with transaction", func(t *testing.T) {
		db := newStubSQL(t)
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			return &database.CUDResponse{Status: true}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		_, err := bw.Insert(context.Background(), &testStruct{Name: "test"}, &sqlx.Tx{})
		assert.NoError(t, err)
	})

	t.Run("with monitoring transaction", func(t *testing.T) {
		db := newStubSQL(t)
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			return &database.CUDResponse{Status: true}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		ctx := newRelicCtx()
		_, err := bw.Insert(ctx, &testStruct{Name: "test"})
		assert.NoError(t, err)
	})
}

func TestBaseWrite_BulkInsert(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db := newStubSQL(t)
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			assert.Equal(t, database.ActionBulkInsert, opts.CUDRequest.Action)
			return &database.CUDResponse{Status: true, RowsAffected: 2}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		data := []testStruct{
			{Name: "a", Email: "a@test.com"},
			{Name: "b", Email: "b@test.com"},
		}
		resp, err := bw.BulkInsert(context.Background(), data)
		require.NoError(t, err)
		assert.True(t, resp.Status)
	})

	t.Run("non-slice data", func(t *testing.T) {
		db := newStubSQL(t)
		bw := NewBaseWrite(db, "test_table")
		_, err := bw.BulkInsert(context.Background(), &testStruct{Name: "test"})
		assert.Error(t, err)
	})

	t.Run("with monitoring transaction", func(t *testing.T) {
		db := newStubSQL(t)
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			return &database.CUDResponse{Status: true, RowsAffected: 2}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		ctx := newRelicCtx()
		_, err := bw.BulkInsert(ctx, []testStruct{{Name: "a"}, {Name: "b"}})
		assert.NoError(t, err)
	})

	t.Run("with transaction arg", func(t *testing.T) {
		db := newStubSQL(t)
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			assert.NotNil(t, opts.Trx)
			return &database.CUDResponse{Status: true, RowsAffected: 2}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		_, err := bw.BulkInsert(context.Background(), []testStruct{{Name: "a"}}, &sqlx.Tx{})
		assert.NoError(t, err)
	})
}

func TestBaseWrite_Update(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db := newStubSQL(t)
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			assert.Equal(t, database.ActionUpdate, opts.CUDRequest.Action)
			assert.NotNil(t, opts.SelectRequest)
			return &database.CUDResponse{Status: true, RowsAffected: 1}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		data := &testStruct{Name: "updated_name"}
		condition := map[string]interface{}{"id = ?": 1}
		resp, err := bw.Update(context.Background(), data, condition)
		require.NoError(t, err)
		assert.True(t, resp.Status)
	})

	t.Run("with monitoring transaction", func(t *testing.T) {
		db := newStubSQL(t)
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			return &database.CUDResponse{Status: true, RowsAffected: 1}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		ctx := newRelicCtx()
		_, err := bw.Update(ctx, &testStruct{Name: "test"}, map[string]interface{}{"id = ?": 1})
		assert.NoError(t, err)
	})

	t.Run("with transaction arg", func(t *testing.T) {
		db := newStubSQL(t)
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			assert.NotNil(t, opts.Trx)
			return &database.CUDResponse{Status: true, RowsAffected: 1}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		_, err := bw.Update(context.Background(), &testStruct{Name: "test"}, map[string]interface{}{"id = ?": 1}, &sqlx.Tx{})
		assert.NoError(t, err)
	})
}

func TestBaseWrite_UpdateById(t *testing.T) {
	t.Run("struct data without transaction (goes through fast path + trx path via Write())", func(t *testing.T) {
		db := newStubSQL(t)
		db.cachedRebindFn = func(query string) string {
			return query
		}
		writeCalled := false
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			writeCalled = true
			assert.Equal(t, database.ActionUpdateById, opts.CUDRequest.Action)
			return &database.CUDResponse{Status: true, RowsAffected: 1}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		data := &testStruct{Name: "updated", Email: "e@e.com", Status: "active"}
		resp, err := bw.UpdateById(context.Background(), data, 1)
		require.NoError(t, err)
		assert.True(t, resp.Status)
		assert.True(t, writeCalled)
	})

	t.Run("struct data with transaction", func(t *testing.T) {
		db := newStubSQL(t)
		db.cachedRebindFn = func(query string) string {
			return query
		}
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			assert.NotNil(t, opts.Trx)
			return &database.CUDResponse{Status: true, RowsAffected: 1}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		data := &testStruct{Name: "updated", Email: "e@e.com", Status: "active"}
		resp, err := bw.UpdateById(context.Background(), data, 1, &sqlx.Tx{})
		require.NoError(t, err)
		assert.True(t, resp.Status)
	})

	t.Run("fast path with real *database.SQL (type assertion succeeds)", func(t *testing.T) {
		db := newSQLForAction()
		db.SetSlowQueryThreshold(9999)
		bw := NewBaseWrite(db, "test_table_fast")
		data := &testStruct{Name: "fast_update", Email: "fast@e.com", Status: "done"}
		resp, err := bw.UpdateById(context.Background(), data, 1)
		require.NoError(t, err)
		assert.True(t, resp.Status)
		assert.Equal(t, int64(1), resp.RowsAffected)
	})

	t.Run("error from getWriteStmt", func(t *testing.T) {
		db := newSQLForAction()
		db.SetSlowQueryThreshold(9999)
		// First call succeeds, populate cache
		bw1 := NewBaseWrite(db, "err_table")
		_, _ = bw1.UpdateById(context.Background(), &testStruct{Name: "x", Email: "x@x.com", Status: "a"}, 1)

		bw2 := NewBaseWrite(db, "err_table")
		resp, err := bw2.UpdateById(context.Background(), &testStruct{Name: "y", Email: "y@y.com", Status: "b"}, 1)
		require.NoError(t, err)
		assert.True(t, resp.Status)
	})

	t.Run("with monitoring transaction (slow path)", func(t *testing.T) {
		db := newStubSQL(t)
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			return &database.CUDResponse{Status: true, RowsAffected: 1}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		ctx := newRelicCtx()
		_, err := bw.UpdateById(ctx, &testStruct{Name: "test", Email: "e@e.com", Status: "a"}, 1)
		assert.NoError(t, err)
	})

	t.Run("with monitoring transaction (trx path)", func(t *testing.T) {
		db := newStubSQL(t)
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			return &database.CUDResponse{Status: true, RowsAffected: 1}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		ctx := newRelicCtx()
		_, err := bw.UpdateById(ctx, &testStruct{Name: "test", Email: "e@e.com", Status: "a"}, 1, &sqlx.Tx{})
		assert.NoError(t, err)
	})

	t.Run("trx path write error", func(t *testing.T) {
		db := newStubSQL(t)
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			return nil, errors.New("write error")
		}
		bw := NewBaseWrite(db, "test_table")
		_, err := bw.UpdateById(context.Background(), &testStruct{Name: "test", Email: "e@e.com", Status: "a"}, 1, &sqlx.Tx{})
		assert.Error(t, err)
	})

	t.Run("fast path exec error", func(t *testing.T) {
		db := newSQLForErrorAction()
		db.SetSlowQueryThreshold(9999)
		bw := NewBaseWrite(db, "error_update_test_table")
		data := &testStruct{Name: "test", Email: "e@e.com", Status: "a"}
		_, err := bw.UpdateById(context.Background(), data, 1)
		assert.Error(t, err)
	})
}

func TestBaseWrite_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db := newStubSQL(t)
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			assert.Equal(t, database.ActionDelete, opts.CUDRequest.Action)
			assert.NotNil(t, opts.SelectRequest)
			return &database.CUDResponse{Status: true, RowsAffected: 1}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		condition := map[string]interface{}{"id = ?": 1}
		resp, err := bw.Delete(context.Background(), condition)
		require.NoError(t, err)
		assert.True(t, resp.Status)
	})

	t.Run("with transaction", func(t *testing.T) {
		db := newStubSQL(t)
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			return &database.CUDResponse{Status: true}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		_, err := bw.Delete(context.Background(), map[string]interface{}{"id = ?": 1}, &sqlx.Tx{})
		assert.NoError(t, err)
	})

	t.Run("with monitoring transaction", func(t *testing.T) {
		db := newStubSQL(t)
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			return &database.CUDResponse{Status: true, RowsAffected: 1}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		ctx := newRelicCtx()
		_, err := bw.Delete(ctx, map[string]interface{}{"id = ?": 1})
		assert.NoError(t, err)
	})
}

func TestBaseWrite_DeleteById(t *testing.T) {
	t.Run("slow path (non-SQL db)", func(t *testing.T) {
		deleteByIdCache = sync.Map{}
		db := newStubSQL(t)
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			assert.Equal(t, database.ActionDeleteById, opts.CUDRequest.Action)
			return &database.CUDResponse{Status: true, RowsAffected: 1}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		resp, err := bw.DeleteById(context.Background(), 1)
		require.NoError(t, err)
		assert.True(t, resp.Status)
	})

	t.Run("slow path with transaction", func(t *testing.T) {
		deleteByIdCache = sync.Map{}
		db := newStubSQL(t)
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			assert.NotNil(t, opts.Trx)
			return &database.CUDResponse{Status: true, RowsAffected: 1}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		resp, err := bw.DeleteById(context.Background(), 1, &sqlx.Tx{})
		require.NoError(t, err)
		assert.True(t, resp.Status)
	})

	t.Run("fast path soft delete with real *database.SQL", func(t *testing.T) {
		deleteByIdCache = sync.Map{}
		db := newSQLForAction()
		db.SetSlowQueryThreshold(9999)
		bw := NewBaseWrite(db, "test_table_fast", true)
		resp, err := bw.DeleteById(context.Background(), 1)
		require.NoError(t, err)
		assert.True(t, resp.Status)
	})

	t.Run("fast path hard delete with real *database.SQL", func(t *testing.T) {
		deleteByIdCache = sync.Map{}
		db := newSQLForAction()
		db.SetSlowQueryThreshold(9999)
		bw := NewBaseWrite(db, "test_table_fast", false)
		resp, err := bw.DeleteById(context.Background(), 1)
		require.NoError(t, err)
		assert.True(t, resp.Status)
	})

	t.Run("with monitoring transaction (slow path)", func(t *testing.T) {
		db := newStubSQL(t)
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			return &database.CUDResponse{Status: true, RowsAffected: 1}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		ctx := newRelicCtx()
		_, err := bw.DeleteById(ctx, 42)
		assert.NoError(t, err)
	})

	t.Run("fast path exec error", func(t *testing.T) {
		deleteByIdCache = sync.Map{}
		db := newSQLForErrorAction()
		db.SetSlowQueryThreshold(9999)
		bw := NewBaseWrite(db, "error_delete_test_table", false)
		_, err := bw.DeleteById(context.Background(), 42)
		assert.Error(t, err)
	})
}

func TestBaseWrite_Upsert(t *testing.T) {
	t.Run("existing record found (update path)", func(t *testing.T) {
		db := newStubSQL(t)
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			*(opts.Result.(*int64)) = 1
			return nil
		}
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			assert.Equal(t, database.ActionUpdate, opts.CUDRequest.Action)
			return &database.CUDResponse{Status: true, RowsAffected: 1}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		data := &testStruct{Name: "existing", Email: "e@e.com"}
		condition := map[string]interface{}{"email": "e@e.com"}
		resp, err := bw.Upsert(context.Background(), data, condition)
		require.NoError(t, err)
		assert.True(t, resp.Status)
	})

	t.Run("no existing record (insert path)", func(t *testing.T) {
		db := newStubSQL(t)
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			return nil
		}
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			assert.Equal(t, database.ActionInsert, opts.CUDRequest.Action)
			return &database.CUDResponse{Status: true, RowsAffected: 1, LastInsertID: 5}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		data := &testStruct{Name: "new", Email: "new@e.com"}
		condition := map[string]interface{}{"email": "new@e.com"}
		resp, err := bw.Upsert(context.Background(), data, condition)
		require.NoError(t, err)
		assert.True(t, resp.Status)
	})

	t.Run("nil value in condition", func(t *testing.T) {
		db := newStubSQL(t)
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			return nil
		}
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			return &database.CUDResponse{Status: true}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		data := &testStruct{Name: "test"}
		condition := map[string]interface{}{"email": nil}
		_, err := bw.Upsert(context.Background(), data, condition)
		assert.NoError(t, err)
	})

	t.Run("error during read check", func(t *testing.T) {
		db := newStubSQL(t)
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			return errors.New("read error")
		}
		bw := NewBaseWrite(db, "test_table")
		_, err := bw.Upsert(context.Background(), &testStruct{}, map[string]interface{}{"email": "e@e.com"})
		assert.Error(t, err)
	})

	t.Run("with options (trx, includeDeleted)", func(t *testing.T) {
		db := newStubSQL(t)
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			*(opts.Result.(*int64)) = 1
			return nil
		}
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			return &database.CUDResponse{Status: true}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		_, err := bw.Upsert(context.Background(), &testStruct{Name: "test"}, map[string]interface{}{"email": "test@test.com"}, &sqlx.Tx{}, true)
		assert.NoError(t, err)
	})

	t.Run("with monitoring transaction", func(t *testing.T) {
		db := newStubSQL(t)
		db.readFn = func(ctx context.Context, opts *database.QueryOpts, additionalParams ...interface{}) error {
			*(opts.Result.(*int64)) = 0
			return nil
		}
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			return &database.CUDResponse{Status: true}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		ctx := newRelicCtx()
		_, err := bw.Upsert(ctx, &testStruct{Name: "test"}, map[string]interface{}{"email": "test@test.com"})
		assert.NoError(t, err)
	})
}

func TestBaseWrite_BulkUpdate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db := newStubSQL(t)
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			assert.Equal(t, database.ActionBulkUpdate, opts.CUDRequest.Action)
			return &database.CUDResponse{Status: true, RowsAffected: 2}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		condition := map[string][]interface{}{
			"id": {1, 2},
		}
		resp, err := bw.BulkUpdate(context.Background(), []testStruct{
			{Name: "new1", Email: "e1@e.com"},
			{Name: "new2", Email: "e2@e.com"},
		}, condition)
		require.NoError(t, err)
		assert.True(t, resp.Status)
	})

	t.Run("error from helper", func(t *testing.T) {
		db := newStubSQL(t)
		bw := NewBaseWrite(db, "test_table")
		_, err := bw.BulkUpdate(context.Background(), "invalid data", nil)
		assert.Error(t, err)
	})

	t.Run("write error from db", func(t *testing.T) {
		db := newStubSQL(t)
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			return nil, errors.New("write error")
		}
		bw := NewBaseWrite(db, "test_table")
		data := []testStruct{{Name: "a"}}
		_, err := bw.BulkUpdate(context.Background(), data, map[string][]interface{}{"id": {1}})
		assert.Error(t, err)
	})

	t.Run("with transaction arg", func(t *testing.T) {
		db := newStubSQL(t)
		db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
			assert.NotNil(t, opts.Trx)
			return &database.CUDResponse{Status: true, RowsAffected: 1}, nil
		}
		bw := NewBaseWrite(db, "test_table")
		_, err := bw.BulkUpdate(context.Background(), []testStruct{{Name: "a"}}, map[string][]interface{}{"id": {1}}, &sqlx.Tx{})
		assert.NoError(t, err)
	})
}

// --- Tests for getDeleteByIdQuery ---

func TestGetDeleteByIdQuery(t *testing.T) {
	deleteByIdCache = sync.Map{}

	t.Run("soft delete", func(t *testing.T) {
		db := newStubSQL(t)
		q := getDeleteByIdQuery(db, "test_table", true)
		assert.Contains(t, q, "UPDATE")
		assert.Contains(t, q, "SET deleted_at = now()")
	})

	t.Run("hard delete", func(t *testing.T) {
		db := newStubSQL(t)
		q := getDeleteByIdQuery(db, "test_table", false)
		assert.Contains(t, q, "DELETE FROM")
	})

	t.Run("cached result", func(t *testing.T) {
		deleteByIdCache = sync.Map{}
		db := newStubSQL(t)
		getDeleteByIdQuery(db, "test_table", true)
		_ = getDeleteByIdQuery(db, "test_table", true)
	})
}

// --- Test getBaseQueryCached (internal function) ---

func TestGetBaseQueryCached(t *testing.T) {
	readQueryCache = sync.Map{}

	t.Run("with no exclude/include", func(t *testing.T) {
		q := getBaseQueryCached(reflect.TypeOf(testStruct{}), "test_table", "", "")
		assert.Equal(t, "SELECT * FROM test_table", q)
	})

	t.Run("with exclude cols", func(t *testing.T) {
		readQueryCache = sync.Map{}
		q := getBaseQueryCached(reflect.TypeOf(testStruct{}), "test_table", "id", "")
		assert.Contains(t, q, "SELECT")
		assert.NotContains(t, q, "id")
	})

	t.Run("with include cols", func(t *testing.T) {
		readQueryCache = sync.Map{}
		q := getBaseQueryCached(reflect.TypeOf(testStruct{}), "test_table", "", "name")
		assert.Contains(t, q, "name")
	})
}

// --- Test cudProcess directly ---

func TestCudProcess_UndefinedAction(t *testing.T) {
	bw := &BaseWrite{&baseAction{db: newStubSQL(t), tableName: "test", isSoftDelete: true}}
	_, err := bw.cudProcess(context.Background(), "undefined_action", nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "undefined action")
}

func TestCudProcess_DeleteAction(t *testing.T) {
	db := newStubSQL(t)
	db.writeFn = func(ctx context.Context, opts *database.QueryOpts, isSoftDelete ...bool) (*database.CUDResponse, error) {
		return &database.CUDResponse{Status: true}, nil
	}
	bw := NewBaseWrite(db, "test_table")
	data := &database.CUDConstructData{}
	_, err := bw.cudProcess(context.Background(), database.ActionDelete, data, nil)
	assert.NoError(t, err)
}

func TestCudProcess_InsertNonPointer(t *testing.T) {
	db := newStubSQL(t)
	bw := NewBaseWrite(db, "test_table")
	_, err := bw.cudProcess(context.Background(), database.ActionInsert, "not a struct", nil)
	assert.Error(t, err)
}

// --- Test helper readQueryCache ---

func TestGetSelectByIdQueryCached(t *testing.T) {
	selectByIdCache = sync.Map{}

	t.Run("cached and uncached", func(t *testing.T) {
		db := newStubSQL(t)
		db.cachedRebindFn = func(query string) string {
			return query
		}
		entry1 := getSelectByIdQueryCached(db, reflect.TypeOf(testStruct{}), "test_table")
		require.NotNil(t, entry1)
		assert.Contains(t, entry1.BaseQuery, "SELECT")
		assert.Contains(t, entry1.BaseQuery, "WHERE id = ?")

		entry2 := getSelectByIdQueryCached(db, reflect.TypeOf(testStruct{}), "test_table")
		assert.Equal(t, entry1, entry2)
	})
}




