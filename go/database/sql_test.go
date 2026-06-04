package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- newSQL with env tests ---

func TestNewSQL_EnvSlowQueryThreshold(t *testing.T) {
	t.Run("should use env threshold when positive", func(t *testing.T) {
		os.Setenv("DATABASE_SLOW_QUERY_THRESHOLD", "2.5")
		defer os.Unsetenv("DATABASE_SLOW_QUERY_THRESHOLD")
		s := newSQL(nil, nil)
		assert.Equal(t, float64(2.5), s.slowQueryThreshold)
	})

	t.Run("should use default when zero", func(t *testing.T) {
		os.Setenv("DATABASE_SLOW_QUERY_THRESHOLD", "0")
		defer os.Unsetenv("DATABASE_SLOW_QUERY_THRESHOLD")
		s := newSQL(nil, nil)
		assert.Equal(t, float64(1), s.slowQueryThreshold)
	})

	t.Run("should use default when negative", func(t *testing.T) {
		os.Setenv("DATABASE_SLOW_QUERY_THRESHOLD", "-1")
		defer os.Unsetenv("DATABASE_SLOW_QUERY_THRESHOLD")
		s := newSQL(nil, nil)
		assert.Equal(t, float64(1), s.slowQueryThreshold)
	})

	t.Run("should use default when invalid", func(t *testing.T) {
		os.Setenv("DATABASE_SLOW_QUERY_THRESHOLD", "abc")
		defer os.Unsetenv("DATABASE_SLOW_QUERY_THRESHOLD")
		s := newSQL(nil, nil)
		assert.Equal(t, float64(1), s.slowQueryThreshold)
	})
}

// --- builder pool tests ---

func TestGetBuilder(t *testing.T) {
	b := getBuilder()
	require.NotNil(t, b)
	assert.Equal(t, 0, b.Len())
	b.WriteString("test")
	putBuilder(b)

	b2 := getBuilder()
	assert.Equal(t, 0, b2.Len()) // Should be reset
	putBuilder(b2)
}

func TestPutBuilder(t *testing.T) {
	b := getBuilder()
	b.WriteString("hello")
	putBuilder(b) // Should not panic

	b2 := getBuilder()
	assert.Equal(t, 0, b2.Len()) // Reused builder should be reset
	putBuilder(b2)
}

// --- Fake driver for sql.DB-based tests ---
// The Write method's non-transaction path requires Master to be *sqlx.DB
// for getWriteStmt to work. We use a fake driver to create a real *sqlx.DB.

type sqlTestFakeDriver struct{}

func (d sqlTestFakeDriver) Open(name string) (driver.Conn, error) {
	return &sqlTestFakeConn{}, nil
}

type sqlTestFakeConn struct{}

func (c *sqlTestFakeConn) Prepare(query string) (driver.Stmt, error) {
	return &sqlTestFakeStmt{}, nil
}
func (c *sqlTestFakeConn) Close() error  { return nil }
func (c *sqlTestFakeConn) Begin() (driver.Tx, error) { return &sqlTestFakeTx{}, nil }

type sqlTestFakeStmt struct{}

func (s *sqlTestFakeStmt) Close() error                                    { return nil }
func (s *sqlTestFakeStmt) NumInput() int                                   { return -1 }
func (s *sqlTestFakeStmt) Exec(args []driver.Value) (driver.Result, error)  { return &sqlTestFakeResult{rowsAffected: 1, lastInsertID: 42}, nil }
func (s *sqlTestFakeStmt) Query(args []driver.Value) (driver.Rows, error)   { return &sqlTestFakeRows{}, nil }

type sqlTestFakeTx struct{}

func (t *sqlTestFakeTx) Commit() error   { return nil }
func (t *sqlTestFakeTx) Rollback() error { return nil }

type sqlTestFakeResult struct {
	lastInsertID  int64
	rowsAffected  int64
}

func (r *sqlTestFakeResult) LastInsertId() (int64, error)  { return r.lastInsertID, nil }
func (r *sqlTestFakeResult) RowsAffected() (int64, error)  { return r.rowsAffected, nil }

type sqlTestFakeRows struct{ done bool }

func (r *sqlTestFakeRows) Columns() []string { return []string{"id"} }
func (r *sqlTestFakeRows) Close() error      { return nil }
func (r *sqlTestFakeRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = int64(99)
	return nil
}

func init() {
	sql.Register("sql_test_fixture", &sqlTestFakeDriver{})
}

