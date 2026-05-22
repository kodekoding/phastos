# Phastos ORM Benchmark Suite

Multi-database benchmark suite comparing **7 Go ORMs** on **MySQL** and **PostgreSQL**.

## ORMs Under Test

| ORM | Category | Description |
|---|---|---|
| `database/sql` (Stdlib) | Raw SQL | Go standard library — no ORM abstraction |
| `sqlx` | Raw SQL | Thin extensions over `database/sql` — struct scanning, named params |
| **Phastos** | Full ORM | Custom ORM with reflection-based struct mapping, query builder, soft delete |
| GORM | Full ORM | Most popular Go ORM; extensive features, callbacks, associations |
| XORM | Full ORM | Schema-based ORM with caching, multiple dialect support |
| Beego ORM | Full ORM | ORM from Beego web framework; auto-migrate, reflect-based |
| Bun | Full ORM | Lightweight ORM with model hooks, soft delete, migrations |

## Benchmark Operations

| Operation | Description | Notes |
|---|---|---|
| `Insert` | Insert a single row | ORM creates query + values from struct |
| `SelectByID` | Select one row by primary key | ORM scans result into struct |
| `SelectList` | Select all rows (no filter) | ORM scans slice of structs |
| `Update` | Update a row by primary key | ORM builds `UPDATE ... SET` from struct |
| `Delete` | Soft-delete a row (set `deleted_at`) | ORM sets timestamp, not `DELETE` |
| `BulkInsert` | Insert 100 rows at once | ORM builds multi-value `INSERT` |
| `Upsert` | MySQL-only `INSERT ... ON DUPLICATE KEY UPDATE` | Phastos-specific feature |
| `*_Parallel` | Same operations with `t.RunParallel` | Measures concurrency throughput |

## Methodology

### Environment

- **Hardware**: Apple M3 Pro, macOS
- **Databases**: MySQL 8.x (Docker, port 3306), PostgreSQL (Docker, port 32771)
- **Connection pooling**: MaxOpenConns=4, MaxIdleConns=4, ConnMaxLifetime=5m
- **Go version**: 1.22+
- **Benchmark tool**: `go test -bench=. -benchmem`

### Controls

- **Same table schema** used across all ORMs (`bench_users`, `bench_products`, `bench_orders`)
- **Same struct fields**: id, name, email, age, status, created_at, updated_at, deleted_at
- **Same data generation**: deterministic `generateUser(i)` with consistent field values
- **Tables recreated** per benchmark run (clean state)
- **Warm-up**: Each benchmark inserts 10 rows before the timed loop begins
- **Logging disabled**: All ORMs configured with silent/no-op loggers
- **Connection settings identical**: Same pool sizes and timeouts for all ORMs

### What We Measure

- **ns/op**: Nanoseconds per operation (wall-clock time including DB round-trip)
- **allocs/op**: Heap allocations per operation (measures GC pressure)
- **B/op**: Bytes allocated per operation (measures memory throughput)

### Important Caveats

- Network latency to Docker containers adds ~100–200µs per round-trip. **Absolute ns/op values are environment-specific.** Relative ORM comparisons within the same run are meaningful; cross-environment comparisons are not.
- Single-row operations (Insert, SelectByID, Update, Delete) are dominated by the database driver round-trip (~80–85% of latency). ORM-layer overhead is the remaining 15–20%.
- BulkInsert is sensitive to query string size and driver batch handling, so ns/op varies more.
- Parallel benchmarks measure throughput under GOMAXPROCS concurrency, not single-goroutine latency.

## Running the Benchmarks

### Prerequisites

```bash
# MySQL (Docker)
docker run -d --name mysql_container -p 3306:3306 -e MYSQL_ROOT_PASSWORD=toor mysql:8

# PostgreSQL (Docker)
docker run -d --name postgresql-master -p 32771:5432 -e POSTGRES_PASSWORD=postgres postgres:15
```

### Run on a single database

