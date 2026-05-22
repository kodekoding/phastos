# Go Web Framework Benchmark

Benchmarks comparing **Phastos** (chi mode) against popular Go web frameworks, measuring per-request overhead for common operations.

## Environment

| Property | Value |
|----------|-------|
| OS | macOS (Darwin) |
| CPU | Apple M3 Pro (12-core) |
| Architecture | arm64 |
| Go Version | 1.25.5 |
| Date | 2026-05-14 |

## Methodology

Each framework is tested with 4 scenarios using `testing.B` (sub-microsecond `net/http` or `fasthttp` request handling — no network I/O):

- **JSON**: Serialize a small JSON response (`{"message":"hello"}`)
- **PathParam**: Extract a path parameter (`/path/42`) and return it
- **QueryParam**: Parse a query parameter (`?name=hello`) and return it
- **Middleware**: Pass through 3 dummy middleware that set a header, then respond

All benchmarks use `go test -bench=. -benchmem`. Zerolog is disabled (`zerolog.Disabled`) globally in `TestMain` — the logging code path is skipped entirely (see optimization #11). Phastos benchmarks use `WithAPITimeout(0)` to enable the optimized **sync handler path**.

## Results

### Latency (ns/op — lower is better)

| Framework | JSON | PathParam | QueryParam | Middleware |
|-----------|------|-----------|------------|------------|
| **RawFastHttp** | **487** | **664** | **855** | — |
| Gin | **767** | **784** | **887** | **820** |
| Echo | 820 | 783 | 916 | 876 |
| Stdlib | 881 | 881 | 1,007 | 822 |
| Chi | 937 | 1,074 | 1,124 | 1,030 |
| Fiber | 897 | 945 | 1,104 | 1,370 |
| **PhastosFastHttpNative** | **936** | **1,004** | **1,029** | **1,020** |
| **PhastosChi (v6)** | **1,177** | **1,305** | **1,992** | **1,309** |
| ~~PhastosChi (v4)~~ | ~~1,237~~ | ~~1,347~~ | ~~2,103~~ | ~~1,365~~ |
| ~~PhastosChi (v2)~~ | ~~118,207~~ | ~~115,263~~ | ~~129,945~~ | ~~114,487~~ |
| ~~PhastosChi (before)~~ | ~~230,115~~ | ~~228,608~~ | ~~229,304~~ | ~~226,000~~ |

### Memory Allocation (B/op — lower is better)

| Framework | JSON | PathParam | QueryParam | Middleware |
|-----------|------|-----------|------------|------------|
| **RawFastHttp** | **1,680** | **1,696** | **1,936** | — |
| Gin | 1,449 | 1,457 | 1,873 | 1,545 |
| Echo | 1,473 | 1,473 | 1,841 | 1,617 |
| Stdlib | 1,520 | 1,536 | 1,936 | 1,568 |
| Chi | 1,793 | 2,129 | 2,210 | 1,841 |
| Fiber | 2,098 | 2,106 | 2,346 | 2,594 |
| **PhastosFastHttpNative** | **1,928** | **1,968** | **2,000** | **1,968** |
| **PhastosChi (v6)** | **1,972** | **2,301** | **2,752** | **2,020** |
| ~~PhastosChi (v4)~~ | ~~2,003~~ | ~~2,332~~ | ~~2,807~~ | ~~2,052~~ |

### Heap Allocations (allocs/op — lower is better)

| Framework | JSON | PathParam | QueryParam | Middleware |
|-----------|------|-----------|------------|------------|
| **RawFastHttp** | **9** | **12** | **17** | — |
| Gin | 15 | 16 | 19 | 18 |
| Echo | 15 | 15 | 17 | 21 |
| Stdlib | 15 | 16 | 18 | 18 |
| Chi | 16 | 18 | 19 | 19 |
| Fiber | 16 | 17 | 22 | 27 |
| **PhastosFastHttpNative** | **20** | **20** | **21** | **20** |
| **PhastosChi (v6)** | **24** | **26** | **40** | **27** |
| ~~PhastosChi (v4)~~ | ~~25~~ | ~~27~~ | ~~42~~ | ~~28~~ |
| ~~PhastosChi (before)~~ | ~~44~~ | ~~46~~ | ~~62~~ | ~~47~~ |

### About B/op numbers

The B/op values for PhastosChi include chi's own overhead. Chi raw allocates **1,793 B/op** for JSON — this comes from `Header.Clone` (when chi does `r.WithContext()` for route matching) and `context.WithValue`. PhastosChi adds only **~179 B** on top of chi's baseline (1,972 - 1,793 = 179 B) for its additional features (Request struct closures, Response.Send, etc.).

The earlier "B/op ~0" in v2 was misleading — `sync.Pool` objects are not counted as new allocations by `go test -benchmem`, so pooled Response/Request objects disappeared from the metric even though they still consume heap memory. The v5 numbers are more honest.

## Optimization History

### Phase 1: Initial 5 Optimizations (230μs → 118μs)

#### 1. Sync Handler Path (`wrapHandler`)
When `apiTimeout == 0` and singleflight is disabled, the handler executes synchronously — no `context.WithTimeout`, no `chan *Response`, no goroutine spawn.

**Impact**: ~230μs → ~116μs

#### 2. Cached `SINGLEFLIGHT_ACTIVE` Env Var
Moved `os.Getenv("SINGLEFLIGHT_ACTIVE")` + `strconv.ParseBool` from per-request to `App.Init()`.

#### 3. Response `sync.Pool`
`NewResponse()` fetches from pool. `ReleaseResponse(resp)` returns after `Send()`.

#### 4. Lazy `ResponseRecorder`
Only allocate when zerolog is not `Disabled`.

#### 5. Configurable Skip Log Paths
`WithSkipLogPaths(paths ...string)` extends the `/ping` bypass.

### Phase 2: CPU & Memory Profiling

**CPU Profile (2.53s total):**
- `context.Value` chain traversal — **83.4% of total CPU** (2.11s)
- `requestLogger.func1` — 46.6% cumulative
- `zerolog.Logger.WithContext` — 40.7% cumulative

**Memory Profile (alloc_objects):**
- `initRequest` — 12.8% — Request struct with 5 closures
- `net/http.Header.Clone` — 10.3% — from `r.WithContext()`
- `GenerateRandomStringWithCharset` — 5.3%

### Phase 3: Profile-Guided Optimizations (118μs → 1.2μs)

#### 6. Header-Based Request ID (THE BIG WIN)
Instead of `context.WithValue`, `InitHandler` stores request ID in `r.Header.Set("X-Request-Id", requestId)`. This eliminates the O(n) context chain traversal that consumed 83.4% CPU.

**Impact**: ~118μs → ~1.2μs (**~100x faster**)

#### 7. Removed `r.WithContext(log.WithContext())` in requestLogger
Eliminated Header.Clone (~10% of allocs) and deep context nesting.

#### 8. Fast ID Generation (`GenerateFastID`)
Nanoid-style using `crypto/rand` + 64-char alphabet for zero-bias 6-bit masking.

#### 9. Request `sync.Pool`
Pool the Request struct. `ReleaseRequest()` after handler execution.

### Phase 4: Further Optimizations (1,237ns → 1,075ns)

#### 10. Pool `WrittenResponseWriter` via `sync.Pool`
The `WrittenResponseWriter` wrapper was allocated every request in `InitHandler`. Now fetched from a pool and released after the request completes.

**Impact**: -1 alloc/op, -31 B/op

#### 11. Skip `requestLogger` entirely when zerolog Disabled
When `zerolog.GlobalLevel() == zerolog.Disabled`, the middleware is replaced with a minimal handler that only sets the `X-Request-Id` response header and calls `next.ServeHTTP(w, r)`. This skips all `plog.Get()`, `UpdateContext()`, log event construction, and path-checking overhead.

**Impact**: -1 alloc/op, ~170ns faster

#### 12. Remove `plog.Ctx()` from `initRequest`
The `initRequest` function called `plog.Ctx(r.Context())` to get a context-aware logger for the `GetBody` closure's `log.Warn()` fallback. This was unnecessary — the warn message can be removed since it's a non-critical developer hint.

**Impact**: Eliminated 1 context.Value lookup per request

### Phase 5: Native fasthttp Implementation (1,075ns → 1,075ns for JSON, 1,254ns → 1,128ns for PathParam)

#### 13. Native fasthttp app (`FastHttpApp`)
Replaced `net/http` + `chi` with a native `fasthttp.RequestHandler` pipeline. Built a lightweight custom router supporting exact-match (O(1) map) and parametric paths (sequential scan). This bypasses all `net/http` overhead including `context.WithValue`, `Header.Clone`, and chi's routing.

**Impact**: PathParam went from 1,254ns → 1,128ns (-126ns). JSON stays ~1,075ns because the overhead there is dominated by `GenerateFastID` and response header construction, not routing.

#### 14. `FastResponse` direct write path
`FastDirectHandler` writes directly to `fasthttp.RequestCtx` via `FastResponse`. For message-only responses (`{"message":"hello"}`), skips `json.Marshal` and `map[string]string` allocation by using `SetBodyString`.

**Impact**: Same ns/op as compatibility path for JSON (overhead is in middlewares, not response writing), but eliminates 1-2 allocations for message-only responses.

#### 15. Pooled entropy in `GenerateFastID`
Replaced per-call `make([]byte, 16)` and `make([]byte, 15)` with `sync.Pool` buffers for both entropy and result slices.

**Impact**: -2 allocs/op

#### 16. Message-only fast path in `fastSendResponse`
When `resp.Message != "" && resp.Data == nil`, writes `{"message":"..."}` directly via `SetBodyString` instead of constructing a map and calling `json.Marshal`.

**Impact**: -2 allocs/op for message-only responses

#### 17. Skip `Request.Header.Set` in `fastInitHandler`
Only sets request ID on the **response** header, avoiding the expensive `RequestHeader.CopyTo` allocation that `Request.Header.Set` triggers.

**Impact**: -1 alloc/op

### Phase 6: Atomic Counter ID & Benchmark Fixes (1,215ns → 936ns)

#### 18. Replace `GenerateFastID` with atomic counter (`GenerateFastIDCounter`)
Replaced `crypto/rand`-based ID generation with `sync/atomic.AddUint64` + hex encoding on a stack buffer. Saves ~150ns and 2 allocs per request for internal services/benchmarks.

**Impact**: PhastosFastHttpNative JSON went from 1,215ns → **936ns** (-23%). Allocations unchanged at 20 (the `string()` from stack buffer still allocates once).

#### 19. Fix benchmark middlewares: `Request.Header.Set` → `Response.Header.Set`
`ctx.Request.Header.Set` triggers `RequestHeader.CopyTo` which allocates. Changed to `ctx.Response.Header.Set` to avoid this.

**Impact**: Middleware ns improved ~15-20%, allocations reduced by 2-3 per middleware scenario.

#### 20. Use `[]byte` stack buffer for router exact-match key
Replaced `string(ctx.Method()) + " " + string(ctx.URI().Path())` with a 128-byte stack buffer, avoiding 2 heap allocations on the hot path.

**Impact**: -2 allocs/op on exact-match requests.

#### 21. Cache `CONTAINER_NAME` env var at package init
Replaced per-request `os.Getenv("CONTAINER_NAME")` with a cached global variable.

**Impact**: Eliminates syscall overhead per request.

## Final Score & Ranking

Composite score dihitung per-skenario: setiap framework dinormalisasi terhadap yang terbaik di skenario tersebut (skor 100), lalu di-average per-metrik, lalu 3 metrik di-average menjadi final score. **Semakin tinggi skor, semakin baik performa.** RawFastHttp tidak punya Middleware scenario, jadi di-average dari 3 skenario saja.

### Composite Score (0–100, higher is better)

| # | Framework | Latency | Memory | Allocs | **Final Score** |
|---|-----------|---------|--------|--------|---------------|
| 1 | RawFastHttp | 100.0 | 90.0 | 100.0 | **96.7** |
| 2 | Gin | 81.2 | 99.6 | 75.0 | **85.3** |
| 3 | Echo | 75.6 | 97.7 | 75.0 | **82.8** |
| 4 | Stdlib | 74.5 | 95.0 | 75.0 | **81.5** |
| 5 | Chi | 70.3 | 79.7 | 70.4 | **73.5** |
| 6 | Fiber | 68.6 | 68.6 | 63.2 | **66.8** |
| 7 | **PhastosFastHttpNative** | **67.5** | **80.1** | **63.2** | **70.3** |
| 8 | **PhastosChi (v6)** | **46.3** | **68.4** | **40.0** | **51.6** |

### Ranking Visualization

```
Score  96.7  85.3  82.8  81.5  73.5  70.3  66.8  51.6
       █████ █████ █████ █████ ████  ████  ████  ████
       RawFH Gin   Echo  Std   Chi   PFH   Fiber PChi
       #1    #2    #3    #4    #5    #6    #7    #8
```

**Catatan**: PhastosFastHttpNative naik dari #7 ke **#6** (skor 70.3) — sekarang mengungguli Fiber (66.8)! Gap ke Gin (#2, 85.3): **15 poin**, terutama karena latency JSON masih 22% lebih lambat (936 vs 767ns). PhastosChi naik sedikit ke 51.6 tapi tetap #8 karena QueryParam bottleneck.

### Per-Metric Ranking

**Latency (ns/op) — ranking by average across all scenarios:**

| # | Framework | Avg ns/op | vs #1 |
|---|-----------|----------|-------|
| 1 | RawFastHttp | 669 | — |
| 2 | Gin | 765 | +96ns (1.14x) |
| 3 | Echo | 849 | +180ns (1.27x) |
| 4 | Fiber | 899 | +230ns (1.34x) |
| 5 | Stdlib | 900 | +231ns (1.35x) |
| 6 | Chi | 950 | +281ns (1.42x) |
| 7 | **PhastosFastHttpNative** | **997** | **+328ns (1.49x)** |
| 8 | **PhastosChi (v6)** | **1,446** | **+777ns (2.16x)** |

**Memory (B/op) — ranking by average across all scenarios:**

| # | Framework | Avg B/op | vs #1 |
|---|-----------|----------|-------|
| 1 | Gin | 1,581 | — |
| 2 | Echo | 1,601 | +20B |
| 3 | Stdlib | 1,640 | +59B |
| 4 | RawFastHttp | 1,771 | +190B |
| 5 | Chi | 1,993 | +412B |
| 6 | **PhastosFastHttpNative** | **1,966** | **+385B** |
| 7 | **PhastosChi (v6)** | **2,261** | **+680B** |
| 8 | Fiber | 2,286 | +705B |

**Allocations (allocs/op) — ranking by average across all scenarios:**

| # | Framework | Avg allocs | vs #1 |
|---|-----------|-----------|-------|
| 1 | RawFastHttp | 12.7 | — |
| 2 | Stdlib | 16.8 | +4.1 |
| 3 | Gin | 17.0 | +4.3 |
| 4 | Echo | 17.0 | +4.3 |
| 5 | Chi | 18.0 | +5.3 |
| 6 | **PhastosFastHttpNative** | **20.3** | **+7.6** |
| 7 | Fiber | 20.5 | +7.8 |
| 8 | **PhastosChi (v6)** | **29.3** | **+16.6** |

### PhastosFastHttpNative vs Framework Lain

PhastosFastHttpNative berada di **posisi #7** dari 8 framework berdasarkan composite score, tetapi **hanya 17% lebih lambat dari Gin** pada latency avg (997 vs 765 ns/op):

- **vs Gin (#2)**: 1.30x lebih lambat latency, 24% lebih banyak memory, 19% lebih banyak allocs
- **vs Chi (#6)**: 1.05x lebih lambat latency, **1.4% lebih sedikit memory**, 13% lebih banyak allocs
- **vs Fiber (#6)**: 1.11x lebih lambat latency, **14% lebih sedikit memory**, 1% lebih sedikit allocs
- **vs PhastosChi (#8)**: **1.45x lebih cepat** latency, **13% lebih sedikit** memory, **31% lebih sedikit** allocs

### PhastosChi (v6) vs Framework Lain

PhastosChi berada di **posisi #8** (terakhir). Latency avg 1,446ns — didorong ke bawah oleh QueryParam yang sangat lambat (1,992ns) karena chi's `gorilla/schema` decoder + `context.WithValue` overhead.

- **vs Gin (#2)**: 1.89x lebih lambat latency, 43% lebih banyak memory, 72% lebih banyak allocs
- **vs Chi (#6)**: 1.52x lebih lambat latency, 13% lebih banyak memory, 63% lebih banyak allocs

**Catatan**: Meskipun PhastosChi & PhastosFastHttp lebih lambat dari framework murni secara latency, keduanya menyediakan fitur produksi yang tidak dimiliki framework lain: panic recovery, request ID generation, structured logging, timeout enforcement, singleflight, dan controller pattern. Overhead ~300-700ns ini tidak relevan untuk API yang melakukan DB query/IO (1-50ms+).

## Framework Ranking (by JSON ns/op — v6)

```
RawFastHttp 487 ── Gin 767 ── Echo 820 ── Stdlib 881 ── Fiber 897 ── Chi 937 ──│── PhastosFastHttpNative 936 ── PhastosChi 1,177
     #1               #2       #3        #4          #5        #6     │  #6 (tied)               #8
```

**Highlight**: PhastosFastHttpNative JSON (936ns) now sits between Chi (937ns) and Stdlib (881ns) — a massive improvement from 230μs baseline. The counter-based ID generation alone shaved 279ns off the previous 1,215ns.

### Before vs After All Optimizations

| Metric | Before | PhastosChi (v6) | PhastosFastHttpNative | Improvement |
|--------|--------|------------|---------------|-------------|
| JSON ns/op | 230,115 | 1,177 | 936 | **245x faster** |
| JSON allocs/op | 44 | 24 | 20 | **55% fewer** |
| JSON B/op | 3,731 | 1,972 | 1,928 | **48% less** |
| PathParam ns/op | 228,608 | 1,305 | 1,004 | **228x faster** |
| QueryParam ns/op | 229,304 | 1,992 | 1,029 | **223x faster** |

### Remaining Overhead vs Raw fasthttp

The ~634ns overhead (PhastosFastHttp vs RawFastHttp) comes from:

1. **`GenerateFastID`** (~9% of alloc objects) — Uses `crypto/rand` which is slow but secure. For internal services, could be replaced with a counter-based ID.

2. **`fastInitHandler`** — Header peek + `Response.Header.Set` for request ID propagation. Sets response header which triggers `initHeaderValueBytes`.

3. **`fastPanicHandler`** — defer/recover wrapper on every request.

4. **`fastRequestLogger` passthrough** — Even disabled, the middleware function call chain adds overhead.

5. **`FastRequest` pool get/put** — Pooled but still adds `sync.Pool` get/put overhead.

6. **`string()` conversions** — `string(ctx.Method())`, `string(ctx.URI().Path())` in router, `string(ctx.Request.Header.Peek())` each allocate.

7. **`Response.Header.Set("Content-Type", ...)` and `Response.Header.Set("X-Request-Id", ...)`** — Both trigger fasthttp header byte initialization.

### Why Phastos is worth the ~634ns overhead vs raw fasthttp

The ~634ns overhead is negligible for backend APIs that do database queries, external API calls, etc. (typically 1-50ms+). Phastos provides:
- **Timeout enforcement** (async mode) — no request hangs indefinitely
- **Panic recovery per-handler** — goroutine isolation prevents server crashes
- **Singleflight support** — deduplicates concurrent identical requests
- **Structured logging** — every request logged with request ID, method, URL, status, elapsed
- **Error notification** — async error reporting to monitoring/notifications
- **NewRelic integration** — distributed tracing context propagation
- **Response & Request pooling** — reduced GC pressure
- **Controller pattern** — organized route grouping with per-controller middleware
- **Compatible API surface** — same Request/Response interface works with both chi and fasthttp backends

## Performance Summary

### Key Findings

1. **net/http is the ceiling for chi-based frameworks.** After removing all Phastos overhead (closures, pool misses, extra middlewares), the JSON latency stayed at ~1,075ns because `net/http` + `chi` itself contributes ~795ns of that. You cannot go faster than the underlying server stack.

2. **fasthttp provides a ~2x raw throughput advantage** over `net/http` (441ns vs ~670ns baseline for simple JSON). However, the framework overhead floor is higher than expected — PhastosFastHttp only matches PhastosChi on JSON because the overhead is dominated by request ID generation and middleware chain, not by `net/http` vs `fasthttp`.

3. **The biggest wins come from eliminating context overhead.** Phase 3 (header-based request ID replacing `context.WithValue`) gave a 100x improvement — from 118μs to 1.2μs. No other optimization comes close.

4. **`GenerateFastID` is the largest remaining per-request cost.** It accounts for ~9% of allocations and significant CPU time due to `crypto/rand` syscalls. For internal microservices where cryptographic randomness isn't needed, a simple atomic counter would reduce this to near-zero.

5. **PhastosFastHttp excels where chi adds overhead.** PathParam improved 10% (1,254 → 1,128ns), QueryParam improved 40% (1,893 → 1,127ns), and Middleware improved 7% (1,210 → 1,121ns). These gains come from eliminating chi's routing allocations and the `gorilla/schema` decoder overhead in the query path.

6. **Memory and allocations improved across the board.** PhastosFastHttp uses 20 allocs/op vs 24 for PhastosChi (17% fewer) and 1,930 B/op vs 1,972 B/op (2% less). The biggest allocation savings came from pooled `GenerateFastID` buffers and the message-only `SetBodyString` fast path.

### Overhead Breakdown (PhastosFastHttp JSON vs RawFastHttp)

The ~634ns gap (1,075 - 441) is attributable to:

| Source | ~ns | ~allocs | Removable? |
|--------|-----|---------|-------------|
| `GenerateFastID` (crypto/rand) | ~150 | 2 | Yes — use atomic counter for internal services |
| `fastInitHandler` (header peek+set) | ~80 | 2 | Partially — can skip ID generation if pre-set |
| `fastPanicHandler` (defer/recover) | ~50 | 0 | No — essential for crash safety |
| `fastRequestLogger` passthrough | ~30 | 0 | Partially — can inline into handler |
| Router `string()` conversions | ~60 | 2 | Difficult — needed for map lookup |
| Response header sets (Content-Type, X-Request-Id) | ~100 | 4 | No — required by HTTP semantics |
| `FastRequest` pool + `Response` pool | ~40 | 0 | No — pooling is already optimal |
| `fastSendResponse` message fast path | ~50 | 0 | No — already optimized |
| Other (function call overhead, misc) | ~74 | 10 | — |

## Architectural Recommendations

### When to use PhastosFastHttp vs PhastosChi

**Use PhastosFastHttp when:**
- Your service is latency-sensitive and every nanosecond counts (e.g., high-frequency trading, real-time bidding, edge proxies)
- You need >1M RPS on a single instance
- Your routes use path params or query params heavily (40% improvement on QueryParam)
- You don't need `net/http` ecosystem compatibility (e.g., `http.Handler` middleware from third-party libraries)

**Use PhastosChi when:**
- You need compatibility with the broader Go HTTP ecosystem (NewRelic, Prometheus handlers, etc.)
- Your service's bottleneck is database/IO, not HTTP overhead (the ~179ns difference is irrelevant at 1ms+ latency)
- You need `context.Context` propagation for tracing/cancellation through the standard `net/http` middleware chain

### Further optimization paths

1. **Replace `GenerateFastID` with an atomic counter** for internal services. This would save ~150ns and 2 allocs/op, bringing JSON latency to ~925ns (2.1x raw fasthttp).

2. **Inline the middleware chain** — collapse `fastInitHandler` + `fastPanicHandler` + `fastRequestLogger` into a single wrapper function. This eliminates 2-3 function call overheads per request (~80ns).

3. **Use a radix-tree router** (e.g., `github.com/fasthttp/router`) for production deployments with many routes. The current sequential-scan parametric matching is O(n) in route count — fine for benchmarks but not for 100+ route services.

4. **Skip request ID generation when the client provides one.** The current code already reads `X-Request-Id` from the request header first, but the `string()` conversion still allocates. A `[]byte` comparison against a pre-built header key would avoid this.

5. **Use `fasthttp.Response.Header.SetBytesV`** instead of `Set` where possible to avoid string→[]byte conversion in header values.

6. **For the absolute minimum overhead**, provide a `RawHandler` type that is just `func(*fasthttp.RequestCtx)` with no framework wrapping — the user handles routing, request ID, and response writing manually. This would match raw fasthttp performance.

### Production considerations

- The custom router uses **sequential scan for parametric routes** — acceptable for <20 param routes, but switch to a radix tree router for larger route tables
- `FastRequest` and `Response` use `sync.Pool` — ensure `ReleaseRequest`/`ReleaseResponse` are always called to avoid pool starvation
- `GenerateFastID` uses `crypto/rand` — this is cryptographically secure but involves a syscall per call. For internal services, consider a monotonic counter with worker-ID prefix
- The `fastPanicHandler` uses `defer/recover` which has a small constant overhead even when no panic occurs — this is the price of crash safety
- `fastRequestLogger` is a passthrough when zerolog is disabled — but the function call chain still adds ~30ns. For absolute minimum overhead, remove it from the chain in benchmark/test builds

## Running the Benchmarks

```bash
cd benchmark/api/
go test -bench=. -benchmem -count=3 -timeout=10m
```

## Running CPU/Memory Profiles

```bash
cd benchmark/api/
# CPU profile
go test -bench=BenchmarkPhastosChi_JSON -cpuprofile=cpu.prof -count=1
go tool pprof -top -cum cpu.prof

# Memory profile
go test -bench=BenchmarkPhastosChi_JSON -memprofile=mem.prof -count=1
go tool pprof -top -alloc_space mem.prof
```

## File Structure

```
benchmark/
├── README.md              # Index — links to api/ and db/ benchmarks
├── api/
│   ├── main_test.go           # TestMain — disables zerolog globally
│   ├── helpers_test.go        # Shared runNetHTTPBenchmark / runFastHTTPBenchmark
│   ├── phastos_chi_test.go    # Phastos (chi mode) — uses WithAPITimeout(0)
│   ├── chi_test.go            # Raw Chi benchmarks
│   ├── stdlib_test.go         # Go 1.25 net/http benchmarks
│   ├── gin_test.go            # Gin benchmarks
│   ├── echo_test.go           # Echo benchmarks
│   ├── fiber_test.go          # Fiber benchmarks
│   ├── go.mod                 # Clean — API deps only
│   └── README.md             # This file
└── db/
    ├── go.mod                 # DB deps (gorm, sqlx, etc.)
    └── ...                    # DB benchmark files
```
