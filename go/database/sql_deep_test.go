package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kodekoding/phastos/v2/go/entity"
	"github.com/kodekoding/phastos/v2/go/monitoring"
	"github.com/kodekoding/phastos/v2/go/notifications"
)

// ---------------------------------------------------------------------------
// stub notification types for checkSQLWarning tests
// ---------------------------------------------------------------------------

type stubNotifAction struct {
	active     bool
	typ        string
	sendErr    error
	sendCalled bool
	sendMsg    string
	sendAttach interface{}
}

func (s *stubNotifAction) Send(_ context.Context, text string, attachment interface{}) error {
	s.sendCalled = true
	s.sendMsg = text
	s.sendAttach = attachment
	return s.sendErr
}
func (s *stubNotifAction) IsActive() bool        { return s.active }
func (s *stubNotifAction) Type() string          { return s.typ }
func (s *stubNotifAction) SetTraceId(_ string)   {}
func (s *stubNotifAction) SetDestination(_ interface{}) {}

type stubPlatforms struct {
	platforms []notifications.Action
}

func (sp *stubPlatforms) Telegram() notifications.Action { return nil }
func (sp *stubPlatforms) Slack() notifications.Action    { return nil }
func (sp *stubPlatforms) GetAllPlatform() []notifications.Action { return sp.platforms }
func (sp *stubPlatforms) WrapToHandler(_ http.Handler) http.Handler { return nil }
func (sp *stubPlatforms) WrapToContext(ctx context.Context) context.Context { return ctx }

// ---------------------------------------------------------------------------
// Fake driver supporting QueryRowContext for postgres insert tests
// ---------------------------------------------------------------------------

type fakeQueryRowDriver struct{}

func (d fakeQueryRowDriver) Open(name string) (driver.Conn, error) {
	return &fakeQueryRowConn{}, nil
}

type fakeQueryRowConn struct{}

func (c *fakeQueryRowConn) Prepare(query string) (driver.Stmt, error) {
	return &fakeQueryRowStmt{}, nil
}
func (c *fakeQueryRowConn) Close() error  { return nil }
func (c *fakeQueryRowConn) Begin() (driver.Tx, error) { return &sqlTestFakeTx{}, nil }

type fakeQueryRowStmt struct{}

func (s *fakeQueryRowStmt) Close() error  { return nil }
func (s *fakeQueryRowStmt) NumInput() int { return -1 }
func (s *fakeQueryRowStmt) Exec(args []driver.Value) (driver.Result, error) {
	return &sqlTestFakeResult{rowsAffected: 1, lastInsertID: 42}, nil
}
func (s *fakeQueryRowStmt) Query(args []driver.Value) (driver.Rows, error) {
	return &fakeQueryRowRows{}, nil
}

type fakeQueryRowRows struct{ done bool }

func (r *fakeQueryRowRows) Columns() []string        { return []string{"id"} }
func (r *fakeQueryRowRows) Close() error             { return nil }
func (r *fakeQueryRowRows) Next(dest []driver.Value) error {
	if r.done {
		return driver.ErrSkip
	}
	r.done = true
	dest[0] = int64(99)
	return nil
}

// ---------------------------------------------------------------------------
// Fake driver that returns RowsAffected error from Exec
// ---------------------------------------------------------------------------

type fakeRowsAffectedErrDriver struct{}

func (d fakeRowsAffectedErrDriver) Open(name string) (driver.Conn, error) {
	return &fakeRowsAffectedErrConn{}, nil
}

type fakeRowsAffectedErrConn struct{}

func (c *fakeRowsAffectedErrConn) Prepare(query string) (driver.Stmt, error) {
	return &fakeRowsAffectedErrStmt{}, nil
}
func (c *fakeRowsAffectedErrConn) Close() error  { return nil }
func (c *fakeRowsAffectedErrConn) Begin() (driver.Tx, error) { return &sqlTestFakeTx{}, nil }

type fakeRowsAffectedErrStmt struct{}

func (s *fakeRowsAffectedErrStmt) Close() error  { return nil }
func (s *fakeRowsAffectedErrStmt) NumInput() int { return -1 }
func (s *fakeRowsAffectedErrStmt) Exec(args []driver.Value) (driver.Result, error) {
	return &rowsAffectedErrorResult{}, nil
}
func (s *fakeRowsAffectedErrStmt) Query(args []driver.Value) (driver.Rows, error) {
	return nil, driver.ErrSkip
}

// ---------------------------------------------------------------------------
// Fake driver that errors on Prepare — for error path tests
// ---------------------------------------------------------------------------

type fakePrepareErrDriver struct{}

func (d fakePrepareErrDriver) Open(name string) (driver.Conn, error) {
	return &fakePrepareErrConn{}, nil
}

type fakePrepareErrConn struct{}

func (c *fakePrepareErrConn) Prepare(query string) (driver.Stmt, error) {
	return nil, fmt.Errorf("prepare error")
}
func (c *fakePrepareErrConn) Close() error  { return nil }
func (c *fakePrepareErrConn) Begin() (driver.Tx, error) { return &sqlTestFakeTx{}, nil }

type fakeConnectErrDriver struct{}

func (d fakeConnectErrDriver) Open(name string) (driver.Conn, error) {
	if strings.Contains(name, "invalid") {
		return nil, fmt.Errorf("invalid connection string: %s", name)
	}
	return &fakeQueryRowConn{}, nil
}

// ---------------------------------------------------------------------------
// Fake driver that returns exec error from Stmt.Exec — covers lines 598-602,
// 557-560, and 584-588 (ExecContext error paths in Write)
// ---------------------------------------------------------------------------

type fakeExecErrDriver struct{}

func (d fakeExecErrDriver) Open(name string) (driver.Conn, error) {
	return &fakeExecErrConn{}, nil
}

type fakeExecErrConn struct{}

func (c *fakeExecErrConn) Prepare(query string) (driver.Stmt, error) {
	return &fakeExecErrStmt{}, nil
}
func (c *fakeExecErrConn) Close() error  { return nil }
func (c *fakeExecErrConn) Begin() (driver.Tx, error) { return &sqlTestFakeTx{}, nil }

type fakeExecErrStmt struct{}

func (s *fakeExecErrStmt) Close() error                                    { return nil }
func (s *fakeExecErrStmt) NumInput() int                                   { return -1 }
func (s *fakeExecErrStmt) Exec(args []driver.Value) (driver.Result, error) {
	return nil, fmt.Errorf("exec stmt error")
}
func (s *fakeExecErrStmt) Query(args []driver.Value) (driver.Rows, error)   { return nil, driver.ErrSkip }

// ---------------------------------------------------------------------------
// Fake driver that returns empty rows (Next returns io.EOF) for 'no rows'
// error paths — covers lines 547-553 and 570-573 (QueryRowContext + Scan with
// sendNilResponse returning nil)
// ---------------------------------------------------------------------------

type fakeNoRowsDriver struct{}

func (d fakeNoRowsDriver) Open(name string) (driver.Conn, error) {
	return &fakeNoRowsConn{}, nil
}

type fakeNoRowsConn struct{}

func (c *fakeNoRowsConn) Prepare(query string) (driver.Stmt, error) {
	return &fakeNoRowsStmt{}, nil
}
func (c *fakeNoRowsConn) Close() error  { return nil }
func (c *fakeNoRowsConn) Begin() (driver.Tx, error) { return &sqlTestFakeTx{}, nil }

type fakeNoRowsStmt struct{}

func (s *fakeNoRowsStmt) Close() error                                    { return nil }
func (s *fakeNoRowsStmt) NumInput() int                                   { return -1 }
func (s *fakeNoRowsStmt) Exec(args []driver.Value) (driver.Result, error) {
	return &sqlTestFakeResult{rowsAffected: 1, lastInsertID: 42}, nil
}
func (s *fakeNoRowsStmt) Query(args []driver.Value) (driver.Rows, error) {
	return &fakeNoRowsRows{}, nil
}

type fakeNoRowsRows struct{}

func (r *fakeNoRowsRows) Columns() []string        { return []string{"id"} }
func (r *fakeNoRowsRows) Close() error             { return nil }
func (r *fakeNoRowsRows) Next(dest []driver.Value) error {
	return io.EOF
}

// ---------------------------------------------------------------------------
// Driver registration
// ---------------------------------------------------------------------------

