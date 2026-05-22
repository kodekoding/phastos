package database

import (
	"context"
	"database/sql"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTableRequest_SetWhereCondition(t *testing.T) {
	t.Run("should add condition with value", func(t *testing.T) {
		req := &TableRequest{}
		req.SetWhereCondition("name = ?", "John")

		assert.Len(t, req.InitiateWhere, 1)
		assert.Equal(t, "name = ?", req.InitiateWhere[0])
		assert.Len(t, req.InitiateWhereValues, 1)
		assert.Equal(t, "John", req.InitiateWhereValues[0])
	})

	t.Run("should add condition without value", func(t *testing.T) {
		req := &TableRequest{}
		req.SetWhereCondition("deleted_at IS NULL")

		assert.Len(t, req.InitiateWhere, 1)
		assert.Equal(t, "deleted_at IS NULL", req.InitiateWhere[0])
		assert.Empty(t, req.InitiateWhereValues)
	})

	t.Run("should accumulate multiple conditions", func(t *testing.T) {
		req := &TableRequest{}
		req.SetWhereCondition("status = ?", "active")
		req.SetWhereCondition("age > ?", 18)
		req.SetWhereCondition("deleted_at IS NULL")

		assert.Len(t, req.InitiateWhere, 3)
		assert.Equal(t, "status = ?", req.InitiateWhere[0])
		assert.Equal(t, "age > ?", req.InitiateWhere[1])
		assert.Equal(t, "deleted_at IS NULL", req.InitiateWhere[2])

		assert.Len(t, req.InitiateWhereValues, 2)
		assert.Equal(t, "active", req.InitiateWhereValues[0])
		assert.Equal(t, 18, req.InitiateWhereValues[1])
	})

	t.Run("should handle multiple values for IN clause", func(t *testing.T) {
		req := &TableRequest{}
		req.SetWhereCondition("id IN (?,?,?)", 1, 2, 3)

		assert.Len(t, req.InitiateWhere, 1)
		assert.Len(t, req.InitiateWhereValues, 3)
		assert.Equal(t, 1, req.InitiateWhereValues[0])
		assert.Equal(t, 2, req.InitiateWhereValues[1])
		assert.Equal(t, 3, req.InitiateWhereValues[2])
	})

	t.Run("should skip nil value", func(t *testing.T) {
		req := &TableRequest{}
		req.SetWhereCondition("status = ?", nil)

		assert.Len(t, req.InitiateWhere, 1)
		assert.Empty(t, req.InitiateWhereValues)
	})
}

func TestCUDConstructData_SetValues(t *testing.T) {
	t.Run("should append values", func(t *testing.T) {
		data := &CUDConstructData{}
		data.SetValues("value1")
		data.SetValues(42)
		data.SetValues(true)

		assert.Len(t, data.Values, 3)
		assert.Equal(t, "value1", data.Values[0])
		assert.Equal(t, 42, data.Values[1])
		assert.Equal(t, true, data.Values[2])
	})
}

func TestExecutedQuery_GetGeneratedQuery(t *testing.T) {
	t.Run("should return query and params map", func(t *testing.T) {
		eq := &executedQuery{
			query:  "SELECT * FROM users WHERE id = ?",
			params: []interface{}{1},
		}

		result := eq.GetGeneratedQuery()
		assert.Len(t, result, 1)

		params, exists := result["SELECT * FROM users WHERE id = ?"]
		assert.True(t, exists)
		assert.Equal(t, []interface{}{1}, params)
	})

	t.Run("should handle empty query", func(t *testing.T) {
		eq := &executedQuery{}
		result := eq.GetGeneratedQuery()
		assert.Len(t, result, 1)

		params, exists := result[""]
		assert.True(t, exists)
		assert.Nil(t, params)
	})

	t.Run("should handle multiple params", func(t *testing.T) {
		eq := &executedQuery{
			query:  "SELECT * FROM users WHERE id = ? AND name = ?",
			params: []interface{}{1, "John"},
		}
		result := eq.GetGeneratedQuery()
		params := result["SELECT * FROM users WHERE id = ? AND name = ?"]
		assert.Equal(t, []interface{}{1, "John"}, params)
	})
}

// --- Stub implementations for testing SQL struct delegation methods ---

// stubDB implements both Master and Follower interfaces for testing.
type stubDB struct {
	lastQuery     string
	lastArgs      []interface{}
	getError      error
	selectError   error
	queryRows     *sql.Rows
	queryError    error
	queryRow      *sql.Row
	queryRowx     *sqlx.Row
	queryxRows    *sqlx.Rows
	rebindResult  string
	namedRows     *sqlx.Rows
	namedError    error
	execResult    sql.Result
	execError     error
	beginTx       *sql.Tx
	beginError    error
	beginxTx      *sqlx.Tx
	beginxError   error
	beginTxOpts   *sql.Tx
	beginTxError  error
	namedExecRes  sql.Result
	namedExecErr  error
	bindNamedQry  string
	bindNamedArgs []interface{}
	bindNamedErr  error
	lastCtx       context.Context
}

func (s *stubDB) Get(dest interface{}, query string, args ...interface{}) error {
	s.lastQuery = query
	s.lastArgs = args
	return s.getError
}

func (s *stubDB) GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	s.lastCtx = ctx
	s.lastQuery = query
	s.lastArgs = args
	return s.getError
}

