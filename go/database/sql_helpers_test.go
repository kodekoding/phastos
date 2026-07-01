package database

import (
	"context"
	"strings"
	"testing"

	custerr "github.com/kodekoding/phastos/v2/go/error"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- generateParamArgsForLike tests ---

func TestGenerateParamArgsForLike(t *testing.T) {
	result := generateParamArgsForLike("test")
	assert.Equal(t, "%test%", result)
}

func TestGenerateParamArgsForLike_Empty(t *testing.T) {
	result := generateParamArgsForLike("")
	assert.Equal(t, "%%", result)
}

// --- checkKeyword tests ---

func TestCheckKeyword_WithKeywordAndSearchCols(t *testing.T) {
	req := &TableRequest{
		Keyword:       "john",
		SearchColsStr: "name,email",
	}
	var builder strings.Builder
	var params []interface{}

	err := checkKeyword(context.Background(), req, &builder, &params)
	require.NoError(t, err)

	assert.Equal(t, []string{"name", "email"}, req.SearchCols)
	assert.Contains(t, builder.String(), "name LIKE ? OR email LIKE ?")
	// Should have trailing " OR " before the closing ")"
	assert.Contains(t, builder.String(), " OR )")
	assert.Len(t, params, 2)
	assert.Equal(t, "%john%", params[0])
	assert.Equal(t, "%john%", params[1])
}

func TestCheckKeyword_WithKeywordAndSearchCols_WithInitiateWhere(t *testing.T) {
	req := &TableRequest{
		Keyword:         "john",
		SearchColsStr:   "name",
		InitiateWhere:   []string{"status = ?"},
		InitiateWhereValues: []interface{}{"active"},
	}
	var builder strings.Builder
	var params []interface{}

	err := checkKeyword(context.Background(), req, &builder, &params)
	require.NoError(t, err)
	assert.Contains(t, builder.String(), " AND (")
	assert.Contains(t, builder.String(), "name LIKE ?")
}

func TestCheckKeyword_KeywordWithoutSearchCols(t *testing.T) {
	req := &TableRequest{
		Keyword:     "john",
		SearchColsStr: "",
	}
	var builder strings.Builder
	var params []interface{}

	err := checkKeyword(context.Background(), req, &builder, &params)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Keyword Cols is required when Keyword Field is filled")
}

func TestCheckKeyword_NoKeyword(t *testing.T) {
	req := &TableRequest{
		Keyword: "",
	}
	var builder strings.Builder
	var params []interface{}

	err := checkKeyword(context.Background(), req, &builder, &params)
	assert.NoError(t, err)
	assert.Empty(t, builder.String())
	assert.Empty(t, params)
}

// --- checkSortParam tests ---

func TestCheckSortParam_WithOrderBy(t *testing.T) {
	req := &TableRequest{OrderBy: "created_at DESC"}
	var builder strings.Builder

	checkSortParam(context.Background(), req, &builder)
	assert.Contains(t, builder.String(), " ORDER BY created_at DESC")
}

func TestCheckSortParam_WithoutOrderBy(t *testing.T) {
	req := &TableRequest{OrderBy: ""}
	var builder strings.Builder

	checkSortParam(context.Background(), req, &builder)
	assert.Empty(t, builder.String())
}

// --- checkCreatedDateParam tests ---

func TestCheckCreatedDateParam_StartAndEnd(t *testing.T) {
	req := &TableRequest{
		CreatedStart: "2024-01-01",
		CreatedEnd:   "2024-12-31",
		engine:       "postgres",
	}
	var builder strings.Builder
	var params []interface{}

	checkCreatedDateParam(context.Background(), req, &builder, &params)
	query := builder.String()
	assert.Contains(t, query, "created_at >= ?")
	assert.Contains(t, query, "created_at <= ?")
	assert.Contains(t, query, " AND ")
	assert.Len(t, params, 2)
	assert.Equal(t, "2024-01-01 00:00:00", params[0])
	assert.Equal(t, "2024-12-31 23:59:59", params[1])
}

func TestCheckCreatedDateParam_StartOnly(t *testing.T) {
	req := &TableRequest{
		CreatedStart: "2024-01-01",
		engine:       "postgres",
	}
	var builder strings.Builder
	var params []interface{}

	checkCreatedDateParam(context.Background(), req, &builder, &params)
	assert.Contains(t, builder.String(), "created_at >= ?")
	assert.Len(t, params, 1)
	assert.Equal(t, "2024-01-01 00:00:00", params[0])
}

func TestCheckCreatedDateParam_EndOnly(t *testing.T) {
	req := &TableRequest{
		CreatedEnd: "2024-12-31",
		engine:     "postgres",
	}
	var builder strings.Builder
	var params []interface{}

	checkCreatedDateParam(context.Background(), req, &builder, &params)
	assert.Contains(t, builder.String(), "created_at <= ?")
	assert.Len(t, params, 1)
	assert.Equal(t, "2024-12-31 23:59:59", params[0])
}

func TestCheckCreatedDateParam_CustomDateColFilter(t *testing.T) {
	req := &TableRequest{
		CreatedStart:      "2024-01-01",
		CustomDateColFilter: "updated_at",
		engine:            "postgres",
	}
	var builder strings.Builder
	var params []interface{}

	checkCreatedDateParam(context.Background(), req, &builder, &params)
	assert.Contains(t, builder.String(), "updated_at >= ?")
}

func TestCheckCreatedDateParam_WithMainTableAlias(t *testing.T) {
	req := &TableRequest{
		CreatedStart:   "2024-01-01",
		MainTableAlias: "t",
		engine:         "postgres",
	}
	var builder strings.Builder
	var params []interface{}

	checkCreatedDateParam(context.Background(), req, &builder, &params)
	assert.Contains(t, builder.String(), "t.created_at >= ?")
}

func TestCheckCreatedDateParam_MySQLEngine(t *testing.T) {
	req := &TableRequest{
		CreatedStart: "2024-01-01",
		engine:       "mysql",
	}
	var builder strings.Builder
	var params []interface{}

	checkCreatedDateParam(context.Background(), req, &builder, &params)
	assert.Contains(t, builder.String(), "DATE_FORMAT(created_at")
	assert.Contains(t, builder.String(), "STR_TO_DATE(?,")
}

func TestCheckCreatedDateParam_NoDates(t *testing.T) {
	req := &TableRequest{engine: "postgres"}
	var builder strings.Builder
	var params []interface{}

	checkCreatedDateParam(context.Background(), req, &builder, &params)
	// Should still add deleted_at filter
	assert.Contains(t, builder.String(), "deleted_at IS NULL")
}

func TestCheckCreatedDateParam_IncludeDeleted(t *testing.T) {
	req := &TableRequest{
		IncludeDeleted: true,
		engine:         "postgres",
	}
	var builder strings.Builder
	var params []interface{}

	checkCreatedDateParam(context.Background(), req, &builder, &params)
	assert.Empty(t, builder.String()) // No deleted filter when IncludeDeleted=true
	assert.Empty(t, params)
}

func TestCheckCreatedDateParam_NotContainsDeletedCol(t *testing.T) {
	req := &TableRequest{
		NotContainsDeletedCol: true,
		engine:                "postgres",
	}
	var builder strings.Builder
	var params []interface{}

	checkCreatedDateParam(context.Background(), req, &builder, &params)
	assert.Empty(t, builder.String()) // No deleted filter when NotContainsDeletedCol=true
}

func TestCheckCreatedDateParam_IsDeleted(t *testing.T) {
	req := &TableRequest{
		IsDeleted: "1",
		engine:    "postgres",
	}
	var builder strings.Builder
	var params []interface{}

	checkCreatedDateParam(context.Background(), req, &builder, &params)
	assert.Contains(t, builder.String(), "deleted_at IS NOT NULL")
}

func TestCheckCreatedDateParam_IsDeletedWithMainTableAlias(t *testing.T) {
	req := &TableRequest{
		IsDeleted:       "1",
		MainTableAlias:  "t",
		engine:          "postgres",
	}
	var builder strings.Builder
	var params []interface{}

	checkCreatedDateParam(context.Background(), req, &builder, &params)
	assert.Contains(t, builder.String(), "t.deleted_at IS NOT NULL")
}

// --- checkInitiateWhere tests ---

func TestCheckInitiateWhere_WithConditions(t *testing.T) {
	req := &TableRequest{
		InitiateWhere:       []string{"status = ?", "age > ?"},
		InitiateWhereValues: []interface{}{"active", 18},
	}
	var builder strings.Builder
	var params []interface{}

	checkInitiateWhere(context.Background(), req, &builder, &params)
	// Should produce: "status = ? AND age > ?" (trailing " AND " removed)
	assert.Contains(t, builder.String(), "status = ? AND age > ?")
	assert.Equal(t, []interface{}{"active", 18}, params)
}

func TestCheckInitiateWhere_NoConditions(t *testing.T) {
	req := &TableRequest{}
	var builder strings.Builder
	var params []interface{}

	checkInitiateWhere(context.Background(), req, &builder, &params)
	assert.Empty(t, builder.String())
	assert.Empty(t, params)
}

func TestCheckInitiateWhere_SingleCondition(t *testing.T) {
	req := &TableRequest{
		InitiateWhere:       []string{"id = ?"},
		InitiateWhereValues: []interface{}{42},
	}
	var builder strings.Builder
	var params []interface{}

	checkInitiateWhere(context.Background(), req, &builder, &params)
	assert.Contains(t, builder.String(), "id = ?")
	assert.Equal(t, []interface{}{42}, params)
}

// --- sendNilResponse tests ---

func TestSendNilResponse_NoRowsError(t *testing.T) {
	result, err := sendNilResponse(errors.New("sql: no rows in result set"), "test.context")
	assert.Nil(t, result)
	assert.Nil(t, err)
}

func TestSendNilResponse_OtherError(t *testing.T) {
	otherErr := errors.New("connection refused")
	result, err := sendNilResponse(otherErr, "test.context")
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "test.context")
}

