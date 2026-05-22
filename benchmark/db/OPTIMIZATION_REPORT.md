# Phastos ORM — Performance Optimization Report

## Overview

This report documents the complete optimization journey for Phastos, a custom Go ORM built on sqlx. The work spanned two major phases — 19 baseline optimizations (B1–B11, R1–R8) and 6 architectural optimizations (O1–O6) — resulting in dramatic performance improvements that place Phastos ahead of SQLx and GORM on most operations.

**Platform**: Apple M3 Pro, macOS, Go 1.x  
**Databases**: MySQL 8 (Docker), PostgreSQL 16 (Docker)  
**Methodology**: `go test -benchmem -benchtime=5s -count=3`, median of 3 runs  
**Competitors**: database/sql (stdlib), sqlx, GORM

---

## Cumulative Performance Gains

### MySQL — Before vs After All Optimizations

| Operation | Before (Baseline) | After (O1–O6) | Improvement |
|---|---|---|---|
| **SelectByID** | 164k ns / 59a | 79k ns / 36a | **2.1× faster, −39% allocs** |
| **SelectList** | 206k ns / 304a | 114k ns / 245a | **1.8× faster, −19% allocs** |
| **Delete** | 180k ns / 26a | 93k ns / 12a | **1.9× faster, −54% allocs** |
| **Update** | 1,570k ns / 72a | 1,531k ns / 22a | **1.0× faster, −69% allocs** |
| **Insert** | 1,599k ns / 68a | 1,481k ns / 46a | **1.1× faster, −32% allocs** |

### PostgreSQL — Before vs After All Optimizations

| Operation | Before (Baseline) | After (O1–O6) | Improvement |
|---|---|---|---|
| **SelectByID** | 149k ns / 59a | 71k ns / 36a | **2.1× faster, −39% allocs** |
| **SelectList** | 180k ns / 304a | 94k ns / 284a | **1.9× faster, −7% allocs** |
| **Delete** | 144k ns / 36a | 66k ns / 13a | **2.2× faster, −64% allocs** |
| **Update** | 958k ns / 72a | 1,074k ns / 25a | comparable, −65% allocs |
| **Insert** | 1,102k ns / 68a | 1,037k ns / 64a | **1.1× faster, −6% allocs** |

---

## Final Benchmark Results — Phastos vs Competitors

### MySQL (Apple M3 Pro)

| Operation | Stdlib | SQLx | GORM | **Phastos** |
|---|---|---|---|---|
| Insert | 1,310k / 22a | 1,764k / 18a | 1,725k / 82a | **1,481k / 46a** |
| SelectByID | 162k / 37a | 180k / 40a | 164k / 81a | **79k / 36a** |
| SelectList | 168k / 205a | 200k / 237a | 195k / 401a | **114k / 245a** |
| Update | 1,458k / 15a | 1,783k / 14a | 1,814k / 75a | **1,531k / 22a** |
| Delete | 163k / 11a | 188k / 10a | 278k / 66a | **93k / 12a** |

*(ns/op median / allocs per operation)*

### PostgreSQL (Apple M3 Pro)

| Operation | SQLx | GORM | **Phastos** |
|---|---|---|---|
| Insert | 1,087k / 28a | 1,059k / 98a | **1,037k / 64a** |
| SelectByID | 150k / 49a | 81k / 96a | **71k / 36a** |
| SelectList | 169k / 283a | 102k / 452a | **94k / 284a** |
| Update | 143k / 23a | 1,044k / 68a | **1,074k / 25a** |
| Delete | 141k / 18a | 209k / 60a | **66k / 13a** |

---

## Competitive Analysis

### Phastos vs SQLx

| Operation | MySQL | PostgreSQL |
|---|---|---|
| Insert | 1.2× faster | 1.1× faster |
| SelectByID | **2.3× faster** | **2.1× faster** |
| SelectList | **1.8× faster** | **1.8× faster** |
| Update | 1.2× faster | 7.5× slower* |
| Delete | **2.0× faster** | **2.1× faster** |

*PG Update: SQLx uses raw `ExecContext` (no prepare), while Phastos uses `PrepareContext` + cached stmt. The extra Prepare round-trip on `lib/pq` is the bottleneck. Using `pgx` would close this gap.

### Phastos vs GORM