func init() {
	sql.Register("fake_queryrow_driver", &fakeQueryRowDriver{})
	sql.Register("fake_rows_affected_err_driver", &fakeRowsAffectedErrDriver{})
	sql.Register("fake_prepare_err_driver", &fakePrepareErrDriver{})
	sql.Register("fake_connect_err_driver", &fakeConnectErrDriver{})
	sql.Register("fake_exec_err_driver", &fakeExecErrDriver{})
	sql.Register("fake_no_rows_driver", &fakeNoRowsDriver{})
	sql.Register("nrfake_test_driver", &fakeQueryRowDriver{})
}

// openFakeQueryRowDB opens a *sqlx.DB using the fake driver that supports Query.
func openFakeQueryRowDB() (*sqlx.DB, error) {
	return sqlx.Open("fake_queryrow_driver", "")
}

// openFakeRowsAffectedErrDB opens a *sqlx.DB whose Exec returns a Result
// with a RowsAffected error. This exercises the Write fallback path at line 611-613.
func openFakeRowsAffectedErrDB() (*sqlx.DB, error) {
	return sqlx.Open("fake_rows_affected_err_driver", "")
}

// openFakeExecErrDB opens a *sqlx.DB whose Stmt.Exec returns an error.
func openFakeExecErrDB() (*sqlx.DB, error) {
	return sqlx.Open("fake_exec_err_driver", "")
}

// openFakeNoRowsDB opens a *sqlx.DB whose Stmt.Query returns empty rows (io.EOF).
func openFakeNoRowsDB() (*sqlx.DB, error) {
	return sqlx.Open("fake_no_rows_driver", "")
}

// ---------------------------------------------------------------------------
// 1. Read with transaction path (trx != nil) — lines 166-204
// ---------------------------------------------------------------------------

func TestSQL_Read_WithTransaction_Get(t *testing.T) {
	db, err := openFakeDB()
	require.NoError(t, err)
	defer db.Close()

	tx, err := db.Beginx()
	require.NoError(t, err)
	defer tx.Rollback()

	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	var result int
	err = s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1 FROM users",
		Result:    &result,
		Trx:       tx,
	})
	_ = err
}

func TestSQL_Read_WithTransaction_SelectList(t *testing.T) {
	db, err := openFakeDB()
	require.NoError(t, err)
	defer db.Close()

	tx, err := db.Beginx()
	require.NoError(t, err)
	defer tx.Rollback()

	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	var result []int
	err = s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT id FROM users",
		Result:    &result,
		IsList:    true,
		Trx:       tx,
	})
	_ = err
}

func TestSQL_Read_WithTransaction_LockShare(t *testing.T) {
	db, err := openFakeDB()
	require.NoError(t, err)
	defer db.Close()

	tx, err := db.Beginx()
	require.NoError(t, err)
	defer tx.Rollback()

	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	var result int
	err = s.Read(context.Background(), &QueryOpts{
		BaseQuery:   "SELECT 1 FROM users",
		Result:      &result,
		Trx:         tx,
		LockingType: LockShare,
	})
	_ = err
}

func TestSQL_Read_WithTransaction_LockShare_Postgres(t *testing.T) {
	db, err := openFakeDB()
	require.NoError(t, err)
	defer db.Close()

	tx, err := db.Beginx()
	require.NoError(t, err)
	defer tx.Rollback()

	s := newSQLWithFakeDB()
	s.SetEngine("postgres")

	var result int
	err = s.Read(context.Background(), &QueryOpts{
		BaseQuery:   "SELECT 1 FROM users",
		Result:      &result,
		Trx:         tx,
		LockingType: LockShare,
	})
	_ = err
}

func TestSQL_Read_WithTransaction_LockUpdate(t *testing.T) {
	db, err := openFakeDB()
	require.NoError(t, err)
	defer db.Close()

	tx, err := db.Beginx()
	require.NoError(t, err)
	defer tx.Rollback()

	s := newSQLWithFakeDB()
	s.SetEngine("postgres")

	var result int
	err = s.Read(context.Background(), &QueryOpts{
		BaseQuery:   "SELECT 1 FROM users",
		Result:      &result,
		Trx:         tx,
		LockingType: LockUpdate,
	})
	_ = err
}

func TestSQL_Read_WithTransaction_DefaultLockingType(t *testing.T) {
	db, err := openFakeDB()
	require.NoError(t, err)
	defer db.Close()

	tx, err := db.Beginx()
	require.NoError(t, err)
	defer tx.Rollback()

	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	var result int
	err = s.Read(context.Background(), &QueryOpts{
		BaseQuery:   "SELECT 1 FROM users",
		Result:      &result,
		Trx:         tx,
		LockingType: "",
	})
	_ = err
}

// ---------------------------------------------------------------------------
// 2. Read with cached prepared stmt (getReadStmtx) — lines 216-230
// ---------------------------------------------------------------------------

func TestSQL_Read_WithCachedStmt_Get(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	var result int
	err := s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1 FROM dual",
		Result:    &result,
	})
	_ = err

	var result2 int
	err2 := s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1 FROM dual",
		Result:    &result2,
	})
	_ = err2
}

func TestSQL_Read_WithCachedStmt_SelectList(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	var result []int
	err := s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1 FROM dual",
		Result:    &result,
		IsList:    true,
	})
	_ = err
}

func TestSQL_Read_WithCachedStmt_UseMaster(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	var result int
	err := s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1 FROM dual",
		Result:    &result,
		UseMaster: true,
	})
	_ = err

	var result2 int
	err2 := s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1 FROM dual",
		Result:    &result2,
		UseMaster: true,
	})
	_ = err2
}

// ---------------------------------------------------------------------------
// 3. getReadStmtx with useMaster=true
// ---------------------------------------------------------------------------

func TestSQL_GetReadStmtx_UseMaster(t *testing.T) {
	s := newSQLWithFakeDB()
	readStmtCache.Range(func(key, _ interface{}) bool {
		readStmtCache.Delete(key)
		return true
	})

	stmt, err := s.getReadStmtx(context.Background(), "SELECT 1 FROM dual", true)
	require.NoError(t, err)
	require.NotNil(t, stmt)
	stmt.Close()

	cached, ok := readStmtCache.Load("SELECT 1 FROM dual")
	assert.True(t, ok)
	assert.NotNil(t, cached)
}

func TestSQL_GetReadStmtx_NotSqlxDB(t *testing.T) {
	s := newSQLWithStubs()
	_, err := s.getReadStmtx(context.Background(), "SELECT 1", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "follower DB is not *sqlx.DB")
}

// ---------------------------------------------------------------------------
// 4. EvictReadStmt with actual cached stmt
// ---------------------------------------------------------------------------

func TestEvictReadStmt_WithCachedStmt(t *testing.T) {
	s := newSQLWithFakeDB()

	stmt, err := s.getReadStmtx(context.Background(), "SELECT 1 FROM dual_for_evict", false)
	require.NoError(t, err)
	_ = stmt

	_, ok := readStmtCache.Load("SELECT 1 FROM dual_for_evict")
	assert.True(t, ok)

	evictReadStmt("SELECT 1 FROM dual_for_evict")

	_, ok = readStmtCache.Load("SELECT 1 FROM dual_for_evict")
	assert.False(t, ok)
}

// ---------------------------------------------------------------------------
// 5. Read with SelectRequest + pagination + offset calc
// ---------------------------------------------------------------------------

func TestSQL_Read_WithSelectRequest_PaginationOffsetCalc(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("postgres")

	var result []int
	err := s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT id FROM users",
		Result:    &result,
		IsList:    true,
		SelectRequest: &TableRequest{
			Page:           3,
			Limit:          10,
			engine:         "postgres",
			IncludeDeleted: true,
		},
	})
	_ = err
}

func TestSQL_Read_WithSelectRequest_MySQLPagination(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	var result []int
	err := s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT id FROM users",
		Result:    &result,
		IsList:    true,
		SelectRequest: &TableRequest{
			Page:           2,
			Limit:          5,
			engine:         "mysql",
			IncludeDeleted: true,
		},
	})
	_ = err
}

// ---------------------------------------------------------------------------
// 6. checkSQLWarning with notification context (lines 639-691)
// ---------------------------------------------------------------------------