func TestSendNilResponse_WithParams(t *testing.T) {
	otherErr := errors.New("syntax error")
	result, err := sendNilResponse(otherErr, "test.context", "param1", 42)
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "test.context")
}

func TestSendNilResponse_UniqueViolation(t *testing.T) {
	pqErr := &pq.Error{
		Code:       "23505",
		Constraint: "users_email_key",
		Table:      "users",
	}
	result, err := sendNilResponse(pqErr, "test.context")
	assert.Nil(t, result)
	require.Error(t, err)

	var reqErr *custerr.RequestError
	require.True(t, errors.As(err, &reqErr))
	assert.Equal(t, 409, reqErr.GetCode())
}

func TestSendNilResponse_CheckViolation(t *testing.T) {
	pqErr := &pq.Error{
		Code:       "23514",
		Constraint: "some_check",
	}
	result, err := sendNilResponse(pqErr, "test.context")
	assert.Nil(t, result)
	require.Error(t, err)

	var reqErr *custerr.RequestError
	require.True(t, errors.As(err, &reqErr))
	assert.Equal(t, 422, reqErr.GetCode())
}

func TestSendNilResponse_OtherPostgresError(t *testing.T) {
	pqErr := &pq.Error{
		Code: "23503", // foreign_key_violation — not our target
	}
	result, err := sendNilResponse(pqErr, "test.context")
	assert.Nil(t, result)
	require.Error(t, err)

	var reqErr *custerr.RequestError
	require.True(t, errors.As(err, &reqErr))
	assert.Equal(t, 500, reqErr.GetCode())
}