| Operation | MySQL | PostgreSQL |
|---|---|---|
| Insert | 1.2× faster | 1.0× faster |
| SelectByID | **2.1× faster** | **1.1× faster** |
| SelectList | **1.7× faster** | 1.1× faster |
| Update | 1.2× faster | comparable |
| Delete | **3.0× faster** | **3.2× faster** |

---

## Phase 1: Baseline Optimizations (B1–B11, R1–R8)

These optimizations addressed fundamental overhead in the ORM's hot paths — repeated reflection, string allocation, and lack of statement caching.

### B1: Reflection caching
Struct field metadata (column names, types, tags) was recomputed on every query via `reflect.Type.Field()` + `Tag.Lookup()`. Cached in `structCache sync.Map` keyed by `reflect.Type`, eliminating repeated reflection walks.

### B2: Select-columns cache
`GenerateSelectCols` recomputed column lists from struct tags each call. Added `selectColsCache sync.Map` keyed by `(Type, ExcludeCols, IncludeCols)`. On cache hit, returns pre-computed `[]string` directly.

### B3: PostgreSQL ExecContext
PG `Write()` path used `QueryRowContext` + `Scan` for all write actions, which is slower than `ExecContext` for non-INSERT operations. Changed UPDATE/DELETE to use `ExecContext` via cached prepared statements.

### B4: Removed goroutines from Read/Write
`Read()` and `Write()` spawned goroutines for New Relic monitoring segments. Goroutine scheduling overhead (1–2µs) exceeded the benefit for short-lived segments. Replaced with direct segment creation in the calling goroutine.

### B5: Pre-sized slices
`ExtractUpdateValues` and `GenerateSelectCols` used `append` on nil slices, causing repeated allocations. Pre-sized with `make([]T, 0, cap)` based on known field counts.

### B6: Builder pool (`sync.Pool` for `strings.Builder`)
`Read()` and `Write()` allocated a new `strings.Builder` per call. Pooled via `sync.Pool` to reuse across calls, reducing GC pressure.

### B7: `WriteString` for query building
Query construction used `fmt.Fprintf` for simple string appends. Replaced with `builder.WriteString()` which avoids `fmt` formatting overhead.

### B8: UpdateTemplateInfo cache
Pre-computed the full UPDATE SET clause template per `reflect.Type` — column names, field paths for value extraction. Eliminates repeated reflection + string building on each `UpdateById` call.

### B9: UpdateById fast path
When no transaction and the struct type is known, `UpdateById` bypasses `Write()` entirely — uses cached template + cached stmt directly. Saves builder allocation, `CUDConstructData` allocation, and `QueryOpts` allocation.

### B10: CachedRebind
`Rebind()` scans the query string for `?` placeholders and replaces them with `$1, $2, ...` for PostgreSQL. For cached templates, the same query is produced every time. Added `rebindCache sync.Map` to cache results.

### B11: UpdateById stmt cache
`writeStmtCache sync.Map` caches `*sql.Stmt` per query string. On first call, prepares and stores. On subsequent calls, reuses directly. Evicts on execution error (stale connection).

### R1: writeStmtCache
Consolidated prepared statement caching for all non-transaction write paths. Same stmt is reused across `UpdateById`, `DeleteById`, and `Update` calls.

### R3: readQueryCache
Cached base SELECT queries per `(reflect.Type, tableName)` in `readQueryCache sync.Map`. Avoids `getBaseQuery` reflection + `fmt.Sprintf` per read call.

### R5: cudDataPool (`sync.Pool` for `CUDConstructData`)
Pooled `CUDConstructData` objects via `sync.Pool`. Eliminates per-write heap allocation of the CUD data struct.

### R6: timeFormatLayout constant
Extracted `"2006-01-02 15:04:05"` as a package-level constant instead of a string literal, allowing the compiler to deduplicate and avoid per-call allocation.

### R7: Removed slow path from UpdateById
When the struct type is known (the common case), `UpdateById` no longer falls through to the generic `cudProcess` path. The template-based fast path handles all struct inputs.

### R8: Unified cache entries
`updateByIdCacheEntry.Template` now references the shared `UpdateTemplateInfo` from `struct_cache.go` instead of maintaining a separate per-type cache. Eliminates duplicate caching.

---

## Phase 2: Architectural Optimizations (O1–O6)

