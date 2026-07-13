# Auto-Binding Handler — Design Spec

**Date:** 2026-07-13
**Status:** Draft
**Scope:** phastos + timetraq-backend

## Problem

Setiap handler controller di timetraq memiliki 5 baris boilerplate identik untuk binding request:

```go
func (a *absence) Insert(req api.Request, ctx context.Context) *api.Response {
    var requestData employeemodel.AbsenceCreatePayload   // declare struct
    if err := req.GetBody(&requestData); err != nil {    // bind body
        return api.NewResponse().SetError(err)            // error check
    }
    data, err := a.uc.Insert(ctx, &requestData)           // forward to usecase
    if err != nil {
        return api.NewResponse().SetError(err)
    }
    return api.NewResponse().SetData(data)
}
```

Pattern ini berulang di 30+ handler: declare → bind → error-check → forward. Total ~130 baris boilerplate.

Selain itu, response wrapping (`SetError`/`SetData`) juga selalu sama — tidak ada logika custom di 80% handler.

## Goal

1. **Auto-binding**: Phastos bind request body, query params, dan path params secara otomatis berdasarkan route annotations sebelum handler dipanggil
2. **Clean handler**: Handler hanya pass-through ke usecase — tidak ada binding logic
3. **Clean usecase**: Usecase mengambil data dari context tanpa perlu check error (data sudah clean & validated)
4. **Backward compatible**: Handler lama (`func(req, ctx) *api.Response`) tetap jalan

## Design

### 1. Handler Signature

```go
// Old (still supported):
type Handler func(Request, context.Context) *api.Response

// New:
type Handler2 func(context.Context) (any, error)
```

Phastos mendeteksi signature handler saat registrasi. `Handler2` mendapat auto-binding.

### 2. Route Annotations

| Annotation | Purpose | Example |
|-----------|---------|---------|
| `api.WithRequest(T)` | Body binding type | `api.WithRequest(new(models.Account))` |
| `api.WithQuery(T)` | Query params binding type | `api.WithQuery(new(database.TableRequest))` |
| `api.WithPath(path)` | Path with inline type hints | `api.WithPath("/absence/{id:int64}/recap/{month}")` |

Path param default type = `string`. Inline type via `{name:type}`:

```
{id}           → string
{id:int64}     → int64
{month:int}    → int
{ratio:float64} → float64
{active:bool}  → bool
```

Supported path param types: `string`, `int`, `int8`, `int16`, `int32`, `int64`, `uint`, `uint8`, `uint16`, `uint32`, `uint64`, `float32`, `float64`, `bool`.

### 3. Binding Flow (Phastos Internal)

```
HTTP Request masuk
  │
  ├── 1. Parse path params dari WithPath pattern
  │      Extract {name:type} → convert string ke typed value
  │      ├── GAGAL (e.g. "abc" → int64) → return 422, handler NOT called
  │      └── SUKSES → store in context
  │
  ├── 2a. Bind query params (if query params exist in URL)
  │      gorilla/schema → struct → validate
  │      ├── GAGAL → return 400/422, handler NOT called
  │      └── SUKSES → store in context
  │
  ├── 2b. Bind body (if WithRequest is set)
  │      json.Unmarshal → struct → validate
  │      ├── GAGAL → return 400/422, handler NOT called
  │      └── SUKSES → store in context
  │
  │   Note: Query params diproses sebelum body. Jika WithQuery digunakan
  │   (GET endpoints), hanya query binding yang dilakukan, tanpa body.
  │
  └── 3. SEMUA SUKSES → call handler(ctx)
```

Jika binding gagal di step manapun, phastos langsung return error response ke client. Handler tidak pernah dipanggil dengan data invalid.

### 4. Context Package (`phastosctx`)

Package: `github.com/kodekoding/phastos/v2/go/context`
Import alias: `phastosctx`

```go
// Request — get bound body data
func Request[T any](ctx context.Context) *T

// Query — get bound query params
func Query[T any](ctx context.Context) *T

// PathParam — get typed path parameter
func PathParam[T any](ctx context.Context, name string) T

// File — get uploaded file
func File(ctx context.Context, name string) (multipart.File, *multipart.FileHeader, error)
```

Tidak ada error return dari `Request`, `Query`, dan `PathParam` — data sudah dijamin clean & validated oleh phastos sebelum handler dipanggil.

`File` mengembalikan error karena file upload hanya divalidasi presence-nya, bukan konten.

### 5. Response Wrapping (Phastos Internal)

```go
result, err := handler(ctx)

switch {
case err != nil:
    response = api.NewResponse().SetError(err)
case result == nil:
    response = api.NewResponse()  // 200 OK, no body
case isCustomResponse(result):
    response = result.(*api.Response)  // file download, redirect, etc.
default:
    response = api.NewResponse().SetData(result)
}
```

All responses automatically include `X-Trace-ID` as a response header (not in the JSON body).
This makes trace IDs accessible without parsing the response payload. The trace ID is set
before the handler runs and flows through the entire request lifecycle.

**Response headers (auto-injected):**

| Header | Value | Description |
|--------|-------|-------------|
| `X-Trace-ID` | UUID/trace string | Tracks request across services (NR/OTel) |

### 6. Controller Example (Before/After)

**Before:**
```go
// Route
api.NewRoute("POST", a.Insert, api.WithPath("/absence"),
    api.WithMiddleware(a.UseMiddleware("audit_log")))

// Handler — 13 lines
func (a *absence) Insert(req api.Request, ctx context.Context) *api.Response {
    var requestData employeemodel.AbsenceCreatePayload
    if err := req.GetBody(&requestData); err != nil {
        return api.NewResponse().SetError(err)
    }
    data, err := a.uc.Insert(ctx, &requestData)
    if err != nil {
        return api.NewResponse().SetError(err)
    }
    return api.NewResponse().SetData(data)
}
```