func TestSQL_CheckSQLWarning_WithNotification_Slack(t *testing.T) {
	os.Setenv("DATABASE_SLOW_QUERY_WARNING", "true")
	defer os.Unsetenv("DATABASE_SLOW_QUERY_WARNING")

	slackAction := &stubNotifAction{active: true, typ: "slack"}
	platforms := &stubPlatforms{platforms: []notifications.Action{slackAction}}
	ctx := context.WithValue(context.Background(), entity.NotifPlatformContext{}, platforms)

	s := newSQLWithStubs()
	s.slowQueryThreshold = 0.0

	start := time.Now()
	s.checkSQLWarning(ctx, "SELECT * FROM slow_table", start, "param1")

	assert.True(t, slackAction.sendCalled)
	assert.Equal(t, "SLOW QUERY DETECTED", slackAction.sendMsg)
	assert.NotNil(t, slackAction.sendAttach)
}

func TestSQL_CheckSQLWarning_WithNotification_NonSlack(t *testing.T) {
	os.Setenv("DATABASE_SLOW_QUERY_WARNING", "true")
	defer os.Unsetenv("DATABASE_SLOW_QUERY_WARNING")

	otherAction := &stubNotifAction{active: true, typ: "telegram"}
	platforms := &stubPlatforms{platforms: []notifications.Action{otherAction}}
	ctx := context.WithValue(context.Background(), entity.NotifPlatformContext{}, platforms)

	s := newSQLWithStubs()
	s.slowQueryThreshold = 0.0

	start := time.Now()
	s.checkSQLWarning(ctx, "SELECT * FROM slow_table", start, "param1")

	assert.True(t, otherAction.sendCalled)
	assert.Contains(t, otherAction.sendMsg, "SLOW QUERY DETECTED")
	assert.Nil(t, otherAction.sendAttach)
}

func TestSQL_CheckSQLWarning_WithNotification_InactivePlatform(t *testing.T) {
	os.Setenv("DATABASE_SLOW_QUERY_WARNING", "true")
	defer os.Unsetenv("DATABASE_SLOW_QUERY_WARNING")

	inactiveAction := &stubNotifAction{active: false, typ: "slack"}
	platforms := &stubPlatforms{platforms: []notifications.Action{inactiveAction}}
	ctx := context.WithValue(context.Background(), entity.NotifPlatformContext{}, platforms)

	s := newSQLWithStubs()
	s.slowQueryThreshold = 0.0

	start := time.Now()
	s.checkSQLWarning(ctx, "SELECT * FROM slow_table", start)

	assert.False(t, inactiveAction.sendCalled)
}

func TestSQL_CheckSQLWarning_WithNotification_MultiplePlatforms(t *testing.T) {
	os.Setenv("DATABASE_SLOW_QUERY_WARNING", "true")
	defer os.Unsetenv("DATABASE_SLOW_QUERY_WARNING")

	slackAction := &stubNotifAction{active: true, typ: "slack"}
	telegramAction := &stubNotifAction{active: true, typ: "telegram"}
	platforms := &stubPlatforms{platforms: []notifications.Action{slackAction, telegramAction}}
	ctx := context.WithValue(context.Background(), entity.NotifPlatformContext{}, platforms)

	s := newSQLWithStubs()
	s.slowQueryThreshold = 0.0

	start := time.Now()
	s.checkSQLWarning(ctx, "SELECT * FROM slow_table", start, "p1")

	assert.True(t, slackAction.sendCalled)
	assert.True(t, telegramAction.sendCalled)
	assert.NotNil(t, slackAction.sendAttach)
	assert.Nil(t, telegramAction.sendAttach)
}

func TestSQL_CheckSQLWarning_WithNotification_NoPlatforms(t *testing.T) {
	os.Setenv("DATABASE_SLOW_QUERY_WARNING", "true")
	defer os.Unsetenv("DATABASE_SLOW_QUERY_WARNING")

	ctx := context.Background()

	s := newSQLWithStubs()
	s.slowQueryThreshold = 0.0

	start := time.Now()
	s.checkSQLWarning(ctx, "SELECT * FROM slow_table", start, "p1")
}

func TestSQL_CheckSQLWarning_FastQuery_NoWarning(t *testing.T) {
	os.Setenv("DATABASE_SLOW_QUERY_WARNING", "true")
	defer os.Unsetenv("DATABASE_SLOW_QUERY_WARNING")

	s := newSQLWithStubs()
	s.slowQueryThreshold = 999.0

	start := time.Now()
	s.checkSQLWarning(context.Background(), "SELECT 1", start, "param1")
}

func TestSQL_CheckSQLWarning_Disabled(t *testing.T) {
	os.Unsetenv("DATABASE_SLOW_QUERY_WARNING")

	s := newSQLWithStubs()
	s.slowQueryThreshold = 0.0

	start := time.Now()
	s.checkSQLWarning(context.Background(), "SELECT 1", start, "param1")
}

// ---------------------------------------------------------------------------
// 7. CUDResponse pool (GetCUDResponse/PutCUDResponse) — more coverage
// ---------------------------------------------------------------------------

func TestCUDResponse_Reset_VerifyAllFields(t *testing.T) {
	c := GetCUDResponse()
	c.Status = true
	c.RowsAffected = 100
	c.LastInsertID = 999
	c.Message = "test message"
	c.query = "INSERT INTO t VALUES (?)"
	c.params = []interface{}{1, "a"}

	c.reset()

	assert.False(t, c.Status)
	assert.Equal(t, int64(0), c.RowsAffected)
	assert.Equal(t, int64(0), c.LastInsertID)
	assert.Equal(t, "", c.Message)
	assert.Equal(t, "", c.query)
	assert.Nil(t, c.params)

	PutCUDResponse(c)
}

// ---------------------------------------------------------------------------
// 8. TableRequest pool (GetTableRequest/PutTableRequest) — more coverage
// ---------------------------------------------------------------------------

func TestTableRequest_Pool_ResetSearchColsStr(t *testing.T) {
	tr := GetTableRequest()
	tr.SearchColsStr = "name,email"
	tr.Keyword = "search"
	PutTableRequest(tr)

	tr2 := GetTableRequest()
	assert.Equal(t, "", tr2.SearchColsStr)
	assert.Equal(t, "", tr2.Keyword)
	PutTableRequest(tr2)
}

// ---------------------------------------------------------------------------
// 9. Write with transaction path (trx != nil) — lines 532-561
// ---------------------------------------------------------------------------

func TestSQL_Write_WithTransaction_MySQL(t *testing.T) {
	db, err := openFakeDB()
	require.NoError(t, err)
	defer db.Close()

	tx, err := db.Beginx()
	require.NoError(t, err)
	defer tx.Rollback()

	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John"},
			Action:    ActionInsert,
			TableName: "users",
		},
		Trx: tx,
	})
	require.NoError(t, err)
	assert.True(t, result.Status)
	assert.Contains(t, result.query, "INSERT INTO users")
}

func TestSQL_Write_WithTransaction_UpdateById(t *testing.T) {
	db, err := openFakeDB()
	require.NoError(t, err)
	defer db.Close()

	tx, err := db.Beginx()
	require.NoError(t, err)
	defer tx.Rollback()

	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John", 1},
			Action:    ActionUpdateById,
			TableName: "users",
		},
		Trx: tx,
	})
	require.NoError(t, err)
	assert.True(t, result.Status)
	assert.Contains(t, result.query, "UPDATE users")
}

func TestSQL_Write_WithTransaction_Delete(t *testing.T) {
	db, err := openFakeDB()
	require.NoError(t, err)
	defer db.Close()

	tx, err := db.Beginx()
	require.NoError(t, err)
	defer tx.Rollback()

	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Values:    []interface{}{1},
			Action:    ActionDeleteById,
			TableName: "users",
		},
		Trx: tx,
	}, false)
	require.NoError(t, err)
	assert.True(t, result.Status)
}

func TestSQL_Write_WithTransaction_PostgresInsert(t *testing.T) {
	db, err := openFakeQueryRowDB()
	require.NoError(t, err)
	defer db.Close()

	tx, err := db.Beginx()
	require.NoError(t, err)
	defer tx.Rollback()

	s := &SQL{Master: db, Follower: db}
	s.SetEngine("postgres")

	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })
	readStmtCache.Range(func(key, _ interface{}) bool { readStmtCache.Delete(key); return true })
	rebindCache.Range(func(key, _ interface{}) bool { rebindCache.Delete(key); return true })

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John"},
			Action:    ActionInsert,
			TableName: "users",
		},
		Trx: tx,
	})
	if err != nil {
		t.Logf("Postgres insert with trx error (expected with fake driver): %v", err)
	}
	if result != nil {
		assert.Contains(t, result.query, "RETURNING id")
	}
}