func (s *stubDB) Select(dest interface{}, query string, args ...interface{}) error {
	s.lastQuery = query
	s.lastArgs = args
	return s.selectError
}

func (s *stubDB) SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	s.lastCtx = ctx
	s.lastQuery = query
	s.lastArgs = args
	return s.selectError
}

func (s *stubDB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	s.lastQuery = query
	s.lastArgs = args
	return s.queryRows, s.queryError
}

func (s *stubDB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	s.lastCtx = ctx
	s.lastQuery = query
	s.lastArgs = args
	return s.queryRows, s.queryError
}

func (s *stubDB) QueryRow(query string, args ...interface{}) *sql.Row {
	s.lastQuery = query
	s.lastArgs = args
	return s.queryRow
}

func (s *stubDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	s.lastCtx = ctx
	s.lastQuery = query
	s.lastArgs = args
	return s.queryRow
}

func (s *stubDB) QueryRowx(query string, args ...interface{}) *sqlx.Row {
	s.lastQuery = query
	s.lastArgs = args
	return s.queryRowx
}

func (s *stubDB) QueryRowxContext(ctx context.Context, query string, args ...interface{}) *sqlx.Row {
	s.lastCtx = ctx
	s.lastQuery = query
	s.lastArgs = args
	return s.queryRowx
}

func (s *stubDB) QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error) {
	s.lastCtx = ctx
	s.lastQuery = query
	s.lastArgs = args
	return s.queryxRows, s.queryError
}

func (s *stubDB) Rebind(q string) string {
	return s.rebindResult
}

func (s *stubDB) NamedQuery(query string, arg interface{}) (*sqlx.Rows, error) {
	s.lastQuery = query
	s.lastArgs = []interface{}{arg}
	return s.namedRows, s.namedError
}

func (s *stubDB) NamedQueryContext(ctx context.Context, query string, arg interface{}) (*sqlx.Rows, error) {
	s.lastCtx = ctx
	s.lastQuery = query
	s.lastArgs = []interface{}{arg}
	return s.namedRows, s.namedError
}

func (s *stubDB) Exec(query string, args ...interface{}) (sql.Result, error) {
	s.lastQuery = query
	s.lastArgs = args
	return s.execResult, s.execError
}

func (s *stubDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	s.lastCtx = ctx
	s.lastQuery = query
	s.lastArgs = args
	return s.execResult, s.execError
}

