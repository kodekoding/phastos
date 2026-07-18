# Database / Query Builder

Phastos database layer provides a SQL query builder with read/write separation, connection pooling, soft delete, transactions, prepared statement caching, and automatic query construction via reflection-based struct mapping. Built on `jmoiron/sqlx`.

## Connection

### Connect

`database.Connect()` initialises both master (read-write) and follower (read-only) connections using environment variables:

| Variable | Description | Default |
|---|---|---|
| `DATABASE_ENGINE` | `mysql`, `nrmysql`, `postgres`, `nrpostgres` | — (required) |
| `DATABASE_CONN_STRING_MASTER` | DSN for master/write DB | — (required) |
| `DATABASE_CONN_STRING_FOLLOWER` | DSN for follower/read DB | — (required) |
| `DATABASE_MAX_OPEN_CONN` | Max open connections per pool | `10` |
| `DATABASE_MAX_IDLE_CONN` | Max idle connections per pool | `2` |
| `DATABASE_CONN_MAX_LIFETIME` | Max seconds before a connection is recycled | `300` (5 min) |
| `DATABASE_CONN_MAX_IDLE_TIME` | Max seconds a connection stays idle | `45` |

NR-prefixed engines (`nrmysql`, `nrpostgres`) use New Relic instrumented drivers for automatic query tracing.

```go
import "github.com/kodekoding/phastos/v2/go/database"

db, err := database.Connect()
if err != nil {
    log.Fatal(err)
}
```

## Core Types

### ISQL Interface (`go/database/definition.go:91`)

```go
type ISQL interface {
    Master
    Follower
    GetTransaction() Transactions
    Read(ctx context.Context, opts *QueryOpts, additionalParams ...interface{}) error
    Write(ctx context.Context, opts *QueryOpts, isSoftDelete ...bool) (*CUDResponse, error)
    CachedRebind(query string) string
}
```

### QueryOpts (`go/database/definition.go:131`)

```go
type QueryOpts struct {
    BaseQuery     string
    Conditions    func(ctx context.Context)
    Result        interface{}
    IsList        bool
    SelectRequest *TableRequest
    CUDRequest    *CUDConstructData
    Trx           *sqlx.Tx
    LockingType   string
    UseMaster     bool
    // ... internal fields
}
```

### TableRequest (`go/database/definition.go:172`)

```go
type TableRequest struct {
    Keyword               string        // search keyword
    SearchColsStr         string        // comma-separated column names for keyword search
    Page                  int           // page number (1-based)
    Limit                 int           // page size
    OrderBy               string        // ORDER BY clause
    GroupBy               string        // GROUP BY clause
    CreatedStart          string        // date range start (YYYY-MM-DD)
    CreatedEnd            string        // date range end (YYYY-MM-DD)
    CustomDateColFilter   string        // custom date column name (default: created_at)
    InitiateWhere         []string      // raw WHERE conditions
    InitiateWhereValues   []interface{} // values for InitiateWhere placeholders
    IncludeDeleted        bool          // include soft-deleted rows
    MainTableAlias        string        // alias prefix for column qualifiers
    IsDeleted             string        // "1" to filter only deleted rows
}
```

### CUDConstructData (`go/database/definition.go:192`)

```go
type CUDConstructData struct {
    Cols       []string
    Values     []interface{}
    ColsInsert string
    BulkValues string
    BulkQuery  string
    Action     string
    TableName  string
}
```

### CUDResponse (`go/database/definition.go:148`)

```go
type CUDResponse struct {
    Status       bool   `json:"status"`
    RowsAffected int64  `json:"rows_affected"`
    LastInsertID int64  `json:"last_insert_id"`
    Message      string `json:"message,omitempty"`
}
```

### SelectResponse (`go/database/definition.go:167`)

```go
type SelectResponse struct {
    Data interface{} `json:"data"`
    *ResponseMetaData
}

type ResponseMetaData struct {
    RequestParam  interface{} `json:"request_param"`
    TotalData     int64       `json:"total_data"`
    TotalFiltered int64       `json:"total_filtered"`
}
```

## Read Operations

`db.Read(ctx, &database.QueryOpts{...})` returns results into `Result` by reference. The method:

1. Calls `Conditions(ctx)` if set (for dynamic query parts before execution)
2. Generates add-on clauses (WHERE, GROUP BY, ORDER BY, LIMIT) from `SelectRequest`
3. Rebind-s `?` placeholders to the engine-appropriate format
4. Executes via prepared statement cache (LRU, max 500 entries) or falls back to direct DB

### Single Row