func TestSendNilResponse_WrappedPQError(t *testing.T) {
	pqErr := &pq.Error{
		Code:       "23505",
		Constraint: "users_email_key",
	}
	wrappedErr := errors.Wrap(pqErr, "outer wrap")
	result, err := sendNilResponse(wrappedErr, "test.context")
	assert.Nil(t, result)
	require.Error(t, err)

	var reqErr *custerr.RequestError
	require.True(t, errors.As(err, &reqErr))
	assert.Equal(t, 409, reqErr.GetCode())
}

// --- GenerateAddOnQuery tests ---

func TestGenerateAddOnQuery_MinimalRequest(t *testing.T) {
	req := &TableRequest{engine: "postgres"}
	query, params, err := GenerateAddOnQuery(context.Background(), req)
	require.NoError(t, err)
	// Should add deleted_at filter
	assert.Contains(t, query, "deleted_at IS NULL")
	assert.Empty(t, params)
}

func TestGenerateAddOnQuery_WithKeyword(t *testing.T) {
	req := &TableRequest{
		Keyword:       "john",
		SearchColsStr: "name,email",
		engine:        "postgres",
	}
	query, params, err := GenerateAddOnQuery(context.Background(), req)
	require.NoError(t, err)
	assert.Contains(t, query, "WHERE")
	assert.Contains(t, query, "name LIKE ?")
	assert.Contains(t, query, "email LIKE ?")
	assert.True(t, len(params) >= 2)
}