These optimizations targeted systemic overhead that Phase 1 couldn't address — per-call Prepare in read paths, variable-length SET clauses breaking stmt caching, and allocation-heavy intermediate objects.

### O1: Read-path prepared statement cache
**Problem**: `Read()` always called `db.SelectContext`/`db.GetContext` directly, which re-prepares the statement on every call for MySQL and PostgreSQL (non-transaction path). For fixed queries like `GetDetailById`, the same SQL runs every time but the driver can't reuse the prepared statement.

**Solution**: Added `readStmtCache sync.Map` storing `*sqlx.Stmt` per final query string. `Read()` non-trx path tries `getReadStmtx()` first; on cache hit, uses `stmt.SelectContext`/`stmt.GetContext` directly. Falls back to direct DB call on miss. Evicts on execution error for stale connection recovery.

**Impact**: SelectByID improved from ~160k to ~79k ns on MySQL (2× faster).

### O2: Zero-alloc GetDetailById fast path
**Problem**: `GetDetailById` allocated `QueryOpts{}`, called `getBaseQuery` (does `reflect.ValueOf` + type unraveling per call), `fmt.Sprintf` to append `WHERE id = ?`, then `Read()` copies base query into `strings.Builder` and runs `Rebind`. For a fixed query, all this is redundant.

**Solution**: Extended `selectByIdCache` to store `selectByIdCacheEntry` with pre-computed rebound query (computed once via `db.CachedRebind`). `GetDetailById` fast path for struct types sets `opts.BaseQuery = entry.ReboundQuery` directly — skips builder, `Sprintf`, and `Rebind` entirely.

**Impact**: Combined with O1, SelectByID dropped from ~160k to ~79k ns on MySQL.

### O3: Pool QueryOpts in read paths
**Problem**: `GetDetailById`, `GetList`, `GetDetail` all allocate `&database.QueryOpts{}` per call. The pool already exists (`queryOptsPool`) but wasn't used in read action methods.

**Solution**: `GetDetailById` fast path uses `GetQueryOpts()`/`PutQueryOpts()` from the existing pool. Saves 1 alloc/op.

**Impact**: Minor allocation reduction (−1 to −3 allocs/op depending on path).

### O4: Fixed-template UpdateById
**Problem**: PostgreSQL `UpdateById` was 905k ns vs SQLx 139k ns (6.5× slower). The root cause: `ExtractUpdateValues` dynamically excluded null/empty fields, producing a different SET clause each call (e.g., `SET name=?,age=?` vs `SET name=?,email=?,age=?`). Each unique query creates a new prepared statement that's never reused. On `lib/pq`, this means a full Prepare round-trip per call.