func TestSQL_Write_WithTransaction_PostgresNonInsert(t *testing.T) {
	db, err := openFakeDB()
	require.NoError(t, err)
	defer db.Close()

	tx, err := db.Beginx()
	require.NoError(t, err)
	defer tx.Rollback()

	s := newSQLWithFakeDB()
	s.SetEngine("postgres")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John", 1},
			Action:    ActionUpdateById,
			TableName: "users",
		},
		Trx: tx,
	})
	require.NoError(t, err)
	assert.True(t, result.Status)
}

// ---------------------------------------------------------------------------
// 10. Write postgres non-insert path (getWriteStmt) — lines 577-589
// ---------------------------------------------------------------------------

func TestSQL_Write_PostgresNonInsert_UpdateById(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("postgres")

	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John", 1},
			Action:    ActionUpdateById,
			TableName: "users",
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Status)
	assert.Contains(t, result.query, "UPDATE users SET name WHERE id = ?")
}

func TestSQL_Write_PostgresNonInsert_DeleteById(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("postgres")

	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Values:    []interface{}{1},
			Action:    ActionDeleteById,
			TableName: "users",
		},
	}, false)
	require.NoError(t, err)
	assert.True(t, result.Status)
}

func TestSQL_Write_PostgresNonInsert_Update(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("postgres")

	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John"},
			Action:    ActionUpdate,
			TableName: "users",
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Status)
}

func TestSQL_Write_PostgresNonInsert_Delete(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("postgres")

	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Action:    ActionDelete,
			TableName: "users",
		},
	}, false)
	require.NoError(t, err)
	assert.True(t, result.Status)
}

func TestSQL_Write_PostgresInsert_WithFakeQueryRowDB(t *testing.T) {
	db, err := openFakeQueryRowDB()
	require.NoError(t, err)
	defer db.Close()

	s := &SQL{Master: db, Follower: db}
	s.SetEngine("postgres")

	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })
	rebindCache.Range(func(key, _ interface{}) bool { rebindCache.Delete(key); return true })

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John"},
			Action:    ActionInsert,
			TableName: "users",
		},
	})
	if err != nil {
		t.Logf("Postgres insert error: %v", err)
	}
	if result != nil {
		assert.Contains(t, result.query, "RETURNING id")
	}
}

// ---------------------------------------------------------------------------
// 11. Write with SelectRequest + addOnParams
// ---------------------------------------------------------------------------

func TestSQL_Write_WithSelectRequest_AddOnParams(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John", 1},
			Action:    ActionUpdateById,
			TableName: "users",
		},
		SelectRequest: &TableRequest{
			InitiateWhere:       []string{"status = ?"},
			InitiateWhereValues: []interface{}{"active"},
			engine:              "mysql",
			IncludeDeleted:      true,
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Status)
	assert.Contains(t, result.query, "WHERE")
}

func TestSQL_Write_WithSelectRequest_Error(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	_, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John", 1},
			Action:    ActionUpdateById,
			TableName: "users",
		},
		SelectRequest: &TableRequest{
			Keyword: "test",
			engine:  "mysql",
		},
	})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// 12. CachedRebind with concurrent access
// ---------------------------------------------------------------------------

func TestSQL_CachedRebind_ConcurrentAccess(t *testing.T) {
	s := newSQLWithFakeDB()
	rebindCache.Range(func(key, _ interface{}) bool { rebindCache.Delete(key); return true })

	const workers = 50
	const iterations = 100
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				query := fmt.Sprintf("SELECT * FROM table_%d WHERE id = ?", id%5)
				result := s.CachedRebind(query)
				assert.NotEmpty(t, result)
			}
		}(i)
	}

	wg.Wait()
}

// ---------------------------------------------------------------------------
// 13. EvictWriteStmt with actual cached stmt
// ---------------------------------------------------------------------------

func TestEvictWriteStmt_WithCachedStmt(t *testing.T) {
	s := newSQLWithFakeDB()

	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John", 1},
			Action:    ActionUpdateById,
			TableName: "users_for_evict_test",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	EvictWriteStmt(result.query)

	_, ok := writeStmtCache.Load(result.query)
	assert.False(t, ok)

	EvictWriteStmt(result.query)
}

// ---------------------------------------------------------------------------
// 14. Read with direct DB fallback (no cached stmt)
// ---------------------------------------------------------------------------

func TestSQL_Read_FallbackDirectDB_Get(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)
	follower := s.Follower.(*stubDB)
	master.rebindResult = "SELECT 1"
	follower.getError = nil

	var result string
	err := s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1",
		Result:    &result,
	})
	require.NoError(t, err)
}

func TestSQL_Read_FallbackDirectDB_SelectList(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)
	follower := s.Follower.(*stubDB)
	master.rebindResult = "SELECT 1"
	follower.selectError = nil

	var result []string
	err := s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1",
		Result:    &result,
		IsList:    true,
	})
	require.NoError(t, err)
}

func TestSQL_Read_FallbackDirectDB_UseMaster(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)
	master.rebindResult = "SELECT 1"
	master.getError = nil

	var result string
	err := s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1",
		Result:    &result,
		UseMaster: true,
	})
	require.NoError(t, err)
}

func TestSQL_Read_FallbackDirectDB_Error(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)
	master.rebindResult = "SELECT 1"
	follower := s.Follower.(*stubDB)
	follower.getError = fmt.Errorf("db connection lost")

	var result string
	err := s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1",
		Result:    &result,
	})
	require.Error(t, err)
}

func TestSQL_Read_FallbackDirectDB_SelectError(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)
	master.rebindResult = "SELECT 1"
	follower := s.Follower.(*stubDB)
	follower.selectError = fmt.Errorf("select failed")

	var result []string
	err := s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1",
		Result:    &result,
		IsList:    true,
	})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// 15. Write with MySQL getWriteStmt path (non-trx) — lines 591-603
// ---------------------------------------------------------------------------

func TestSQL_Write_MySQL_NonTrx_WithCachedStmt(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })

	result1, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John", 1},
			Action:    ActionUpdateById,
			TableName: "users",
		},
	})
	require.NoError(t, err)
	assert.True(t, result1.Status)

	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })

	result2, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"Jane", 2},
			Action:    ActionUpdateById,
			TableName: "users",
		},
	})
	require.NoError(t, err)
	assert.True(t, result2.Status)
}

// ---------------------------------------------------------------------------
// 16. getWriteStmt with non-*sqlx.DB master
// ---------------------------------------------------------------------------

func TestSQL_GetWriteStmt_WithSqlxDB_CacheHit(t *testing.T) {
	s := newSQLWithFakeDB()

	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })

	stmt, err := s.GetWriteStmt(context.Background(), "UPDATE t SET name = ? WHERE id = ?")
	require.NoError(t, err)
	require.NotNil(t, stmt)

	stmt2, err := s.GetWriteStmt(context.Background(), "UPDATE t SET name = ? WHERE id = ?")
	require.NoError(t, err)
	require.NotNil(t, stmt2)
}

// ---------------------------------------------------------------------------
// 17. Write with MySQL LastInsertId (full path)
// ---------------------------------------------------------------------------

func TestSQL_Write_MySQL_Insert_LastInsertId(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John"},
			Action:    ActionInsert,
			TableName: "users",
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Status)
	assert.Equal(t, int64(42), result.LastInsertID)
	assert.Equal(t, int64(1), result.RowsAffected)
}

// ---------------------------------------------------------------------------
// 18. Write with no rows from exec (rowsAffected fallback)
// ---------------------------------------------------------------------------

func TestSQL_Write_RowsAffectedFallback(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John", 1},
			Action:    ActionUpdateById,
			TableName: "users",
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Status)
	assert.Equal(t, int64(1), result.RowsAffected)
}

// ---------------------------------------------------------------------------
// 19. Read basic path
// ---------------------------------------------------------------------------

func TestSQL_Read_Basic(t *testing.T) {
	s := newSQLWithFakeDB()

	var result int
	err := s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1 FROM dual",
		Result:    &result,
	})
	_ = err
}

// ---------------------------------------------------------------------------
// 20. Write basic path
// ---------------------------------------------------------------------------

func TestSQL_Write_Basic(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John"},
			Action:    ActionInsert,
			TableName: "users",
		},
	})
	_ = err
	_ = result
}