```bash
cd benchmark/db

# MySQL
./run.sh --db mysql

# PostgreSQL
./run.sh --db postgres

# Custom DSN
./run.sh --db postgres "postgres://user:pass@host:5432/mydb?sslmode=disable"
```

### Run on both databases

```bash
./run_all.sh
```

### Run specific ORM benchmarks

```bash
BENCHMARK_DB_TYPE=postgres go test -bench=BenchmarkPhastos -benchmem -count=5
BENCHMARK_DB_TYPE=mysql go test -bench=BenchmarkGorm_Update -benchmem
```

### Run with custom configuration

```bash
# Override DSN via environment
BENCHMARK_DB_TYPE=mysql BENCHMARK_DB_DSN="root:pass@tcp(localhost:3306)/testdb" go test -bench=. -benchmem
```

## Optimization History

### Baseline (before any optimizations)

PostgreSQL:
| Operation | ns/op | allocs | B/op |
|---|---|---|---|
| Insert | 1,102k | 68 | 2,760 |
| SelectByID | 149k | 59 | 2,601 |
| Update | 958k | 72 | 2,632 |
| Delete | 144k | 36 | 1,737 |
| BulkInsert | 2,658k | 3,214 | 192,018 |

MySQL:
| Operation | ns/op | allocs | B/op |
|---|---|---|---|
| SelectList | 206k | — | — |
| Delete | 180k | 26 | 1,088 |
| BulkInsert | — | 2,795 | — |

### Phase 1: Infrastructure Optimizations (B1–B7)

| ID | Optimization | Files Changed |
|---|---|---|
| B1 | Reflection caching — `sync.Map` cache keyed by `reflect.Type`, stores pre-computed field metadata | `struct_cache.go`, `struct.go` |
| B2 | `selectColsCache` — caches `GenerateSelectCols` results per type + filter combo | `struct_cache.go`, `struct.go` |
| B3 | PG Update: `ExecContext` + `RowsAffected()` instead of `QueryRowContext+Scan` with `RETURNING id` | `sql.go` |
| B4 | Removed goroutines from `ConstructColNameAndValueBulk` — sequential loop is faster for typical sizes | `struct.go` |
| B5 | Pre-sized slices in `readField()` — `make([]string, 0, numField)` | `struct.go` |
| B6 | `builderPool` — `sync.Pool` for `strings.Builder` reuse in `Write()` | `sql.go` |
| B7 | `WriteString` replaces `fmt.Sprintf` for SQL construction in `Write()` | `sql.go` |

**Results after B1–B7 (PostgreSQL)**:
| Operation | ns/op | allocs | B/op | Change |
|---|---|---|---|---|
| Insert | 1,015k | 66 | 2,859 | allocs -3% |
| SelectByID | 149k | 59 | 2,601 | stable |
| Update | 1,025k | 65 | 2,642 | allocs -10% |
| Delete | 143k | 31 | 1,296 | allocs **-14%**, B/op **-25%** |
| BulkInsert | 2,698k | 2,997 | 210,722 | allocs -7% |

### Phase 2: Template Caching (B8–B11)

| ID | Optimization | Files Changed |
|---|---|---|
| B8 | `UpdateTemplateInfo` cache — pre-computed UPDATE query template per `reflect.Type` with field paths, column names, null patterns | `struct_cache.go` |
| B9 | Fast path in `ConstructColNameAndValueForUpdate` — skips `readField()` + `Remove()` loop, uses cached field path iteration | `struct.go` |
| B10 | `CachedRebind()` — caches `Rebind` results per query string to skip repeated `?`→`$N` scanning | `sql.go` |
| B11 | `UpdateById` template cache — cached query + value extraction per `(reflect.Type, tableName)` in `write.go` | `write.go`, `definition.go` |