```go
type User struct {
    Id   int64  `json:"id" db:"id"`
    Name string `json:"name" db:"name"`
}

var user User
err := db.Read(ctx, &database.QueryOpts{
    BaseQuery: "SELECT id, name FROM users",
    SelectRequest: &database.TableRequest{
        InitiateWhere:       []string{"id = ?"},
        InitiateWhereValues: []interface{}{42},
    },
    Result: &user,
})
// user.Id == 42, user.Name == "Alice"
```

#### Pagination with `Page` and `Limit`

```go
var users []User
err := db.Read(ctx, &database.QueryOpts{
    BaseQuery: "SELECT id, name FROM users",
    SelectRequest: &database.TableRequest{
        Page:    1,
        Limit:   20,
        OrderBy: "created_at DESC",
    },
    Result: &users,
    IsList: true,
})
// LIMIT/OFFSET generated automatically (engine-aware: postgres uses LIMIT ? OFFSET ?, mysql uses LIMIT ?,?)
```

#### Keyword Search

```go
var users []User
req := &database.TableRequest{
    Keyword:       "alice",
    SearchColsStr: "name,email",
    Page:          1,
    Limit:         10,
}
err := db.Read(ctx, &database.QueryOpts{
    BaseQuery:     "SELECT id, name, email FROM users",
    SelectRequest: req,
    Result:        &users,
    IsList:        true,
})
// Generates: WHERE (name LIKE ? OR email LIKE ?) with LIKE params as %alice%
```

#### Date Range Filter

```go
req := &database.TableRequest{
    CreatedStart: "2025-01-01",
    CreatedEnd:   "2025-12-31",
    Page:         1,
    Limit:        10,
}
// Generates: WHERE created_at >= ? AND created_at <= ?
// MySQL uses STR_TO_DATE / DATE_FORMAT for comparison
```

#### Raw Where Conditions

```go
req := &database.TableRequest{}
req.SetWhereCondition("status = ?", "active")
req.SetWhereCondition("deleted_at IS NULL")
// Generates: WHERE status = ? AND deleted_at IS NULL
```

#### Locking (within transactions)

```go
var user User
err := db.Read(ctx, &database.QueryOpts{
    BaseQuery: "SELECT id, balance FROM users WHERE id = ?",
    Result:    &user,
    Trx:       trx,
    LockingType: database.LockUpdate, // " FOR UPDATE"
})
```

#### Force Master Read

```go
var user User
err := db.Read(ctx, &database.QueryOpts{
    BaseQuery: "SELECT id, name FROM users WHERE id = ?",
    Result:    &user,
    UseMaster: true,
    SelectRequest: &database.TableRequest{
        InitiateWhere:       []string{"id = ?"},
        InitiateWhereValues: []interface{}{42},
    },
})
```

### List with Count Query

```go
// Manual count + list pattern:
totalData, totalFiltered, err := commonRepo.Count(ctx, req, "users")
req := &database.TableRequest{
    Page:    1,
    Limit:   10,
    OrderBy: "created_at DESC",
}
var users []User
err := db.Read(ctx, &database.QueryOpts{
    BaseQuery:     "SELECT id, name FROM users",
    SelectRequest: req,
    Result:        &users,
    IsList:        true,
})

response := database.GetSelectResponse()
response.Data = users
response.ResponseMetaData = &database.ResponseMetaData{
    TotalData:     int64(totalData),
    TotalFiltered: int64(totalFiltered),
}
```

### Object Pool Helpers

Use pool helpers to reduce heap allocations in hot paths:

```go
qOpts := database.GetQueryOpts()
defer database.PutQueryOpts(qOpts)

req := database.GetTableRequest()
defer database.PutTableRequest(req)

resp := database.GetSelectResponse()
defer database.PutSelectResponse(resp)
```

## Write Operations

`db.Write(ctx, opts)` generates queries from `CUDConstructData` and executes them on the master DB.

### Action Constants

| Constant | Value | Description |
|---|---|---|
| `database.ActionInsert` | `"insert"` | Single row INSERT |
| `database.ActionBulkInsert` | `"bulk_insert"` | Multi-row INSERT |
| `database.ActionUpdate` | `"update"` | UPDATE with condition |
| `database.ActionUpdateById` | `"update_by_id"` | UPDATE WHERE id = ? |
| `database.ActionUpsert` | `"upsert"` | INSERT ... ON DUPLICATE KEY UPDATE |
| `database.ActionDelete` | `"delete"` | Soft/hard delete with condition |
| `database.ActionDeleteById` | `"delete_by_id"` | Soft/hard delete WHERE id = ? |
| `database.ActionBulkUpdate` | `"bulk_update"` | UPDATE with JOIN on bulk values |