func TestGenerateAddOnQuery_KeywordWithoutSearchCols(t *testing.T) {
	req := &TableRequest{
		Keyword:     "john",
		SearchColsStr: "",
		engine:      "postgres",
	}
	_, _, err := GenerateAddOnQuery(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Keyword Cols is required")
}

func TestGenerateAddOnQuery_WithInitiateWhere(t *testing.T) {
	req := &TableRequest{
		InitiateWhere:       []string{"status = ?"},
		InitiateWhereValues: []interface{}{"active"},
		engine:              "postgres",
	}
	query, params, err := GenerateAddOnQuery(context.Background(), req)
	require.NoError(t, err)
	assert.Contains(t, query, "WHERE")
	assert.Contains(t, query, "status = ?")
	assert.Contains(t, query, "deleted_at IS NULL")
	assert.Equal(t, []interface{}{"active"}, params[:1])
}

func TestGenerateAddOnQuery_WithPagination_Postgres(t *testing.T) {
	req := &TableRequest{
		Page:     2,
		Limit:    10,
		engine:   "postgres",
		IncludeDeleted: true, // Skip deleted filter
	}
	query, params, err := GenerateAddOnQuery(context.Background(), req)
	require.NoError(t, err)
	assert.Contains(t, query, "LIMIT ? OFFSET ?")
	// Postgres: LIMIT=10, OFFSET=10
	assert.Equal(t, []interface{}{10, 10}, params)
}

func TestGenerateAddOnQuery_WithPagination_MySQL(t *testing.T) {
	req := &TableRequest{
		Page:     3,
		Limit:    20,
		engine:   "mysql",
		IncludeDeleted: true,
	}
	query, params, err := GenerateAddOnQuery(context.Background(), req)
	require.NoError(t, err)
	assert.Contains(t, query, "LIMIT ?,?")
	// MySQL: offset=40, limit=20
	assert.Equal(t, []interface{}{40, 20}, params)
}

func TestGenerateAddOnQuery_WithGroupBy(t *testing.T) {
	req := &TableRequest{
		GroupBy:       "status",
		engine:        "postgres",
		IncludeDeleted: true,
	}
	query, _, err := GenerateAddOnQuery(context.Background(), req)
	require.NoError(t, err)
	assert.Contains(t, query, "GROUP BY status")
}

func TestGenerateAddOnQuery_WithOrderBy(t *testing.T) {
	req := &TableRequest{
		OrderBy:       "created_at DESC",
		engine:        "postgres",
		IncludeDeleted: true,
	}
	query, _, err := GenerateAddOnQuery(context.Background(), req)
	require.NoError(t, err)
	assert.Contains(t, query, "ORDER BY created_at DESC")
}

func TestGenerateAddOnQuery_WithCreatedDateRange(t *testing.T) {
	req := &TableRequest{
		CreatedStart:   "2024-01-01",
		CreatedEnd:     "2024-12-31",
		engine:         "postgres",
		IncludeDeleted: true,
	}
	query, params, err := GenerateAddOnQuery(context.Background(), req)
	require.NoError(t, err)
	assert.Contains(t, query, "WHERE")
	assert.Contains(t, query, "created_at >= ?")
	assert.Contains(t, query, "created_at <= ?")
	assert.True(t, len(params) >= 2)
}

func TestGenerateAddOnQuery_UnknownEngine_Pagination(t *testing.T) {
	req := &TableRequest{
		Page:     1,
		Limit:    10,
		engine:   "unknown",
		IncludeDeleted: true,
	}
	query, params, err := GenerateAddOnQuery(context.Background(), req)
	require.NoError(t, err)
	// Unknown engine should not add LIMIT/OFFSET
	assert.NotContains(t, query, "LIMIT")
	assert.Empty(t, params)
}

func TestGenerateAddOnQuery_PageZeroOrLimitZero_NoPagination(t *testing.T) {
	req := &TableRequest{
		Page:     0,
		Limit:    10,
		engine:   "postgres",
		IncludeDeleted: true,
	}
	query, params, err := GenerateAddOnQuery(context.Background(), req)
	require.NoError(t, err)
	assert.NotContains(t, query, "LIMIT")
	assert.Empty(t, params)
}

func TestGenerateAddOnQuery_ORReplacement(t *testing.T) {
	// Test the " OR )" -> ")" cleanup at the end
	req := &TableRequest{
		Keyword:       "test",
		SearchColsStr: "col1",
		engine:        "postgres",
		IncludeDeleted: true,
	}
	query, _, err := GenerateAddOnQuery(context.Background(), req)
	require.NoError(t, err)
	// The final query should have " OR )" replaced with ")" by GenerateAddOnQuery
	assert.NotContains(t, query, " OR )")
	assert.Contains(t, query, "col1 LIKE ?")
}

func TestGenerateAddOnQuery_FullQuery(t *testing.T) {
	req := &TableRequest{
		InitiateWhere:       []string{"status = ?"},
		InitiateWhereValues: []interface{}{"active"},
		Keyword:             "john",
		SearchColsStr:       "name",
		CreatedStart:        "2024-01-01",
		CreatedEnd:          "2024-12-31",
		GroupBy:             "status",
		OrderBy:             "created_at DESC",
		Page:                1,
		Limit:               10,
		engine:              "postgres",
	}
	query, params, err := GenerateAddOnQuery(context.Background(), req)
	require.NoError(t, err)
	assert.Contains(t, query, "WHERE")
	assert.Contains(t, query, "status = ?")
	assert.Contains(t, query, "name LIKE ?")
	assert.Contains(t, query, "created_at >= ?")
	assert.Contains(t, query, "created_at <= ?")
	assert.Contains(t, query, "GROUP BY status")
	assert.Contains(t, query, "ORDER BY created_at DESC")
	assert.Contains(t, query, "LIMIT ? OFFSET ?")
	assert.True(t, len(params) >= 5) // InitiateWhere + keyword + start + end + limit + offset
}

// --- SQL.IsPostgres tests ---

func TestSQL_IsPostgres(t *testing.T) {
	s := newSQLWithStubs()

	s.SetEngine("postgres")
	assert.True(t, s.IsPostgres())

	s.SetEngine("nrpostgres")
	assert.True(t, s.IsPostgres())

	s.SetEngine("mysql")
	assert.False(t, s.IsPostgres())

	s.SetEngine("")
	assert.False(t, s.IsPostgres())
}

// --- SQL.Engine tests ---

func TestSQL_Engine(t *testing.T) {
	s := newSQLWithStubs()

	s.SetEngine("mysql")
	assert.Equal(t, "mysql", s.Engine())

	s.SetEngine("postgres")
	assert.Equal(t, "postgres", s.Engine())
}

// --- MySQLEngineGroupActive tests ---

func TestMySQLEngineGroupActive(t *testing.T) {
	active, valid := MySQLEngineGroupActive("mysql")
	assert.True(t, active)
	assert.True(t, valid)

	active, valid = MySQLEngineGroupActive("nrmysql")
	assert.True(t, active)
	assert.True(t, valid)

	active, valid = MySQLEngineGroupActive("postgres")
	assert.False(t, active)
	assert.False(t, valid)

	active, valid = MySQLEngineGroupActive("")
	assert.False(t, active)
	assert.False(t, valid)
}

// --- newSQL tests ---

func TestNewSQL(t *testing.T) {
	// newSQL creates an SQL with default slowQueryThreshold=1 when env var not set
	s := newSQL(nil, nil)
	assert.Equal(t, float64(1), s.slowQueryThreshold)
}

// --- SQL.CachedRebind tests ---

func TestSQL_CachedRebind(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)
	master.rebindResult = "SELECT $1"

	// First call should call Master.Rebind
	result := s.CachedRebind("SELECT ?")
	assert.Equal(t, "SELECT $1", result)

	// Second call should return cached result (same rebindResult from stub)
	result2 := s.CachedRebind("SELECT ?")
	assert.Equal(t, "SELECT $1", result2)
}