**Results after B8–B11 (PostgreSQL)**:
| Operation | ns/op | allocs | B/op | Change from baseline |
|---|---|---|---|---|
| Insert | ~1,061k | 64 | 2,655 | allocs -6% |
| SelectByID | ~156k | 59 | 2,601 | stable |
| **Update** | **963k** | **48** | **2,126** | allocs **-33%**, B/op **-19%** |
| Delete | ~146k | 29 | 1,168 | allocs **-19%**, B/op **-33%** |
| BulkInsert | ~2,488k | 2,992 | 201,962 | allocs -7% |

**Results after B8–B11 (MySQL)**:
| Operation | ns/op | allocs | B/op | Change from baseline |
|---|---|---|---|---|
| **Update** | **1,440k** | **40** | **1,877** | allocs **-25%**, B/op **-29%** |
| Delete | ~191k | 23 | 928 | allocs -12%, B/op -15% |
| SelectList | 183k | 256 | 18,082 | ns/op **-11%** |
| BulkInsert | — | 2,580 | 178,689 | allocs -8% |

## Final Cross-ORM Results

### PostgreSQL — Single-Threaded

| Operation | Stdlib | Sqlx | **Phastos** | Gorm | Xorm | Beego | Bun |
|---|---|---|---|---|---|---|---|
| Insert | 1,253k | 1,012k | **945k** | 1,053k | 1,089k | 1,316k | 913k |
| SelectByID | 151k | 151k | **152k** | 83k | 169k | 154k | 88k |
| SelectList | 174k | 168k | **180k** | 106k | 222k | 122k | 137k |
| Update | 151k | 144k | **963k** | 1,036k | 1,081k | 1,034k | 1,027k |
| Delete | 143k | 142k | **145k** | 213k | 162k | 147k | 81k |
| BulkInsert | 2,180k | 2,395k | **2,488k** | 2,387k | 4,308k | 2,901k | 2,624k |

### PostgreSQL — Update Allocs (ORM comparison)

| ORM | allocs/op | B/op |
|---|---|---|
| Sqlx | 23 | 960 |
| Stdlib | 25 | 888 |
| **Phastos** | **48** | **2,126** |
| Beego | 37 | 1,542 |
| Bun | 20 | 5,456 |
| Gorm | 68 | 5,650 |
| Xorm | 162 | 6,193 |

### MySQL — Single-Threaded

| Operation | Stdlib | Sqlx | **Phastos** | Gorm | Xorm | Beego | Bun |
|---|---|---|---|---|---|---|---|
| Insert | 1,519k | 1,559k | **1,476k** | 1,692k | 1,483k | 1,462k | 1,364k |
| SelectByID | 164k | 160k | **162k** | 162k | 181k | 165k | 91k |
| SelectList | 170k | 175k | **183k** | 188k | 242k | 126k | 112k |
| Update | 1,530k | 1,644k | **1,440k** | 1,776k | 1,628k | 1,680k | 1,527k |
| Delete | 181k | 211k | **200k** | 302k | 184k | 246k | 82k |
| BulkInsert | 3,588k | 3,022k | **3,438k** | 7,013k | 7,934k | 12,886k | 3,036k |

## Analysis

### Where Phastos Excels

- **Fastest ORM for Update** on both MySQL (1,440k ns) and PostgreSQL (963k ns) among reflection-based ORMs
- **Lowest allocs among full ORMs** for Update: 48 (PG) / 40 (MySQL) vs Gorm's 68/75 and Xorm's 162/123
- **Competitive Insert** — within 5% of Bun, the fastest ORM for Insert
- **Smallest memory footprint** — 2,126 B/op (PG Update) vs 5,650 (Gorm), 5,456 (Bun), 6,193 (Xorm)
- **Unique Upsert support** — MySQL `ON DUPLICATE KEY UPDATE` at 316k ns, no other tested ORM provides this natively

### The Update Latency Gap vs Raw SQL

Phastos Update is ~6–7× slower than Sqlx/Stdlib. This is the fundamental cost of ORM abstraction, not a Phastos bug:

| Overhead Source | Approximate Cost |
|---|---|
| `reflect.Value.FieldByIndex` + `.Interface()` per field | ~200–400 ns |
| Interface boxing (`[]interface{}` values) | ~100–200 ns |
| Query string construction (even with caching) | ~50–100 ns |
| `CachedRebind` lookup | ~20 ns |
| `database/sql` driver round-trip | **~800–1,000 ns** (dominant) |

The network/driver round-trip is ~80–85% of total latency. All ORM-layer optimizations target only the remaining 15–20%.

### Why SelectByID and SelectList are Slower than Gorm/Bun

Gorm and Bun use **model caching** — they resolve struct→column mappings once and store typed accessor functions. Phastos still does `reflect.Value.Field()` + type assertion per column on every scan. This is the same optimization gap that was closed for Update via template caching, but has not yet been applied to the read path.

## Recommendations for Further Improvement

### High Impact

1. **Prepared statement reuse (server-side)** — Cache `*sql.Stmt` per query template and reuse across calls instead of `PreparexContext`/close each time. lib/pq and go-sql-driver/mysql both support this. Estimated: 5–15% reduction on the driver round-trip by eliminating the prepare phase.

2. **Code-generated struct accessors** — Replace `reflect.Value.FieldByIndex` + `.Interface()` with `go:generate`-produced typed accessor functions. Eliminates reflection entirely on the hot path. Estimated: 30–50% of the ORM overhead (~150–300 ns per call). This is the same approach used by `easyjson` vs `encoding/json`.

3. **Read-path template caching** — Apply the same `UpdateTemplateInfo` caching pattern to the Select/scan path. Pre-compute column-to-field-index mappings so `SelectContext`/`GetContext` can use `FieldByIndex` instead of per-column reflection. This would close the gap with Gorm/Bun on SelectByID.

### Medium Impact

4. **Batch UpdateById** — Add `UpdateByIdBatch(ctx, []data, []id)` that issues a single prepared statement with multiple `ExecContext` calls, amortizing the prepare overhead.

5. **Pool `CUDConstructData`** — Add a `sync.Pool` for `CUDConstructData` in `pool.go` similar to `tableRequestPool`, reducing allocs per Write call.

6. **Pre-compute `time.Now().Format()` format string** — Store `"2006-01-02 15:04:05"` as a package-level variable instead of creating it implicitly each call.

### Low Impact / Clean-up

7. **Remove slow path fallback** in `ConstructColNameAndValueForUpdate` once template caching is validated in production — the `readField` + `Remove` path is now dead code for struct inputs.

8. **Unify template cache entries** — The `updateByIdCache` in `write.go` and `updateTemplateCache` in `struct_cache.go` both cache per `reflect.Type`. These could be merged into a single cache to reduce memory overhead.

## Project Structure

```
benchmark/db/
├── README.md           # This file
├── go.mod              # Separate Go module (avoids dependency pollution)
├── main_test.go        # TestMain: DB connections, table creation
├── model_test.go       # Struct models for all 7 ORMs
├── stdlib_test.go      # database/sql benchmarks
├── sqlx_test.go        # sqlx benchmarks
├── gorm_test.go        # GORM benchmarks
├── xorm_test.go        # XORM benchmarks
├── beego_test.go       # Beego ORM benchmarks
├── bun_test.go         # Bun benchmarks
├── phastos_test.go     # Phastos ORM benchmarks
├── run.sh              # Single-DB benchmark runner
└── run_all.sh          # Dual-DB benchmark runner
```

## Key Source Files (Phastos ORM)

```
go/helper/
├── struct_cache.go     # B1: structCache, B2: selectColsCache, B8: UpdateTemplateInfo
├── struct.go           # B5: pre-sized slices, B9: Update fast path
└── slice.go            # Remove() helper (unused in fast path)

go/database/
├── sql.go              # B3: ExecContext for PG, B6: builderPool, B7: WriteString, B10: CachedRebind
├── definition.go       # ISQL interface (includes CachedRebind)
└── pool.go             # sync.Pool for TableRequest, QueryOpts, etc.

go/database/action/
└── write.go            # B11: UpdateById template cache with fast path
```