func (s *stubDB) Begin() (*sql.Tx, error) {
	return s.beginTx, s.beginError
}

func (s *stubDB) Beginx() (*sqlx.Tx, error) {
	return s.beginxTx, s.beginxError
}

func (s *stubDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	s.lastCtx = ctx
	return s.beginTxOpts, s.beginTxError
}

func (s *stubDB) NamedExec(query string, arg interface{}) (sql.Result, error) {
	s.lastQuery = query
	s.lastArgs = []interface{}{arg}
	return s.namedExecRes, s.namedExecErr
}

func (s *stubDB) NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error) {
	s.lastCtx = ctx
	s.lastQuery = query
	s.lastArgs = []interface{}{arg}
	return s.namedExecRes, s.namedExecErr
}

func (s *stubDB) BindNamed(query string, arg interface{}) (string, []interface{}, error) {
	return s.bindNamedQry, s.bindNamedArgs, s.bindNamedErr
}

// sqlmockResult is a simple sql.Result implementation for testing.
type sqlmockResult struct {
	lastInsertID int64
	rowsAffected int64
}

func (r sqlmockResult) LastInsertId() (int64, error)  { return r.lastInsertID, nil }
func (r sqlmockResult) RowsAffected() (int64, error) { return r.rowsAffected, nil }

// --- SQL struct delegation tests ---

func newSQLWithStubs() *SQL {
	return &SQL{
		Master:   &stubDB{},
		Follower: &stubDB{},
	}
}

func TestSQL_Get(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)

	var dest string
	err := s.Get(&dest, "SELECT * FROM users WHERE id = ?", 42)
	assert.NoError(t, err)
	assert.Equal(t, "SELECT * FROM users WHERE id = ?", master.lastQuery)
	assert.Equal(t, []interface{}{42}, master.lastArgs)
}

func TestSQL_GetContext(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)

	ctx := context.Background()
	var dest string
	err := s.GetContext(ctx, &dest, "SELECT * FROM users WHERE id = ?", 42)
	assert.NoError(t, err)
	assert.Equal(t, ctx, master.lastCtx)
	assert.Equal(t, "SELECT * FROM users WHERE id = ?", master.lastQuery)
}

func TestSQL_Select(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)

	var dest []string
	err := s.Select(&dest, "SELECT name FROM users")
	assert.NoError(t, err)
	assert.Equal(t, "SELECT name FROM users", master.lastQuery)
}

func TestSQL_SelectContext(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)

	ctx := context.Background()
	var dest []string
	err := s.SelectContext(ctx, &dest, "SELECT name FROM users")
	assert.NoError(t, err)
	assert.Equal(t, ctx, master.lastCtx)
}

func TestSQL_Query(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)

	rows, err := s.Query("SELECT * FROM users")
	assert.NoError(t, err)
	assert.Nil(t, rows)
	assert.Equal(t, "SELECT * FROM users", master.lastQuery)
}

func TestSQL_QueryContext(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)

	ctx := context.Background()
	rows, err := s.QueryContext(ctx, "SELECT * FROM users")
	assert.NoError(t, err)
	assert.Nil(t, rows)
	assert.Equal(t, ctx, master.lastCtx)
}

func TestSQL_QueryRow(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)

	row := s.QueryRow("SELECT 1")
	assert.Nil(t, row)
	assert.Equal(t, "SELECT 1", master.lastQuery)
}

func TestSQL_QueryRowContext(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)

	ctx := context.Background()
	row := s.QueryRowContext(ctx, "SELECT 1")
	assert.Nil(t, row)
	assert.Equal(t, ctx, master.lastCtx)
}

func TestSQL_QueryRowx(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)

	row := s.QueryRowx("SELECT 1")
	assert.Nil(t, row)
	assert.Equal(t, "SELECT 1", master.lastQuery)
}