### Insert

```go
result, err := db.Write(ctx, &database.QueryOpts{
    CUDRequest: &database.CUDConstructData{
        Action:    database.ActionInsert,
        TableName: "users",
        Cols:      []string{"name", "email"},
        Values:    []interface{}{"Alice", "alice@example.com"},
    },
})
// result.LastInsertID = 1
// result.RowsAffected = 1
// result.Status = true
```

PostgreSQL appends `RETURNING id` so `LastInsertID` is populated without an extra query.

### Insert returning into struct

```go
type User struct {
    Id    int64  `json:"id" db:"id"`
    Name  string `json:"name" db:"name"`
    Email string `json:"email" db:"email"`
}
user := User{Name: "Alice", Email: "alice@example.com"}

cols, vals := helper.ConstructColNameAndValue(ctx, &user)
result, err := db.Write(ctx, &database.QueryOpts{
    CUDRequest: &database.CUDConstructData{
        Action:    database.ActionInsert,
        TableName: "users",
        Cols:      cols,
        Values:    vals,
    },
    Result: &user, // populated with generated values (MySQL: LastInsertId)
})
```

### Update by ID

```go
result, err := db.Write(ctx, &database.QueryOpts{
    CUDRequest: &database.CUDConstructData{
        Action:    database.ActionUpdateById,
        TableName: "users",
        Cols:      []string{"name = ?", "email = ?"},
        Values:    []interface{}{"Alice Updated", "new@example.com", 42},
    },
})
```

The last value in `Values` is the ID for the `WHERE id = ?` clause.

### Update with Condition

```go
result, err := db.Write(ctx, &database.QueryOpts{
    CUDRequest: &database.CUDConstructData{
        Action:    database.ActionUpdate,
        TableName: "users",
        Cols:      []string{"status = ?"},
        Values:    []interface{}{"inactive"},
    },
    SelectRequest: &database.TableRequest{
        InitiateWhere:       []string{"email = ?"},
        InitiateWhereValues: []interface{}{"old@example.com"},
    },
})
// Generates: UPDATE users SET status = ? WHERE email = ?
```

### Delete (Soft Delete)

Soft delete is enabled by default — sets `deleted_at = now()` instead of removing rows:

```go
result, err := db.Write(ctx, &database.QueryOpts{
    CUDRequest: &database.CUDConstructData{
        Action:    database.ActionDeleteById,
        TableName: "users",
        Values:    []interface{}{42},
    },
}, true) // second arg = isSoftDelete
// Generates: UPDATE users SET deleted_at = now() WHERE id = ?
```

For hard delete, pass `false`:

```go
result, err := db.Write(ctx, &database.QueryOpts{
    CUDRequest: &database.CUDConstructData{
        Action:    database.ActionDeleteById,
        TableName: "users",
        Values:    []interface{}{42},
    },
}, false)
// Generates: DELETE FROM users WHERE id = ?
```

### Delete with Condition

```go
result, err := db.Write(ctx, &database.QueryOpts{
    CUDRequest: &database.CUDConstructData{
        Action:    database.ActionDelete,
        TableName: "users",
    },
    SelectRequest: &database.TableRequest{
        InitiateWhere:       []string{"status = ?"},
        InitiateWhereValues: []interface{}{"inactive"},
    },
}, true)
// Generates: UPDATE users SET deleted_at = now() WHERE status = ?
```

### Bulk Insert

```go
users := []User{
    {Name: "Alice", Email: "alice@x.com"},
    {Name: "Bob", Email: "bob@x.com"},
}
cudData, err := helper.ConstructColNameAndValueBulk(ctx, &users)
cudData.Action = database.ActionBulkInsert
cudData.TableName = "users"

result, err := db.Write(ctx, &database.QueryOpts{
    CUDRequest: cudData,
})
// Generates: INSERT INTO users (name, email) VALUES (?,?),(?,?)
```

### Upsert

```go
cudData := helper.ConstructColNameAndValueForUpdate(ctx, &user)
cudData.Action = database.ActionUpsert
cudData.TableName = "users"
cudData.ColsInsert = "name, email"
cudData.Values = append(cudData.Values, cudData.Values...) // dup values for ON DUPLICATE KEY UPDATE
// Generates: INSERT INTO users (name, email) VALUES (?,?) ON DUPLICATE KEY UPDATE name = ?, email = ?
```

### Soft-Delete Filtering