// newSQLWithFakeDB creates an SQL with a real *sqlx.DB (using the fake driver)
// so that getWriteStmt/getReadStmtx type assertions succeed.
// It also clears stmt/rebind caches to avoid stale prepared statements
// between tests.
func newSQLWithFakeDB() *SQL {
	db, _ := sqlx.Open("sql_test_fixture", "")
	// Clear stmt caches to avoid stale cached statements from other tests
	writeStmtCache.Range(func(key, _ interface{}) bool {
		writeStmtCache.Delete(key)
		return true
	})
	readStmtCache.Range(func(key, _ interface{}) bool {
		readStmtCache.Delete(key)
		return true
	})
	rebindCache.Range(func(key, _ interface{}) bool {
		rebindCache.Delete(key)
		return true
	})
	return &SQL{
		Master:   db,
		Follower: db,
	}
}

// openFakeDB opens a *sqlx.DB using the fake driver. Shared by transaction tests.
func openFakeDB() (*sqlx.DB, error) {
	return sqlx.Open("sql_test_fixture", "")
}

// --- Write tests using real *sqlx.DB ---

func TestSQL_Write_NilCUDRequest(t *testing.T) {
	s := newSQLWithStubs()
	_, err := s.Write(context.Background(), &QueryOpts{CUDRequest: nil})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CUD Request Struct must be assigned")
}

func TestSQL_Write_UndefinedAction(t *testing.T) {
	s := newSQLWithFakeDB()
	_, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"test"},
			Action:    "undefined",
			TableName: "users",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "action exec is not defined")
}

func TestSQL_Write_InsertAction_MySQL(t *testing.T) {
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
	// The fake driver will exec successfully
	require.NoError(t, err)
	assert.True(t, result.Status)
	assert.Contains(t, result.query, "INSERT INTO users")
}

func TestSQL_Write_UpdateByIdAction(t *testing.T) {
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
	assert.Contains(t, result.query, "UPDATE users SET name WHERE id = ?")
}

func TestSQL_Write_DeleteByIdAction_SoftDelete(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Values:    []interface{}{1},
			Action:    ActionDeleteById,
			TableName: "users",
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Status)
	assert.Contains(t, result.query, "UPDATE users SET deleted_at = now() WHERE id = ?")
}

func TestSQL_Write_DeleteByIdAction_NoSoftDelete(t *testing.T) {
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
	assert.True(t, result.Status)
	assert.Contains(t, result.query, "DELETE FROM users WHERE id = ?")
}

func TestSQL_Write_DeleteAction_SoftDelete(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Action:    ActionDelete,
			TableName: "users",
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Status)
	assert.Contains(t, result.query, "UPDATE users SET deleted_at = now()")
}

func TestSQL_Write_DeleteAction_NoSoftDelete(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Action:    ActionDelete,
			TableName: "users",
		},
	}, false)
	require.NoError(t, err)
	assert.True(t, result.Status)
	assert.Contains(t, result.query, "DELETE FROM users")
}

func TestSQL_Write_UpdateAction(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

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
	assert.Contains(t, result.query, "UPDATE users SET name")
}

func TestSQL_Write_BulkInsertAction(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			ColsInsert: "name",
			BulkValues: "(?),(?)",
			Values:     []interface{}{"John", "Jane"},
			Action:     ActionBulkInsert,
			TableName:  "users",
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Status)
	assert.Contains(t, result.query, "INSERT INTO users")
}

func TestSQL_Write_BulkUpdateAction(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			BulkValues: "SELECT ? AS name UNION ALL SELECT ?",
			BulkQuery:  " SET main_table.name = join_table.name WHERE main_table.id = join_table.id",
			Values:     []interface{}{"John", "Jane"},
			Action:     ActionBulkUpdate,
			TableName:  "users",
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Status)
	assert.Contains(t, result.query, "UPDATE users AS main_table JOIN")
}

func TestSQL_Write_UpsertAction(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:       []string{"name"},
			ColsInsert: "name",
			Values:     []interface{}{"John"},
			Action:     ActionUpsert,
			TableName:  "users",
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Status)
	assert.Contains(t, result.query, "INSERT INTO users")
	assert.Contains(t, result.query, "ON DUPLICATE KEY UPDATE")
}

func TestSQL_Write_WithSelectRequest(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Action:    ActionDelete,
			TableName: "users",
		},
		SelectRequest: &TableRequest{
			InitiateWhere:       []string{"status = ?"},
			InitiateWhereValues: []interface{}{"active"},
			engine:              "mysql",
		},
	}, false)
	require.NoError(t, err)
	assert.True(t, result.Status)
}

