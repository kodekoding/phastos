package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/volatiletech/null"
)

type (
	Master interface {
		Exec(query string, args ...interface{}) (sql.Result, error)

		// ExecContext use master database to exec query
		ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)

		// Begin transaction on master DB
		Begin() (*sql.Tx, error)

		// BeginTx begins transaction on master DB
		BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)

		// Rebind a query from the default bindtype (QUESTION) to the target bindtype.
		Rebind(sql string) string

		// NamedExec do named exec on master DB
		NamedExec(query string, arg interface{}) (sql.Result, error)

		// NamedExecContext do named exec on master DB
		NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error)

		// BindNamed do BindNamed on master DB
		BindNamed(query string, arg interface{}) (string, []interface{}, error)

		// QueryRow executes QueryRow against follower DB
		QueryRow(query string, args ...interface{}) *sql.Row

		// QueryRowContext from sql database
		QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	}

	// Follower defines operation that will be executed to follower DB
	Follower interface {
		// Get from follower database
		Get(dest interface{}, query string, args ...interface{}) error

		// Select from follower database
		Select(dest interface{}, query string, args ...interface{}) error

		// Query from follower database
		Query(query string, args ...interface{}) (*sql.Rows, error)

		// QueryRow executes QueryRow against follower DB
		QueryRow(query string, args ...interface{}) *sql.Row

		// QueryRowContext from sql database
		QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row

		// NamedQuery do named query on follower DB
		NamedQuery(query string, arg interface{}) (*sqlx.Rows, error)

		// GetContext from sql database
		GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error

		// SelectContext from sql database
		SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error

		// QueryContext from sql database
		QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)

		// QueryxContext queries the database and returns an *sqlx.Rows. Any placeholder parameters are replaced with supplied args.
		QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error)

		// QueryRowxContext queries the database and returns an *sqlx.Row. Any placeholder parameters are replaced with supplied args.
		QueryRowxContext(ctx context.Context, query string, args ...interface{}) *sqlx.Row

		// NamedQueryContext do named query on follower DB
		NamedQueryContext(ctx context.Context, query string, arg interface{}) (*sqlx.Rows, error)
	}

	SQL struct {
		Master
		Follower
		master             *sqlx.DB
		follower           *sqlx.DB
		timeout            time.Duration
		slowQueryThreshold float64
		engine             string
	}

	SQLs struct {
		Master             SQLConfig `yaml:"master"`
		Follower           SQLConfig `yaml:"follower"`
		Timeout            int       `yaml:"timeout"`
		SlowQueryThreshold float64   `yaml:"slow_query_threshold"`
	}

	SQLConfig struct {
		Username        string `yaml:"username"`
		Password        string `yaml:"password"`
		Host            string `yaml:"host"`
		Port            string `yaml:"port"`
		Engine          string `yaml:"engine"`
		DBName          string `yaml:"db_name"`
		Timeout         int    `yaml:"timeout"`
		MaxConnLifetime int    `yaml:"max_conn_lifetime"`
		MaxIdleTime     int    `yaml:"max_idle_time"`
		MaxOpenConn     int    `yaml:"max_open_conn"`
		MaxIdleConn     int    `yaml:"max_idle_conn"`
		ConnString      string `yaml:"conn_string"`
	}

	QueryOpts struct {
		BaseQuery         string
		Conditions        func(ctx context.Context)
		ExcludeColumns    string
		Columns           string
		OptionalTableName string // for view name
		SelectRequest     *TableRequest
		CUDRequest        *CUDConstructData
		Result            interface{}
		IsList            bool
		Trx               *sql.Tx
		executedQuery
	}

	CUDResponse struct {
		Status       bool  `json:"status"`
		RowsAffected int64 `json:"rows_affected"`
		LastInsertID int64 `json:"last_insert_id"`
		executedQuery
	}

	executedQuery struct {
		query  string
		params []interface{}
	}

	ResponseMetaData struct {
		RequestParam  interface{} `json:"request_param"`
		TotalData     int64       `json:"total_data"`
		TotalFiltered int64       `json:"total_filtered"`
	}

	SelectResponse struct {
		Data interface{} `json:"data"`
		*ResponseMetaData
	}

	TableRequest struct {
		Keyword               string        `json:"keyword,omitempty" schema:"keyword"`
		SearchColsStr         string        `json:"search_cols,omitempty" schema:"search_cols"`
		SearchCols            []string      `json:"-"`
		Page                  int           `json:"page,omitempty" schema:"page"`
		Limit                 int           `json:"limit,omitempty" schema:"limit"`
		OrderBy               string        `json:"order_by,omitempty" schema:"order_by"`
		GroupBy               string        `json:"group_by,omitempty,omitempty" schema:"group_by"`
		CreatedStart          string        `json:"date_start,omitempty" schema:"date_start"`
		CreatedEnd            string        `json:"date_end,omitempty" schema:"date_end"`
		InitiateWhere         []string      `json:"-"` // will be defined manually at each of usecase services
		InitiateWhereValues   []interface{} `json:"-"` // will be defined manually at each of usecase services
		IncludeDeleted        bool          `json:"-"`
		NotContainsDeletedCol bool          `json:"-"`
		MainTableAlias        string        `json:"-"`
		IsDeleted             string        `json:"is_deleted,omitempty" schema:"is_deleted"`
	}

	CUDConstructData struct {
		Cols       []string      `json:"cols"`
		Values     []interface{} `json:"values"`
		ColsInsert string
		BulkValues string
		BulkQuery  string
		Action     string
		TableName  string
	}

	TimeCol struct {
		CreatedAt string      `json:"created_at" db:"created_at"`
		UpdatedAt null.String `json:"updated_at" db:"updated_at"`
		DeletedAt null.String `json:"deleted_at" db:"deleted_at"`
	}

	BaseColumn[T string | int | int64] struct {
		Id T `json:"id" db:"id"`
		TimeCol
	}
)

func (req *TableRequest) SetWhereCondition(condition string, value ...interface{}) {
	req.InitiateWhere = append(req.InitiateWhere, condition)
	if len(value) > 0 && value[0] != nil {
		req.InitiateWhereValues = append(req.InitiateWhereValues, value...)
	}
}

func (req *CUDConstructData) SetValues(value interface{}) {
	req.Values = append(req.Values, value)
}

// GetGeneratedQuery - return query + params with format map[<query>]<params>
func (e *executedQuery) GetGeneratedQuery() map[string][]interface{} {
	return map[string][]interface{}{
		e.query: e.params,
	}
}