By default, `Read`-generated queries exclude soft-deleted rows (`deleted_at IS NULL` for Postgres, `deleted_at IS NULL OR deleted_at = '0000-00-00...'` for MySQL). Include them:

```go
req := &database.TableRequest{
    IncludeDeleted:             true,
    NotContainsDeletedCol: true, // skip the deleted_at filter entirely
}
```

## Transactions

```go
trx, err := db.Begin()              // returns *sqlx.Tx
if err != nil {
    return err
}
defer db.Finish(trx, &err)          // Commit on nil, Rollback on error

// Use trx in QueryOpts
var user User
err = db.Read(ctx, &database.QueryOpts{
    BaseQuery: "SELECT id, balance FROM users WHERE id = ?",
    SelectRequest: &database.TableRequest{
        InitiateWhere:       []string{"id = ?"},
        InitiateWhereValues: []interface{}{42},
    },
    Result:      &user,
    Trx:         trx,
    LockingType: database.LockUpdate,
})
if err != nil {
    return err
}

result, err := db.Write(ctx, &database.QueryOpts{
    CUDRequest: &database.CUDConstructData{
        Action:    database.ActionUpdateById,
        TableName: "users",
        Cols:      []string{"balance = ?"},
        Values:    []interface{}{user.Balance - 100, 42},
    },
    Trx: trx,
})
if err != nil {
    return err
}
```

### Transaction Methods

```go
// GetTransaction returns a Transactions interface:
trx, err := db.GetTransaction().Begin()
defer db.GetTransaction().Finish(trx, &err)
```

### Full Transaction Pattern

```go
func TransferBalance(ctx context.Context, db database.ISQL, from, to int64, amount float64) (err error) {
    trx, err := db.Begin()
    if err != nil {
        return err
    }
    defer db.Finish(trx, &err)

    // ... read + write operations using trx ...
    return nil
}
```

## CRUD Helpers

### action.Base — Combined Read + Write

`database/action` package provides pre-built CRUD implementations:

```go
import "github.com/kodekoding/phastos/v2/go/database/action"

base := action.NewBase(db, "users") // table: users, softDelete: true
// or:
base := action.NewBase(db, "users", false) // hard delete
```

#### ReadRepo methods (via `action.NewBaseRead`)

```go
// Get list with pagination, search, ordering
opts := &database.QueryOpts{
    BaseQuery:     "SELECT id, name, email FROM users",
    SelectRequest: &database.TableRequest{Page: 1, Limit: 10},
    Result:        &users,
    IsList:        true,
}
err := base.GetList(ctx, opts)

// Get single row by ID
var user User
err := base.GetDetailById(ctx, &user, 42)

// Get single row with custom query
opts := &database.QueryOpts{
    BaseQuery: "SELECT id, name FROM users",
    SelectRequest: &database.TableRequest{
        InitiateWhere:       []string{"email = ?"},
        InitiateWhereValues: []interface{}{"alice@example.com"},
    },
    Result: &user,
}
err := base.GetDetail(ctx, opts)
```

#### WriteRepo methods (via `action.NewBaseWrite`)

```go
// Insert (struct fields mapped automatically via reflection)
user := User{Name: "Alice", Email: "alice@example.com"}
result, err := base.Insert(ctx, &user)

// Update by ID
user.Name = "Alice Updated"
result, err := base.UpdateById(ctx, &user, 42)

// Update with custom condition
result, err := base.Update(ctx, &user, map[string]interface{}{
    "email = ?": "old@example.com",
})

// Upsert (condition for duplicate check, vals for ON DUPLICATE KEY UPDATE)
result, err := base.Upsert(ctx, &user, map[string]interface{}{
    "email": "alice@example.com",
})

// Delete by ID (soft delete by default)
result, err := base.DeleteById(ctx, 42)

// Delete with condition
result, err := base.Delete(ctx, map[string]interface{}{
    "status = ?": "inactive",
})

// Bulk Insert
users := []User{{Name: "A"}, {Name: "B"}}
result, err := base.BulkInsert(ctx, &users)

// Bulk Update
result, err := base.BulkUpdate(ctx, &users, map[string][]interface{}{
    "id": {1, 2},
})

// With transaction
trx, _ := db.Begin()
defer db.Finish(trx, &err)
result, err := base.Insert(ctx, &user, trx)
```

### UsecaseCRUD Interface