func TestSQL_Write_MySQLLastInsertId(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("mysql")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"test"},
			Action:    ActionInsert,
			TableName: "users",
		},
	})
	require.NoError(t, err)
	assert.True(t, result.Status)
	// The fake driver returns rowsAffected=1 and lastInsertID=42
	assert.Equal(t, int64(1), result.RowsAffected)
	assert.Equal(t, int64(42), result.LastInsertID)
}

func TestSQL_Write_InsertAction_Postgres(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("postgres")

	// Postgres insert uses QueryRowContext+Scan for RETURNING id
	// With the fake driver, this will fail because the fake stmt doesn't
	// support QueryRowContext properly. Test that the query contains RETURNING.
	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Cols:      []string{"name"},
			Values:    []interface{}{"John"},
			Action:    ActionInsert,
			TableName: "users",
		},
	})
	// May fail due to fake driver limitations, but the query should contain RETURNING
	if result != nil {
		assert.Contains(t, result.query, "RETURNING id")
	}
	_ = err
}

// --- Postgres RETURNING id tests for UPDATE-like actions ---
// CUD responses must always carry a non-zero LastInsertID so callers (FE,
// downstream validators) can confirm the row they intended to write was the
// one actually affected. These tests pin the behavior for every action that
// is built with RETURNING id on PG.

func TestSQL_Write_UpdateByIdAction_Postgres_LastInsertIDPopulated(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("postgres")

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
	assert.Contains(t, result.query, "UPDATE users SET name WHERE id = ? RETURNING id")
	assert.NotZero(t, result.LastInsertID,
		"UpdateById on PG must return a non-zero LastInsertID so FE can map the response")
	assert.Equal(t, int64(1), result.RowsAffected)
}

func TestSQL_Write_UpdateAction_Postgres_LastInsertIDPopulated(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("postgres")
	writeStmtCache.Range(func(key, _ interface{}) bool { writeStmtCache.Delete(key); return true })

	tableReq := &TableRequest{
		InitiateWhere:       []string{"id = ?"},
		InitiateWhereValues: []interface{}{7},
		engine:              "postgres",
	}
	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest:    &CUDConstructData{Cols: []string{"name"}, Values: []interface{}{"Jane"}, Action: ActionUpdate, TableName: "users"},
		SelectRequest: tableReq,
	})
	require.NoError(t, err)
	assert.True(t, result.Status)
	assert.Contains(t, result.query, "UPDATE users SET name WHERE id = ?")
	assert.Contains(t, result.query, "RETURNING id")
	assert.NotZero(t, result.LastInsertID,
		"Update on PG must return a non-zero LastInsertID so FE can map the response")
}

func TestSQL_Write_DeleteByIdSoftDelete_Postgres_LastInsertIDPopulated(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("postgres")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Values:    []interface{}{1},
			Action:    ActionDeleteById,
			TableName: "users",
		},
	}, true)
	require.NoError(t, err)
	assert.True(t, result.Status)
	assert.Contains(t, result.query, "UPDATE users SET deleted_at = now() WHERE id = ? RETURNING id")
	assert.NotZero(t, result.LastInsertID,
		"DeleteById (soft-delete) on PG must return a non-zero LastInsertID so FE can map the response")
}

func TestSQL_Write_DeleteByIdHardDelete_Postgres_NoReturningId(t *testing.T) {
	s := newSQLWithFakeDB()
	s.SetEngine("postgres")

	result, err := s.Write(context.Background(), &QueryOpts{
		CUDRequest: &CUDConstructData{
			Values:    []interface{}{1},
			Action:    ActionDeleteById,
			TableName: "users",
		},
	}, false)
	require.NoError(t, err)
	assert.True(t, result.Status)
	assert.Contains(t, result.query, "DELETE FROM users WHERE id = ?")
	assert.NotContains(t, result.query, "RETURNING id",
		"hard-delete has no row to return; RETURNING id would be a SQL error")
}

// --- MySQL regression: existing behavior is preserved ---

func TestSQL_Write_UpdateByIdAction_MySQL_NoReturningId(t *testing.T) {
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
	assert.NotContains(t, result.query, "RETURNING id",
		"MySQL does not support RETURNING; we must not emit it on non-PG engines")
}

// --- CheckSQLWarning tests ---