**After:**
```go
// Route
api.NewRoute("POST", a.Insert, api.WithPath("/absence"),
    api.WithRequest(new(employeemodel.AbsenceCreatePayload)),
    api.WithMiddleware(a.UseMiddleware("audit_log")))

// Handler — 3 lines
func (a *absence) Insert(ctx context.Context) (any, error) {
    return a.uc.Insert(ctx)
}

// Usecase
func (uc *absenceUsecase) Insert(ctx context.Context) (any, error) {
    payload := phastosctx.Request[*employeemodel.AbsenceCreatePayload](ctx)
    // business logic here — data already clean & validated
}
```

**PUT with typed path param:**
```go
// Route
api.NewRoute("PUT", a.Update,
    api.WithPath("/{id:int64}"),
    api.WithRequest(new(models.Account)))

// Handler
func (a *master) Update(ctx context.Context) (any, error) {
    return a.uc.Update(ctx)
}

// Usecase
func (uc *accountUsecase) Update(ctx context.Context) (any, error) {
    payload := phastosctx.Request[*models.Account](ctx)
    id := phastosctx.PathParam[int64](ctx, "id")
    // id is already validated int64
}
```

**DELETE — error-only return:**
```go
// Handler
func (a *master) Delete(ctx context.Context) (any, error) {
    return nil, a.uc.Delete(ctx)
}
```

**File download — custom response:**
```go
// Handler
func (a *event) ExportReport(ctx context.Context) (any, error) {
    buf, err := a.uc.GenerateExcel(ctx)
    if err != nil {
        return nil, err
    }
    return api.NewResponse().SetFileDownload(buf.Bytes(), "report.xlsx"), nil
}
```

### 7. Backward Compatibility

Phastos detects handler signature at route registration:

```go
func registerHandler(...) {
    // Inspect handler signature
    switch h := handler.(type) {
    case Handler2:   // func(ctx Context) (any, error)
        // New flow: auto-bind → store in context → call handler → wrap response
    default:          // func(Request, Context) *Response
        // Old flow: manual binding (unchanged)
    }
}
```

Old handlers continue to work without any changes. Migration is per-handler, not all-or-nothing.

### 8. OpenAPI Integration

`WithRequest`, `WithQuery`, and typed `WithPath` annotations automatically enrich the OpenAPI spec:

- `WithRequest(T)` → request body schema from `T`
- `WithResponse(T)` → 200 response schema from `T`
- `WithQuery(T)` → query parameter schemas from `T`
- `WithPath("/{id:int64}")` → path parameter with `type: integer, format: int64`

No duplicate type definitions needed — the annotations serve both binding AND documentation.

> **Note:** `WithResponse` must be explicitly set for the 200 response schema to appear in Swagger UI. It is NOT auto-detected from the usecase return type — the handler signature `func(ctx) (any, error)` erases the concrete type. `WithResponse` is purely a documentation annotation; at runtime, phastos serializes the return value via `SetData(result)` regardless.

### 9. Auto Error Responses

Error responses are auto-generated based on route configuration. Each response includes the standard response body schema (`api.Response` with `message`, `code`, `data` fields) so API consumers can see the contract in Swagger UI.

| Status | Trigger Condition | Auto-Generated? |
|--------|------------------|:---:|
| **200** | Success | Dari `WithResponse` |
| **400** | Binding/validation gagal (body, query, path param) | Auto jika route punya `WithRequest`, `WithQuery`, atau `WithPath` |
| **401** | No/invalid auth token | Auto jika route punya SecurityScheme (middleware auth) |
| **403** | Forbidden access (role/group/permission) | Auto jika middleware authorization terdaftar |
| **422** | Business logic / processing error | Auto (selalu ada, catch-all) |
| **500** | Unhandled server error | Auto (selalu ada) |

**409 Conflict** tetap per-route explicit via `api.WithErrorResponse(409, "DUPLICATE_ENTRY", "Resource already exists")`.

Auto-generated error response sample in Swagger UI:
```json
// 400 — Binding Error
{
  "message": "validation failed: field 'email' is required",
  "code": "ERROR_VALIDATION",
  "data": null
}

// 401 — Unauthorized
{
  "message": "missing or invalid authorization",
  "code": "UNAUTHORIZED",
  "data": null
}
```

All responses (including errors) include a `X-Trace-ID` header for request tracing:

```
X-Trace-ID: a1b2c3d4-e5f6-7890-abcd-ef1234567890
```

### 10. Migration Strategy

1. **Phase 1 — Phastos**: Add `Handler2`, context package (`phastosctx`), auto-binding logic, typed `WithPath`, backward compat layer
2. **Phase 2 — timetraq**: Migrate one controller as pilot (e.g., `absence`) — validate no regressions
3. **Phase 3 — Rollout**: Migrate remaining controllers incrementally

## Non-Goals

- Removing `api.Request` entirely (needed for backward compat + advanced use cases like raw body access)
- Auto-binding for multipart/mixed content types (retain manual `req.GetFile()`)

## Key Decisions

1. **Handler = `func(ctx) (any, error)`** — minimal signature, phastos handles the rest
2. **Clean data guarantee** — `phastosctx.Request[T]` never returns error because binding happens before handler
3. **Inline path param types** — `{id:int64}` in the path string, zero additional API surface
4. **Backward compatible** — old handlers unchanged, new handlers opt-in