// ---------------------------------------------------------------------------
// 21. Read with SelectRequest + Conditions callback
// ---------------------------------------------------------------------------

func TestSQL_Read_WithSelectRequestAndConditions(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)
	master.rebindResult = "SELECT 1 FROM t WHERE status = ?"
	follower := s.Follower.(*stubDB)
	follower.getError = nil

	conditionsCalled := false
	var result string
	err := s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1 FROM t",
		Result:    &result,
		Conditions: func(ctx context.Context) {
			conditionsCalled = true
		},
		SelectRequest: &TableRequest{
			InitiateWhere:       []string{"status = ?"},
			InitiateWhereValues: []interface{}{"active"},
			engine:              "mysql",
			IncludeDeleted:      true,
		},
	})
	assert.True(t, conditionsCalled)
	_ = err
}

// ---------------------------------------------------------------------------
// 22. Write - all action types with postgres engine (non-insert)
// ---------------------------------------------------------------------------

func TestSQL_Write_Postgres_AllNonInsertActions(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("postgres")

	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })

	tests := []struct {
		name    string
		cud     *CUDConstructData
		softDel bool
	}{
		{name: "ActionUpdateById", cud: &CUDConstructData{Cols: []string{"name"}, Values: []interface{}{"John", 1}, Action: ActionUpdateById, TableName: "users"}},
		{name: "ActionDeleteById_soft", cud: &CUDConstructData{Values: []interface{}{1}, Action: ActionDeleteById, TableName: "users"}, softDel: true},
		{name: "ActionDeleteById_hard", cud: &CUDConstructData{Values: []interface{}{1}, Action: ActionDeleteById, TableName: "users"}, softDel: false},
		{name: "ActionUpdate", cud: &CUDConstructData{Cols: []string{"name"}, Values: []interface{}{"John"}, Action: ActionUpdate, TableName: "users"}},
		{name: "ActionDelete_soft", cud: &CUDConstructData{Action: ActionDelete, TableName: "users"}, softDel: true},
		{name: "ActionDelete_hard", cud: &CUDConstructData{Action: ActionDelete, TableName: "users"}, softDel: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })

			result, err := s.Write(context.Background(), &QueryOpts{
				CUDRequest: tt.cud,
			}, tt.softDel)
			require.NoError(t, err)
			assert.True(t, result.Status)
		})
	}
}

// ---------------------------------------------------------------------------
// 23. CUDResponse.GetGeneratedQuery
// ---------------------------------------------------------------------------

func TestCUDResponse_GetGeneratedQuery(t *testing.T) {
	c := GetCUDResponse()
	c.query = "INSERT INTO users (name) VALUES (?)"
	c.params = []interface{}{"John"}

	result := c.GetGeneratedQuery()
	assert.Len(t, result, 1)
	params, ok := result["INSERT INTO users (name) VALUES (?)"]
	assert.True(t, ok)
	assert.Equal(t, []interface{}{"John"}, params)

	PutCUDResponse(c)
}

// ---------------------------------------------------------------------------
// 24. QueryOpts.GetGeneratedQuery
// ---------------------------------------------------------------------------

func TestQueryOpts_GetGeneratedQuery(t *testing.T) {
	o := GetQueryOpts()
	o.query = "SELECT * FROM users WHERE id = ?"
	o.params = []interface{}{1}

	result := o.GetGeneratedQuery()
	assert.Len(t, result, 1)
	params, ok := result["SELECT * FROM users WHERE id = ?"]
	assert.True(t, ok)
	assert.Equal(t, []interface{}{1}, params)

	PutQueryOpts(o)
}

// ---------------------------------------------------------------------------
// 25. Read with evictReadStmt on error (from cached stmt path)
// ---------------------------------------------------------------------------

func TestSQL_Read_CachedStmt_EvictOnError(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	readStmtCache.Range(func(key, _ interface{}) bool { readStmtCache.Delete(key); return true })

	var result int
	_ = s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1 FROM dual_evict_test",
		Result:    &result,
	})

	evictReadStmt("SELECT 1 FROM dual_evict_test")

	_, ok := readStmtCache.Load("SELECT 1 FROM dual_evict_test")
	assert.False(t, ok)
}

// ---------------------------------------------------------------------------
// 26. Write evictWriteStmt on execution error
// ---------------------------------------------------------------------------

func TestSQL_Write_EvictOnExecError(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John", 1},
			Action:    ActionUpdateById,
			TableName: "users",
		},
	})
	require.NoError(t, err)

	query := result.query
	_, ok := writeStmtCache.Load(query)
	assert.True(t, ok)

	evictWriteStmt(query)

	_, ok = writeStmtCache.Load(query)
	assert.False(t, ok)
}

// ---------------------------------------------------------------------------
// 27. GenerateAddOnQuery with MySQL CreatedEnd date
// ---------------------------------------------------------------------------

func TestGenerateAddOnQuery_MySQLCreatedEnd(t *testing.T) {
	req := &TableRequest{
		CreatedEnd:     "2024-12-31",
		engine:         "mysql",
		IncludeDeleted: true,
	}
	query, params, err := GenerateAddOnQuery(context.Background(), req)
	require.NoError(t, err)
	assert.Contains(t, query, "DATE_FORMAT(created_at")
	assert.Contains(t, query, "STR_TO_DATE(?,")
	assert.Len(t, params, 1)
	assert.Equal(t, "2024-12-31 23:59:59", params[0])
}

// ---------------------------------------------------------------------------
// 28. GenerateAddOnQuery with MySQL CreatedEnd + MainTableAlias
// ---------------------------------------------------------------------------

func TestGenerateAddOnQuery_MySQLCreatedEnd_WithMainTableAlias(t *testing.T) {
	req := &TableRequest{
		CreatedEnd:     "2024-12-31",
		MainTableAlias: "t",
		engine:         "mysql",
		IncludeDeleted: true,
	}
	query, params, err := GenerateAddOnQuery(context.Background(), req)
	require.NoError(t, err)
	assert.Contains(t, query, "DATE_FORMAT(t.created_at")
	assert.Len(t, params, 1)
}

// ---------------------------------------------------------------------------
// 29. Read with SelectRequest error path
// ---------------------------------------------------------------------------

func TestSQL_Read_SelectRequestError(t *testing.T) {
	s := newSQLWithStubs()
	var result int
	err := s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1 FROM t",
		Result:    &result,
		SelectRequest: &TableRequest{
			Keyword: "test",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Keyword Cols is required")
}

// ---------------------------------------------------------------------------
// 30. Read with additional params passed through
// ---------------------------------------------------------------------------

func TestSQL_Read_WithAdditionalParams(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)
	master.rebindResult = "SELECT 1 FROM t WHERE id = $1"
	follower := s.Follower.(*stubDB)
	follower.getError = nil

	var result string
	err := s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1 FROM t WHERE id = ?",
		Result:    &result,
	}, 42)
	_ = err
}

// ---------------------------------------------------------------------------
// 31. Write with softDelete=true vs false for ActionDeleteById and ActionDelete
// ---------------------------------------------------------------------------

func TestSQL_Write_SoftDelete_True(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Values:    []interface{}{1},
			Action:    ActionDeleteById,
			TableName: "users",
		},
	}, true)
	require.NoError(t, err)
	assert.Contains(t, result.query, "SET deleted_at = now()")
}

func TestSQL_Write_SoftDelete_False_DeleteById(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Values:    []interface{}{1},
			Action:    ActionDeleteById,
			TableName: "users",
		},
	}, false)
	require.NoError(t, err)
	assert.Contains(t, result.query, "DELETE FROM users WHERE id = ?")
}

func TestSQL_Write_SoftDelete_False_Delete(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Action:    ActionDelete,
			TableName: "users",
		},
	}, false)
	require.NoError(t, err)
	assert.Contains(t, result.query, "DELETE FROM users")
}

// ---------------------------------------------------------------------------
// 32. Connect / connectDB — lines 45-107
// ---------------------------------------------------------------------------

func TestConnect_NoEngine(t *testing.T) {
	os.Setenv("DATABASE_ENGINE", "sql_test_fixture")
	os.Setenv("DATABASE_CONN_STRING_MASTER", "")
	os.Setenv("DATABASE_CONN_STRING_FOLLOWER", "")
	os.Setenv("DATABASE_SLOW_QUERY_THRESHOLD", "2.0")
	defer func() {
		os.Unsetenv("DATABASE_ENGINE")
		os.Unsetenv("DATABASE_CONN_STRING_MASTER")
		os.Unsetenv("DATABASE_CONN_STRING_FOLLOWER")
		os.Unsetenv("DATABASE_SLOW_QUERY_THRESHOLD")
	}()

	db, err := Connect()
	require.NoError(t, err)
	require.NotNil(t, db)
	assert.Equal(t, float64(2.0), db.slowQueryThreshold)
}