func TestSQL_QueryRowxContext(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)

	ctx := context.Background()
	row := s.QueryRowxContext(ctx, "SELECT 1")
	assert.Nil(t, row)
	assert.Equal(t, ctx, master.lastCtx)
}

func TestSQL_QueryxContext(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)

	ctx := context.Background()
	rows, err := s.QueryxContext(ctx, "SELECT * FROM users")
	assert.NoError(t, err)
	assert.Nil(t, rows)
	assert.Equal(t, ctx, master.lastCtx)
}

func TestSQL_Rebind(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)
	master.rebindResult = "SELECT * FROM users WHERE id = $1"

	result := s.Rebind("SELECT * FROM users WHERE id = ?")
	assert.Equal(t, "SELECT * FROM users WHERE id = $1", result)
}

func TestSQL_NamedQuery(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)

	arg := map[string]interface{}{"id": 1}
	rows, err := s.NamedQuery("SELECT * FROM users WHERE id = :id", arg)
	assert.NoError(t, err)
	assert.Nil(t, rows)
	assert.Equal(t, "SELECT * FROM users WHERE id = :id", master.lastQuery)
}

func TestSQL_NamedQueryContext(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)

	ctx := context.Background()
	arg := map[string]interface{}{"id": 1}
	rows, err := s.NamedQueryContext(ctx, "SELECT * FROM users WHERE id = :id", arg)
	assert.NoError(t, err)
	assert.Nil(t, rows)
	assert.Equal(t, ctx, master.lastCtx)
}

func TestSQL_GetTransaction(t *testing.T) {
	s := newSQLWithStubs()
	txn := s.GetTransaction()
	assert.NotNil(t, txn)
}

func TestSQL_SetEngine(t *testing.T) {
	s := newSQLWithStubs()
	assert.Equal(t, "", s.engine)

	s.SetEngine("postgres")
	assert.Equal(t, "postgres", s.engine)

	s.SetEngine("mysql")
	assert.Equal(t, "mysql", s.engine)
}

func TestSQL_SetSlowQueryThreshold(t *testing.T) {
	s := newSQLWithStubs()
	assert.Equal(t, float64(0), s.slowQueryThreshold)

	s.SetSlowQueryThreshold(5.0)
	assert.Equal(t, 5.0, s.slowQueryThreshold)

	s.SetSlowQueryThreshold(0.5)
	assert.Equal(t, 0.5, s.slowQueryThreshold)
}

func TestSQL_Get_Error(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)
	master.getError = sql.ErrConnDone

	var dest string
	err := s.Get(&dest, "SELECT * FROM users")
	assert.Equal(t, sql.ErrConnDone, err)
}

func TestSQL_Select_Error(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)
	master.selectError = sql.ErrConnDone

	var dest []string
	err := s.Select(&dest, "SELECT * FROM users")
	assert.Equal(t, sql.ErrConnDone, err)
}

func TestSQL_Query_Error(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)
	master.queryError = sql.ErrConnDone

	rows, err := s.Query("SELECT * FROM users")
	assert.Equal(t, sql.ErrConnDone, err)
	assert.Nil(t, rows)
}

func TestSQL_Exec(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)
	master.execResult = sqlmockResult{lastInsertID: 1, rowsAffected: 1}

	result, err := s.Exec("INSERT INTO users (name) VALUES (?)", "John")
	assert.NoError(t, err)
	assert.Equal(t, "INSERT INTO users (name) VALUES (?)", master.lastQuery)
	assert.Equal(t, []interface{}{"John"}, master.lastArgs)

	rowsAffected, _ := result.RowsAffected()
	assert.Equal(t, int64(1), rowsAffected)
}

func TestSQL_ExecContext(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)
	master.execResult = sqlmockResult{lastInsertID: 1, rowsAffected: 1}

	ctx := context.Background()
	result, err := s.ExecContext(ctx, "INSERT INTO users (name) VALUES (?)", "John")
	assert.NoError(t, err)
	assert.Equal(t, ctx, master.lastCtx)

	rowsAffected, _ := result.RowsAffected()
	assert.Equal(t, int64(1), rowsAffected)
}