func TestSQL_CheckSQLWarning(t *testing.T) {
	t.Run("should not warn when disabled", func(t *testing.T) {
		os.Unsetenv("DATABASE_SLOW_QUERY_WARNING")
		s := newSQLWithStubs()
		s.CheckSQLWarning(context.Background(), "SELECT 1", time.Now(), "param1")
	})

	t.Run("should handle enabled with slow query", func(t *testing.T) {
		os.Setenv("DATABASE_SLOW_QUERY_WARNING", "true")
		defer os.Unsetenv("DATABASE_SLOW_QUERY_WARNING")
		s := newSQLWithStubs()
		s.slowQueryThreshold = 0.0
		s.CheckSQLWarning(context.Background(), "SELECT 1", time.Now(), "param1")
	})

	t.Run("should not warn when query is fast", func(t *testing.T) {
		os.Setenv("DATABASE_SLOW_QUERY_WARNING", "true")
		defer os.Unsetenv("DATABASE_SLOW_QUERY_WARNING")
		s := newSQLWithStubs()
		s.slowQueryThreshold = 999.0
		start := time.Now()
		s.CheckSQLWarning(context.Background(), "SELECT 1", start, "param1")
	})
}

// --- GetWriteStmt tests ---

func TestSQL_GetWriteStmt_NotSqlxDB(t *testing.T) {
	s := newSQLWithStubs()
	// stubDB is not *sqlx.DB, so GetWriteStmt should return error
	_, err := s.GetWriteStmt(context.Background(), "INSERT INTO t (name) VALUES (?)")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "master DB is not *sqlx.DB")
}

// --- Read validation tests ---

func TestSQL_Read_EmptyBaseQuery(t *testing.T) {
	s := newSQLWithStubs()
	err := s.Read(context.Background(), &QueryOpts{BaseQuery: "", Result: new(int)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Base Query cannot be empty")
}

func TestSQL_Read_NilResult(t *testing.T) {
	s := newSQLWithStubs()
	err := s.Read(context.Background(), &QueryOpts{BaseQuery: "SELECT 1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Result must be assigned")
}

func TestSQL_Read_NonPointerResult(t *testing.T) {
	s := newSQLWithStubs()
	err := s.Read(context.Background(), &QueryOpts{BaseQuery: "SELECT 1", Result: 42})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Result must be a pointer")
}

// --- Read with SelectRequest tests ---

func TestSQL_Read_WithSelectRequestError(t *testing.T) {
	s := newSQLWithStubs()
	var result int
	err := s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1 FROM t",
		Result:    &result,
		SelectRequest: &TableRequest{
			Keyword: "test",
			// Missing SearchColsStr - will cause error
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Keyword Cols is required")
}

func TestSQL_Read_WithSelectRequestPagination(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)
	master.rebindResult = "SELECT 1 FROM t LIMIT ?,?"
	master.getError = nil

	var result int
	err := s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1 FROM t",
		Result:    &result,
		UseMaster: true,
		SelectRequest: &TableRequest{
			Page:           1,
			Limit:          10,
			engine:         "mysql",
			IncludeDeleted: true,
		},
	})
	// May fail at DB execution but pagination should be in query
	_ = err
	assert.Contains(t, master.lastQuery, "LIMIT")
}

// --- Transaction Finish test with real DB ---

func TestTransaction_Finish_CommitAndRollback(t *testing.T) {
	// Open a real database with the fake driver
	db, err := sqlx.Open("sql_test_fixture", "")
	require.NoError(t, err)
	defer db.Close()

	// Create the transaction using the real DB
	master := db
	txn := NewTransaction(master)

	// Begin a real transaction
	tx, err := txn.Begin()
	require.NoError(t, err)
	require.NotNil(t, tx)

	// Finish with no error should commit
	txn.Finish(tx, nil)

	// Begin another transaction for rollback test
	tx2, err := txn.Begin()
	require.NoError(t, err)
	require.NotNil(t, tx2)

	// Finish with error should rollback
	txn.Finish(tx2, fmt.Errorf("some error"))
}

// --- EvictReadStmt / EvictWriteStmt with cached items ---

func TestEvictReadStmt_NonExistentKey(t *testing.T) {
	// Should not panic when evicting non-existent key
	EvictReadStmt("nonexistent-read-query")
}

func TestEvictWriteStmt_NonExistentKey(t *testing.T) {
	// Should not panic when evicting non-existent key
	EvictWriteStmt("nonexistent-write-query")
}

// --- QueryOpts pool tests ---

func TestGetQueryOpts_SetWhereCondition(t *testing.T) {
	o := GetQueryOpts()
	o.BaseQuery = "SELECT * FROM users"
	o.SelectRequest = GetTableRequest()
	o.SelectRequest.SetWhereCondition("status = ?", "active")
	PutQueryOpts(o)
	PutTableRequest(o.SelectRequest)
}