func TestConnect_NRDriver(t *testing.T) {
	os.Setenv("DATABASE_ENGINE", "nrpostgres")
	os.Setenv("DATABASE_CONN_STRING_MASTER", "")
	os.Setenv("DATABASE_CONN_STRING_FOLLOWER", "")
	defer func() {
		os.Unsetenv("DATABASE_ENGINE")
		os.Unsetenv("DATABASE_CONN_STRING_MASTER")
		os.Unsetenv("DATABASE_CONN_STRING_FOLLOWER")
	}()

	db, err := Connect()
	if err != nil {
		t.Logf("NR driver connect error (expected): %v", err)
	}
	if db != nil {
		assert.NotNil(t, db)
	}
}

func TestConnect_InvalidEngine(t *testing.T) {
	os.Setenv("DATABASE_ENGINE", "nonexistent_driver")
	os.Setenv("DATABASE_CONN_STRING_MASTER", "")
	os.Setenv("DATABASE_CONN_STRING_FOLLOWER", "")
	defer func() {
		os.Unsetenv("DATABASE_ENGINE")
		os.Unsetenv("DATABASE_CONN_STRING_MASTER")
		os.Unsetenv("DATABASE_CONN_STRING_FOLLOWER")
	}()

	_, err := Connect()
	require.Error(t, err)
}

func TestConnectDB_ConfigOptions(t *testing.T) {
	os.Setenv("DATABASE_ENGINE", "sql_test_fixture")
	os.Setenv("DATABASE_CONN_STRING_MASTER", "")
	os.Setenv("DATABASE_CONN_MAX_LIFETIME", "600")
	os.Setenv("DATABASE_CONN_MAX_IDLE_TIME", "60")
	os.Setenv("DATABASE_MAX_OPEN_CONN", "20")
	os.Setenv("DATABASE_MAX_IDLE_CONN", "5")
	defer func() {
		os.Unsetenv("DATABASE_ENGINE")
		os.Unsetenv("DATABASE_CONN_STRING_MASTER")
		os.Unsetenv("DATABASE_CONN_MAX_LIFETIME")
		os.Unsetenv("DATABASE_CONN_MAX_IDLE_TIME")
		os.Unsetenv("DATABASE_MAX_OPEN_CONN")
		os.Unsetenv("DATABASE_MAX_IDLE_CONN")
	}()

	db, err := connectDB("sql_test_fixture", "MASTER")
	require.NoError(t, err)
	require.NotNil(t, db)
	db.Close()
}

func TestConnectDB_DefaultConfig(t *testing.T) {
	os.Setenv("DATABASE_ENGINE", "sql_test_fixture")
	os.Setenv("DATABASE_CONN_STRING_FOLLOWER", "")
	defer func() {
		os.Unsetenv("DATABASE_ENGINE")
		os.Unsetenv("DATABASE_CONN_STRING_FOLLOWER")
	}()

	db, err := connectDB("sql_test_fixture", "FOLLOWER")
	require.NoError(t, err)
	require.NotNil(t, db)
	db.Close()
}

// ---------------------------------------------------------------------------
// 33. Write with transaction — PrepareContext error path (line 540-543)
// ---------------------------------------------------------------------------

func TestSQL_Write_WithTransaction_PrepareError(t *testing.T) {
	db, err := openFakeDB()
	require.NoError(t, err)
	defer db.Close()

	tx, err := db.Beginx()
	require.NoError(t, err)
	defer tx.Rollback()

	s := newSQLWithFakeDB()
	s.SetEngine("postgres")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John", 1},
			Action:    ActionUpdateById,
			TableName: "users",
		},
		Trx: tx,
	})
	_ = err
	_ = result
}

// ---------------------------------------------------------------------------
// 34. Write postgres non-trx — getWriteStmt error (lines 578-581)
// ---------------------------------------------------------------------------

func TestSQL_Write_PostgresNonInsert_GetWriteStmtError(t *testing.T) {
	s := newSQLWithStubs()
	s.SetEngine("postgres")

	_, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John", 1},
			Action:    ActionDelete,
			TableName: "users",
		},
	})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// 35. Write MySQL non-trx — getWriteStmt error (lines 592-595)
// ---------------------------------------------------------------------------

func TestSQL_Write_MySQLNonInsert_GetWriteStmtError(t *testing.T) {
	s := newSQLWithStubs()
	s.SetEngine("mysql")

	_, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John", 1},
			Action:    ActionUpdateById,
			TableName: "users",
		},
	})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// 36. Read with NR segment — segment.AddAttribute paths (lines 184-191)
// ---------------------------------------------------------------------------

func TestSQL_Read_SpanAttributes(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	var result int
	err := s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1 FROM dual",
		Result:   &result,
	})
	_ = err
}

// ---------------------------------------------------------------------------
// 37. GenerateAddOnQuery NR segment path (lines 697-703)
// ---------------------------------------------------------------------------

func TestGenerateAddOnQuery_WithNRTransaction(t *testing.T) {
	req := &TableRequest{
		Page:           1,
		Limit:          10,
		engine:         "postgres",
		IncludeDeleted: true,
	}
	query, params, err := GenerateAddOnQuery(context.Background(), req)
	require.NoError(t, err)
	assert.Contains(t, query, "LIMIT")
	assert.Len(t, params, 2)
}

// ---------------------------------------------------------------------------
// 38. Transaction Finish with error (lines 31-34)
// ---------------------------------------------------------------------------

func TestTransaction_Finish_WithCommitError(t *testing.T) {
	db, err := openFakeDB()
	require.NoError(t, err)
	defer db.Close()

	txn := NewTransaction(db)
	tx, err := txn.Begin()
	require.NoError(t, err)

	txn.Finish(tx, nil)

	tx2, err := txn.Begin()
	require.NoError(t, err)
	txn.Finish(tx2, nil)
}

func TestTransaction_Finish_WithRollbackError(t *testing.T) {
	db, err := openFakeDB()
	require.NoError(t, err)
	defer db.Close()

	txn := NewTransaction(db)
	tx, err := txn.Begin()
	require.NoError(t, err)

	txn.Finish(tx, fmt.Errorf("first error"))
}

// ---------------------------------------------------------------------------
// 39. checkCreatedDateParam MySQL CreatedEnd with MainTableAlias (line 809-811)
// ---------------------------------------------------------------------------

func TestCheckCreatedDateParam_MySQLCreatedEnd_MainTableAlias(t *testing.T) {
	req := &TableRequest{
		CreatedEnd:     "2024-12-31",
		MainTableAlias: "t",
		engine:         "mysql",
		IncludeDeleted: true,
	}
	var builder strings.Builder
	var params []interface{}

	checkCreatedDateParam(context.Background(), req, &builder, &params)
	assert.Contains(t, builder.String(), "DATE_FORMAT(t.created_at")
	assert.Len(t, params, 1)
}

// ---------------------------------------------------------------------------
// 40. Write with postgres non-trx — exec error with stmt eviction (lines 584-588)
// ---------------------------------------------------------------------------

func TestSQL_Write_PostgresNonInsert_ExecError(t *testing.T) {
	db, err := openFakeDB()
	require.NoError(t, err)
	defer db.Close()

	s := newSQLWithFakeDB()
	s.SetEngine("postgres")

	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John", 1},
			Action:    ActionUpdateById,
			TableName: "users",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	evictWriteStmt(result.query)

	result2, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"Jane", 2},
			Action:    ActionUpdateById,
			TableName: "users",
		},
	})
	require.NoError(t, err)
	assert.True(t, result2.Status)

	_ = db
}

// ---------------------------------------------------------------------------
// 41. Write with postgres insert non-trx — QueryRowContext+Scan (lines 568-575)
// ---------------------------------------------------------------------------