func TestSQL_CachedRebind_DifferentQueries(t *testing.T) {
	s := newSQLWithStubs()
	master := s.Master.(*stubDB)

	master.rebindResult = "SELECT $1, $2"
	result1 := s.CachedRebind("SELECT ?, ?")
	assert.Equal(t, "SELECT $1, $2", result1)

	master.rebindResult = "SELECT $1"
	result2 := s.CachedRebind("SELECT ?")
	assert.Equal(t, "SELECT $1", result2)
}

// --- SQL.Read with Conditions callback ---

func TestSQL_Read_WithConditionsCallback(t *testing.T) {
	s := newSQLWithStubs()
	conditionsCalled := false
	var result string

	err := s.Read(context.Background(), &QueryOpts{
		BaseQuery: "SELECT 1",
		Result:    &result,
		Conditions: func(ctx context.Context) {
			conditionsCalled = true
		},
	})

	// Will fail after conditions because the stub doesn't have a real DB
	// but we can verify conditions was called
	assert.True(t, conditionsCalled)
	_ = err // Expected to fail at DB execution
}

// --- EvictReadStmt / EvictWriteStmt tests (just ensure they don't panic) ---

func TestEvictReadStmt_NoPanic(t *testing.T) {
	// Evicting a non-existent key should not panic
	EvictReadStmt("nonexistent-query")
}

func TestEvictWriteStmt_NoPanic(t *testing.T) {
	// Evicting a non-existent key should not panic
	EvictWriteStmt("nonexistent-query")
}