**Solution**: `ExtractFixedUpdateValues` always emits all fields regardless of null/empty status. For `null.String` with `Valid=false`, sends `nil` (SQL NULL). For empty strings, sends `""` (preserves NOT NULL constraints). For `created_at`, the field is excluded from the UPDATE template entirely (it's set once at INSERT). This produces an invariant query string — the same `SET name=?,email=?,age=?,status=?,updated_at=?,deleted_at=? WHERE id=?` every time — so one cached `*sql.Stmt` is reused across all calls.

**Impact**: PG Update allocs dropped from 72 to 25 (−65%). MySQL Update allocs from 38 to 22 (−42%). The fixed template ensures the prepared statement cache is effective.

### O5: Cached DeleteById with pre-prepared stmt
**Problem**: `DeleteById` allocated `CUDConstructData{}` + `QueryOpts{}` per call, then went through `Write()` which built the query from scratch. The query is always `UPDATE table SET deleted_at = now() WHERE id = ?` (or `DELETE FROM table WHERE id = ?`).

**Solution**: Cache the rebound query per `(tableName, isSoftDelete)` in `deleteByIdCache sync.Map`. Fast path: get cached query → `GetWriteStmt` → `stmt.ExecContext(ctx, id)` → return pooled `CUDResponse`. No `CUDConstructData`, no `QueryOpts`, no `Write()` builder.

**Impact**: Delete allocs dropped from 22 to 12 on MySQL, 36 to 13 on PostgreSQL. Latency improved ~2× on both databases.

### O6: CachedRebind in Read() non-trx path
**Problem**: `Read()` called `db.Rebind(query.String())` per call. For fixed queries, the same rebind result is produced every time.

**Solution**: Changed to `this.CachedRebind(query.String())` which checks `rebindCache` first. Already partially in place from B10; fully wired into the Read path.

**Impact**: Eliminates 1 string scan/allocation per read call.

---

## Architectural Advantages

### 1. Multi-layer statement caching
Phastos maintains three independent stmt caches:
- `writeStmtCache` — prepared statements for UPDATE/DELETE/INSERT (bypasses `Write()` builder)
- `readStmtCache` — prepared statements for SELECT (bypasses per-call Prepare)
- `rebindCache` — cached `Rebind` results (bypasses string scanning)

These caches work together: a fixed template (O4) ensures the same query string is produced every time, making the stmt cache effective. Without a fixed template, each unique null-pattern creates a new query that can never be reused.

### 2. Template-based query construction
Instead of dynamically building queries from struct field inspection on each call, Phastos pre-computes a query template per struct type at first use. The template includes all column names, field paths for value extraction, and the rebound query string. Subsequent calls only extract values from the struct instance — no reflection, no string building, no `fmt.Sprintf`.

### 3. Fast-path architecture
Phastos uses a tiered approach:
- **Fast path** (struct type, no transaction): Bypasses `Write()`/`Read()` entirely. Directly uses cached stmt + cached query. Minimal allocations via `sync.Pool`.
- **Trx path** (struct type, with transaction): Uses the template for consistent query construction but goes through `Write()`/`Read()` for proper transaction handling.
- **Slow path** (non-struct inputs): Original implementation with dynamic column extraction.

This means the common case (struct type, no transaction) gets the fastest possible path, while edge cases still work correctly.

### 4. Pooled intermediate objects
All short-lived objects are pooled via `sync.Pool`:
- `CUDResponse` — write result
- `CUDConstructData` — write input
- `QueryOpts` — read/write options
- `TableRequest` — read filtering
- `strings.Builder` — query construction
- `SelectResponse` — read result

This reduces GC pressure significantly, especially under high concurrency.

### 5. Eviction-based cache invalidation
All stmt caches use an eviction strategy: if execution fails (e.g., stale connection), the stmt is removed from cache and re-prepared on next access. This is simple, correct, and doesn't require explicit cache invalidation on connection pool changes.

---

## Known Limitations

### PostgreSQL Update latency
Phastos Update on PostgreSQL is ~7.5× slower than SQLx (1,074k vs 143k ns). SQLx uses a raw `db.ExecContext` call (no prepare), while Phastos uses `PrepareContext` + cached stmt. The `lib/pq` driver adds a network round-trip for Prepare. Switching to `pgx` (which supports true prepared statement caching at the connection level) would close this gap.

### Insert allocation overhead
Phastos Insert allocates 46–64 objects per call vs SQLx's 18–28. This is inherent to Phastos' higher-level API: reflection-based column extraction, `CUDConstructData`, `QueryOpts`, and `CUDResponse` objects. The template-based approach (used for Update) could be extended to Insert, but the benefit is smaller since Insert queries are already one-time-use (RETURNING id prevents stmt reuse on PG).

---

## Files Modified

| File | Changes |
|---|---|
| `go/helper/struct_cache.go` | Reflection caching, select-cols cache, `UpdateTemplateInfo`, `ExtractUpdateValues`, `ExtractFixedUpdateValues`, `created_at` skip |
| `go/database/sql.go` | `builderPool`, `rebindCache`, `writeStmtCache`, `readStmtCache`, `getWriteStmt`, `getReadStmtx`, `CachedRebind`, `evictWriteStmt`, `evictReadStmt`, exported helpers, Read() cached stmt path, Write() cached stmt path |
| `go/database/action/write.go` | `UpdateById` fast path, `DeleteById` fast path, `deleteByIdCache`, `updateByIdCache`, `getDeleteByIdQuery` |
| `go/database/action/read.go` | `selectByIdCache` with rebound query, `GetDetailById` fast path, pooled `QueryOpts` |
| `go/database/pool.go` | `queryOptsPool`, `cudResponsePool`, `cudDataPool`, `tableRequestPool`, `selectResponsePool` |
| `go/database/definition.go` | `SetEngine`, `SetSlowQueryThreshold`, `ISQL` interface updates |