```go
type UsecaseCRUD interface {
    GetList(ctx context.Context, requestData interface{}) (*database.SelectResponse, error)
    GetDetailById(ctx context.Context, id interface{}) (*database.SelectResponse, error)
    Insert(ctx context.Context, data interface{}) (*database.CUDResponse, error)
    Update(ctx context.Context, data interface{}) (*database.CUDResponse, error)
    Delete(ctx context.Context, id interface{}) (*database.CUDResponse, error)
}
```

### RepoCRUD Interface

```go
type RepoCRUD interface {
    ReadRepo     // GetList, GetDetail, GetDetailById, Count
    WriteRepo    // Insert, BulkInsert, BulkUpdate, Update, Upsert, UpdateById, Delete, DeleteById
}
```

## Error Handling

### sendNilResponse

The internal `sendNilResponse` function (`go/database/sql.go:1064`) maps database errors to HTTP status codes:

- `no rows in result set` → returns `nil, nil` (empty result)
- PostgreSQL unique violation (`code 23505`) → HTTP **409** Conflict
- PostgreSQL check violation (`code 23514`) → HTTP **422** Unprocessable Entity
- Other errors → HTTP **500**

```go
err := db.Read(ctx, opts)
// err nil + result nil → no rows found
// err *custerr with Code 409/422 → constraint violation
// err *custerr with Code 500 → internal error
```

### Retrieving Generated Query

```go
result, err := db.Write(ctx, opts)
if result != nil {
    queryMap := result.GetGeneratedQuery() // map[string][]interface{} — map[<query>]<params>
}
```

### Slow Query Detection

Set `DATABASE_SLOW_QUERY_WARNING=true` and `DATABASE_SLOW_QUERY_THRESHOLD` (seconds, default `1`) to receive platform notifications (slack, etc.) for slow queries.

```bash
export DATABASE_SLOW_QUERY_WARNING=true
export DATABASE_SLOW_QUERY_THRESHOLD=2.0
```

## Engine Constants

```go
database.MySQLEngine      // "mysql"
database.NRMySQLEngine    // "nrmysql" (New Relic instrumented)
database.PostgresEngine   // "postgres"
database.NRPostgresEngine // "nrpostgres" (New Relic instrumented)
```

Engine-aware behavior:
- Placeholder rebinding (`?` → `$1`, `$2` for PG)
- `RETURNING id` appended for PG INSERT/UPDATE
- `FOR SHARE` vs `LOCK IN SHARE MODE` for locking
- `LIMIT ? OFFSET ?` (PG) vs `LIMIT ?,?` (MySQL) for pagination
- Date format functions differ (`STR_TO_DATE`/`DATE_FORMAT` for MySQL)

## Complete Usage Example

```go
package main

import (
    "context"
    "log"

    "github.com/kodekoding/phastos/v2/go/database"
    "github.com/kodekoding/phastos/v2/go/database/action"
)

type Product struct {
    Id    int64  `json:"id" db:"id"`
    Name  string `json:"name" db:"name"`
    Price int64  `json:"price" db:"price"`
}

func main() {
    db, err := database.Connect()
    if err != nil {
        log.Fatal(err)
    }

    base := action.NewBase(db, "products")
    ctx := context.Background()

    // Insert
    product := Product{Name: "Widget", Price: 9900}
    result, err := base.Insert(ctx, &product)
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Inserted: id=%d, affected=%d", result.LastInsertID, result.RowsAffected)

    // List with search
    var products []Product
    searchReq := &database.TableRequest{
        Keyword:       "Widget",
        SearchColsStr: "name",
        Page:          1,
        Limit:         10,
    }
    err = db.Read(ctx, &database.QueryOpts{
        BaseQuery:     "SELECT id, name, price FROM products",
        SelectRequest: searchReq,
        Result:        &products,
        IsList:        true,
    })
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Found %d products", len(products))
}
```

## Pool Utilities

Object pools are available for heap-allocation reduction in hot request paths:

```go
qOpts := database.GetQueryOpts()
defer database.PutQueryOpts(qOpts)

req := database.GetTableRequest()
defer database.PutTableRequest(req)

resp := database.GetSelectResponse()
defer database.PutSelectResponse(resp)

cudResp := database.GetCUDResponse()
defer database.PutCUDResponse(cudResp)

cudData := database.GetCUDConstructData()
defer database.PutCUDConstructData(cudData)
```

## Internal Caches

- **Prepared statement cache** — LRU bounded (500 entries each for reads and writes). Evicted on capacity; close is deferred to connection recycling.
- **Rebind cache** — bounded sync.Map (2000 entries), caches `?` → `$N` rebinding per query string.
- **DeleteById query cache** — `sync.Map` per `(tableName, isSoftDelete)`, stores pre-computed rebound queries.