func TestSQL_Begin(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)
	master.beginError = sql.ErrConnDone

	tx, err := s.Begin()
	assert.Equal(t, sql.ErrConnDone, err)
	assert.Nil(t, tx)
}

func TestSQL_Beginx(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)
	master.beginxError = sql.ErrConnDone

	tx, err := s.Beginx()
	assert.Equal(t, sql.ErrConnDone, err)
	assert.Nil(t, tx)
}

func TestSQL_BeginTx(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)
	master.beginTxError = sql.ErrConnDone

	ctx := context.Background()
	tx, err := s.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	assert.Equal(t, sql.ErrConnDone, err)
	assert.Nil(t, tx)
}

func TestSQL_NamedExec(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)
	master.namedExecRes = sqlmockResult{lastInsertID: 5, rowsAffected: 1}

	arg := map[string]interface{}{"name": "John"}
	result, err := s.NamedExec("INSERT INTO users (name) VALUES (:name)", arg)
	assert.NoError(t, err)
	assert.Equal(t, "INSERT INTO users (name) VALUES (:name)", master.lastQuery)

	lastInsertID, _ := result.LastInsertId()
	assert.Equal(t, int64(5), lastInsertID)
}

func TestSQL_NamedExecContext(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)
	master.namedExecRes = sqlmockResult{lastInsertID: 5, rowsAffected: 1}

	ctx := context.Background()
	arg := map[string]interface{}{"name": "John"}
	result, err := s.NamedExecContext(ctx, "INSERT INTO users (name) VALUES (:name)", arg)
	assert.NoError(t, err)
	assert.Equal(t, ctx, master.lastCtx)

	lastInsertID, _ := result.LastInsertId()
	assert.Equal(t, int64(5), lastInsertID)
}

func TestSQL_BindNamed(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)
	master.bindNamedQry = "INSERT INTO users (name) VALUES (?)"
	master.bindNamedArgs = []interface{}{"John"}

	arg := map[string]interface{}{"name": "John"}
	query, args, err := s.BindNamed("INSERT INTO users (name) VALUES (:name)", arg)
	assert.NoError(t, err)
	assert.Equal(t, "INSERT INTO users (name) VALUES (?)", query)
	assert.Equal(t, []interface{}{"John"}, args)
}

func TestSQL_DelegatesToMaster(t *testing.T) {
	s := &SQL{
		Master:   &stubDB{},
		Follower: &stubDB{},
	}
	master := s.Master.(*stubDB)
	follower := s.Follower.(*stubDB)

	var dest string
	_ = s.Get(&dest, "SELECT 1")

	assert.Equal(t, "SELECT 1", master.lastQuery)
	assert.Empty(t, follower.lastQuery) // Follower should not be called
}

// --- Read/Write validation tests ---

func TestSQL_Read_Validation_EmptyBaseQuery(t *testing.T) {
	s := newSQLWithStubs()
	var result string
	err := s.Read(context.Background(), &QueryOpts{Result: &result})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Base Query cannot be empty")
}

func TestSQL_Read_Validation_NilResult(t *testing.T) {
	s := newSQLWithStubs()
	err := s.Read(context.Background(), &QueryOpts{BaseQuery: "SELECT 1", Result: nil})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Result must be assigned")
}

func TestSQL_Read_Validation_NonPointerResult(t *testing.T) {
	s := newSQLWithStubs()
	result := "not a pointer"
	err := s.Read(context.Background(), &QueryOpts{BaseQuery: "SELECT 1", Result: result})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Result must be a pointer")
}

func TestSQL_Write_Validation_NilCUDRequest(t *testing.T) {
	s := newSQLWithStubs()
	_, err := s.Write(context.Background(), &QueryOpts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CUD Request Struct must be assigned")
}