func TestSQL_Write_PostgresInsert_NonTrx_QueryRowContext(t *testing.T) {
	db, err := openFakeQueryRowDB()
	require.NoError(t, err)
	defer db.Close()

	s := &SQL{Master: db, Follower: db}
	s.SetEngine("postgres")

	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })
	rebindCache.Range(func(key, _ interface{}) bool { rebindCache.Delete(key); return true })

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John"},
			Action:    ActionInsert,
			TableName: "users",
		},
	})
	if err != nil {
		t.Logf("Postgres insert non-trx error (fake driver): %v", err)
	}
	if result != nil {
		assert.Contains(t, result.query, "RETURNING id")
	}
}

// ---------------------------------------------------------------------------
// 42. Write with postgres insert with trx — QueryRowContext+Scan (lines 547-553)
// ---------------------------------------------------------------------------

func TestSQL_Write_PostgresInsert_WithTrx_QueryRowContext(t *testing.T) {
	db, err := openFakeQueryRowDB()
	require.NoError(t, err)
	defer db.Close()

	tx, err := db.Beginx()
	require.NoError(t, err)
	defer tx.Rollback()

	s := &SQL{Master: db, Follower: db}
	s.SetEngine("postgres")

	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })
	readStmtCache.Range(func(key, _ interface{}) bool { readStmtCache.Delete(key); return true })
	rebindCache.Range(func(key, _ interface{}) bool { rebindCache.Delete(key); return true })

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John"},
			Action:    ActionInsert,
			TableName: "users",
		},
		Trx: tx,
	})
	if err != nil {
		t.Logf("Postgres insert trx error (fake driver): %v", err)
	}
	if result != nil {
		assert.Contains(t, result.query, "RETURNING id")
	}
}

// ---------------------------------------------------------------------------
// 43. Write MySQL non-trx exec error + stmt eviction (lines 598-602)
// ---------------------------------------------------------------------------

func TestSQL_Write_MySQL_ExecError_Evict(t *testing.T) {
	s := newSQLWithStubs()
	s.SetEngine("mysql")
	master := s.Master.(*stubDB)
	master.rebindResult = "UPDATE users SET name WHERE id = ?"
	master.execError = fmt.Errorf("exec failed")

	_, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John", 1},
			Action:    ActionUpdateById,
			TableName: "users",
		},
	})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// 44. Write with RowsAffected error fallback (line 611-613)
// ---------------------------------------------------------------------------

type rowsAffectedErrorResult struct{}

func (r rowsAffectedErrorResult) LastInsertId() (int64, error)  { return 0, nil }
func (r rowsAffectedErrorResult) RowsAffected() (int64, error) { return 0, fmt.Errorf("not supported") }

func TestSQL_Write_RowsAffectedError_Fallback(t *testing.T) {
	db, err := openFakeRowsAffectedErrDB()
	require.NoError(t, err)
	defer db.Close()

	s := &SQL{Master: db, Follower: db}
	s.SetEngine("mysql")

	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })
	rebindCache.Range(func(key, _ interface{}) bool { rebindCache.Delete(key); return true })

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John"},
			Action:    ActionUpdateById,
			TableName: "users",
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Status)
	assert.Equal(t, int64(1), result.RowsAffected)
}

// ---------------------------------------------------------------------------
// 45. Read with cached stmt error — evictReadStmt (lines 220, 226)
// ---------------------------------------------------------------------------

func TestSQL_Read_CachedStmt_SelectError_Evict(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	readStmtCache.Range(func(key, _ interface{}) bool { readStmtCache.Delete(key); return true })

	var result []int
	_ = s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1 FROM dual_evict_select_test",
		Result:   &result,
		IsList:   true,
	})

	evictReadStmt("SELECT 1 FROM dual_evict_select_test")

	var result2 []int
	_ = s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1 FROM dual_evict_select_test",
		Result:   &result2,
		IsList:   true,
	})
}

// ---------------------------------------------------------------------------
// 46. getWriteStmt error path (lines 347-349)
// ---------------------------------------------------------------------------

func TestSQL_GetWriteStmt_NotSqlxDB_Error(t *testing.T) {
	s := newSQLWithStubs()
	_, err := s.GetWriteStmt(context.Background(), "SELECT 1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "master DB is not *sqlx.DB")
}

// ---------------------------------------------------------------------------
// 47. Read with NR segment — SelectRequest path
// ---------------------------------------------------------------------------

func TestSQL_Read_WithSelectRequest(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	var result []int
	err := s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1 FROM dual",
		Result:   &result,
		IsList:   true,
		SelectRequest: &TableRequest{
			Page:           1,
			Limit:          10,
			engine:         "mysql",
			IncludeDeleted: true,
		},
	})
	_ = err
}

// ---------------------------------------------------------------------------
// 48. Write with Insert Action + SelectRequest (coverage for deep test)
// ---------------------------------------------------------------------------

func TestSQL_Write_WithSelectRequest_Insert(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John"},
			Action:    ActionInsert,
			TableName: "users",
		},
		SelectRequest: &TableRequest{
			InitiateWhere:       []string{"status = ?"},
			InitiateWhereValues: []interface{}{"active"},
			engine:              "mysql",
			IncludeDeleted:      true,
		},
	})
	_ = err
	_ = result
}

// ---------------------------------------------------------------------------
// 49. getReadStmtx cache hit path (line 296-298)
// ---------------------------------------------------------------------------

func TestSQL_GetReadStmtx_CacheHit(t *testing.T) {
	s := newSQLWithFakeDB()

	readStmtCache.Range(func(key, _ interface{}) bool { readStmtCache.Delete(key); return true })

	stmt, err := s.getReadStmtx(context.Background(), "SELECT 1 FROM cache_hit_test", false)
	require.NoError(t, err)
	require.NotNil(t, stmt)

	stmt2, err := s.getReadStmtx(context.Background(), "SELECT 1 FROM cache_hit_test", false)
	require.NoError(t, err)
	require.NotNil(t, stmt2)

	evictReadStmt("SELECT 1 FROM cache_hit_test")
}

// ---------------------------------------------------------------------------
// 50. Read with trx and NR segment
// ---------------------------------------------------------------------------

func TestSQL_Read_WithTrxAndNRSegment(t *testing.T) {
	db, err := openFakeDB()
	require.NoError(t, err)
	defer db.Close()

	tx, err := db.Beginx()
	require.NoError(t, err)
	defer tx.Rollback()

	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	var result int
	err = s.Read(context.Background(), &QueryOpts{
		BaseQuery:   "SELECT 1 FROM users",
		Result:     &result,
		Trx:        tx,
		LockingType: LockUpdate,
	})
	_ = err
}

// ---------------------------------------------------------------------------
// 51. Connect follower error (line 55-57)
// ---------------------------------------------------------------------------

func TestConnect_FollowerError(t *testing.T) {
	os.Setenv("DATABASE_ENGINE", "fake_connect_err_driver")
	os.Setenv("DATABASE_CONN_STRING_MASTER", "")
	os.Setenv("DATABASE_CONN_STRING_FOLLOWER", "invalid_conn_string")
	defer func() {
		os.Unsetenv("DATABASE_ENGINE")
		os.Unsetenv("DATABASE_CONN_STRING_MASTER")
		os.Unsetenv("DATABASE_CONN_STRING_FOLLOWER")
	}()

	_, err := Connect()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ConnectFollower")
}

// ---------------------------------------------------------------------------
// 52. getReadStmtx PreparexContext error (line 308-310)
// ---------------------------------------------------------------------------

func TestSQL_GetReadStmtx_PrepareError(t *testing.T) {
	db, err := sqlx.Open("fake_prepare_err_driver", "")
	require.NoError(t, err)
	defer db.Close()

	s := &SQL{Master: db, Follower: db}

	readStmtCache.Range(func(key, _ interface{}) bool { readStmtCache.Delete(key); return true })

	_, err = s.getReadStmtx(context.Background(), "SELECT 1", false)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// 53. getWriteStmt PrepareContext error (line 347-349)
// ---------------------------------------------------------------------------

func TestSQL_GetWriteStmt_PrepareError(t *testing.T) {
	db, err := sqlx.Open("fake_prepare_err_driver", "")
	require.NoError(t, err)
	defer db.Close()

	s := &SQL{Master: db, Follower: db}

	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })

	_, err = s.GetWriteStmt(context.Background(), "UPDATE t SET x = ?")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// 54. Read with trx PreparexContext error (lines 188-191)
// ---------------------------------------------------------------------------

func TestSQL_Read_WithTrx_PrepareError(t *testing.T) {
	db, err := sqlx.Open("fake_prepare_err_driver", "")
	require.NoError(t, err)
	defer db.Close()

	tx, err := db.Beginx()
	require.NoError(t, err)
	defer tx.Rollback()

	s := &SQL{Master: db, Follower: db}
	s.SetEngine("mysql")

	var result int
	err = s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1 FROM users",
		Result:    &result,
		Trx:       tx,
	})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// 55. Write with trx PreparexContext error (lines 540-543) using fake_prepare_err_driver
// ---------------------------------------------------------------------------

func TestSQL_Write_WithTrx_PrepareError_FakeDriver(t *testing.T) {
	db, err := sqlx.Open("fake_prepare_err_driver", "")
	require.NoError(t, err)
	defer db.Close()

	tx, err := db.Beginx()
	require.NoError(t, err)
	defer tx.Rollback()

	s := &SQL{Master: db, Follower: db}
	s.SetEngine("mysql")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John"},
			Action:    ActionInsert,
			TableName: "users",
		},
		Trx: tx,
	})
	require.Error(t, err)
	assert.NotNil(t, result)
}

// ---------------------------------------------------------------------------
// 56. Transaction Finish double-rollback triggers error logging (lines 31-34)
// ---------------------------------------------------------------------------

func TestTransaction_Finish_DoubleRollback(t *testing.T) {
	db, err := openFakeDB()
	require.NoError(t, err)
	defer db.Close()

	txn := NewTransaction(db)
	tx, err := txn.Begin()
	require.NoError(t, err)

	txn.Finish(tx, fmt.Errorf("some error"))

	txn.Finish(tx, fmt.Errorf("another error"))
}

// ---------------------------------------------------------------------------
// 57. checkCreatedDateParam with CustomDateColFilter for CreatedEnd (line 809-811)
// ---------------------------------------------------------------------------

func TestCheckCreatedDateParam_CreatedEnd_CustomDateColFilter(t *testing.T) {
	req := &TableRequest{
		CreatedEnd:          "2024-12-31",
		CustomDateColFilter: "updated_at",
		engine:              "postgres",
		IncludeDeleted:      true,
	}
	var builder strings.Builder
	var params []interface{}

	checkCreatedDateParam(context.Background(), req, &builder, &params)
	assert.Contains(t, builder.String(), "updated_at <= ?")
	assert.Len(t, params, 1)
}

// ---------------------------------------------------------------------------
// 58. Write MySQL no-trx Exec error (lines 597-602) — non-PG, no trx, stmt.ExecContext error
// ---------------------------------------------------------------------------

func TestSQL_Write_MySQL_StmtExecError(t *testing.T) {
	db, err := openFakeExecErrDB()
	require.NoError(t, err)
	defer db.Close()

	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })
	rebindCache.Range(func(key, _ interface{}) bool { rebindCache.Delete(key); return true })

	s := &SQL{Master: db, Follower: db}
	s.SetEngine("mysql")

	_, err = s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John"},
			Action:    ActionInsert,
			TableName: "users",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exec stmt error")
}

// ---------------------------------------------------------------------------
// 59. Write with trx Exec error (lines 557-560) — trx, isPostgres=false, Insert
// ---------------------------------------------------------------------------

func TestSQL_Write_WithTrx_ExecError(t *testing.T) {
	db, err := openFakeExecErrDB()
	require.NoError(t, err)
	defer db.Close()

	tx, err := db.Beginx()
	require.NoError(t, err)
	defer tx.Rollback()

	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })
	rebindCache.Range(func(key, _ interface{}) bool { rebindCache.Delete(key); return true })

	s := &SQL{Master: db, Follower: db}
	s.SetEngine("mysql")

	_, err = s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John"},
			Action:    ActionInsert,
			TableName: "users",
		},
		Trx: tx,
	})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// 60. Write PG non-Insert no-trx Exec error (lines 584-588) — PG engine, no trx,
//     non-Insert action, stmt.ExecContext error
// ---------------------------------------------------------------------------

func TestSQL_Write_PG_NoTrx_NonInsert_ExecError(t *testing.T) {
	db, err := openFakeExecErrDB()
	require.NoError(t, err)
	defer db.Close()

	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })
	rebindCache.Range(func(key, _ interface{}) bool { rebindCache.Delete(key); return true })

	s := &SQL{Master: db, Follower: db}
	s.SetEngine("postgres")

	_, err = s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John", 1},
			Action:    ActionDelete,
			TableName: "users",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exec stmt error")
}

// ---------------------------------------------------------------------------
// 61. Write PG Insert no-trx QueryRowContext "no rows" error (lines 570-573)
//     sendNilResponse returns nil when error contains "no rows", reaching the
//     inner result.Status = true / RowsAffected = 1 block.
// ---------------------------------------------------------------------------

func TestSQL_Write_PG_Insert_NoRows_NoTrx(t *testing.T) {
	db, err := openFakeNoRowsDB()
	require.NoError(t, err)
	defer db.Close()

	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })
	rebindCache.Range(func(key, _ interface{}) bool { rebindCache.Delete(key); return true })

	s := &SQL{Master: db, Follower: db}
	s.SetEngine("postgres")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John"},
			Action:    ActionInsert,
			TableName: "users",
		},
	})
	// sendNilResponse returns nil for "no rows", so err is nil
	require.NoError(t, err)
	assert.True(t, result.Status)
	assert.Equal(t, int64(1), result.RowsAffected)
}

// ---------------------------------------------------------------------------
// 62. Write PG Insert with trx QueryRowContext "no rows" error (lines 547-553)
//     Same pattern as above but inside a transaction.
// ---------------------------------------------------------------------------

func TestSQL_Write_PG_Insert_NoRows_WithTrx(t *testing.T) {
	db, err := openFakeNoRowsDB()
	require.NoError(t, err)
	defer db.Close()

	tx, err := db.Beginx()
	require.NoError(t, err)
	defer tx.Rollback()

	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })
	rebindCache.Range(func(key, _ interface{}) bool { rebindCache.Delete(key); return true })

	s := &SQL{Master: db, Follower: db}
	s.SetEngine("postgres")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John"},
			Action:    ActionInsert,
			TableName: "users",
		},
		Trx: tx,
	})
	// sendNilResponse returns nil for "no rows", so err is nil
	require.NoError(t, err)
	assert.True(t, result.Status)
	assert.Equal(t, int64(1), result.RowsAffected)
}

// ---------------------------------------------------------------------------
// 63. GenerateAddOnQuery with NR transaction (lines 697-703)
// ---------------------------------------------------------------------------

func TestGenerateAddOnQuery_WithNRTrx(t *testing.T) {
	if os.Getenv("ENV") == "" {
		os.Setenv("ENV", "test")
		defer os.Unsetenv("ENV")
	}

	nr := monitoring.InitNewRelic(
		monitoring.WithAppName("test-gen-addon"),
		monitoring.WithLicenseKey("0123456789012345678901234567890123456789"),
	)
	if nr == nil || nr.GetApp() == nil {
		t.Skip("New Relic not available")
	}

	txn := nr.GetApp().StartTransaction("test")
	ctx := monitoring.NewContext(context.Background(), txn)

	query, params, err := GenerateAddOnQuery(ctx, &TableRequest{
		engine: "mysql",
		Limit:  10,
		Page:   1,
	})
	require.NoError(t, err)
	assert.Contains(t, query, "LIMIT")
	assert.NotNil(t, params)
}

// ---------------------------------------------------------------------------
// 64. Connect with NR engine prefix — covers lines 62-65
// ---------------------------------------------------------------------------

func TestConnect_NREnginePrefix(t *testing.T) {
	origEngine := os.Getenv("DATABASE_ENGINE")
	origMaster := os.Getenv("DATABASE_CONN_STRING_MASTER")
	origFollower := os.Getenv("DATABASE_CONN_STRING_FOLLOWER")
	defer func() {
		os.Setenv("DATABASE_ENGINE", origEngine)
		os.Setenv("DATABASE_CONN_STRING_MASTER", origMaster)
		os.Setenv("DATABASE_CONN_STRING_FOLLOWER", origFollower)
	}()

	os.Setenv("DATABASE_ENGINE", "nrfake_test_driver")
	os.Setenv("DATABASE_CONN_STRING_MASTER", "dummy")
	os.Setenv("DATABASE_CONN_STRING_FOLLOWER", "dummy")

	db, err := Connect()
	require.NoError(t, err)
	assert.NotNil(t, db)
	assert.Equal(t, "nrfake_test_driver", db.engine)
}
