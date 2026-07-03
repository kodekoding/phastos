# OpenAPI Docs — Auto-Generated API Documentation for Phastos

**Date:** 2026-07-03
**Status:** Draft

## Problem

Currently phastos tidak memiliki mekanisme untuk menghasilkan API documentation secara otomatis. Developer harus menulis dokumentasi secara manual atau menggunakan tools eksternal. Padahal semua informasi yang dibutuhkan (routes, handlers, middleware) sudah tersedia di `ControllerConfig` dan `Route`.

## Solution

Generate OpenAPI 3.0.3 spec otomatis dari registered routes. Metadata endpoint didefinisikan sebagai `RouteDoc` yang nempel di `Route`, middleware metadata sebagai `MiddlewareInfo` yang nempel di registry. Generator jalan di `Start()` dan serve `/docs` (Swagger UI) + `/docs/openapi.json` (JSON spec).

## Data Model

### RouteDoc — metadata per endpoint

```go
type RouteDoc struct {
    Summary        string
    Description    string
    Tags           []string
    Deprecated     bool
    RequestType    any              // Go struct untuk request body
    ResponseType   any              // Go struct untuk response body
    ErrorResponses []ErrorResponseDoc
    Headers        []HeaderDoc     // auto-merge dari middleware registry
    Security       *SecuritySchemeDoc // auto-merge dari middleware registry
}

type ErrorResponseDoc struct {
    StatusCode  int
    Code        string
    Description string
}
```

### Route — tambahan field Doc

```go
type Route struct {
    Method      string
    Path        string
    Handler     Handler
    Version     int
    Middlewares *[]func(http.Handler) http.Handler
    SubRoutes   []Route
    Doc         *RouteDoc   // nil = skip dari OpenAPI spec
}
```

### RouteOption helpers baru

```go
func WithSummary(s string) RouteOption
func WithDescription(s string) RouteOption
func WithTags(tags ...string) RouteOption
func WithRequest(req any) RouteOption
func WithResponse(resp any) RouteOption
func WithErrorResponse(status int, code string, desc string) RouteOption
```

### MiddlewareInfo — metadata middleware di registry

```go
type MiddlewareInfo struct {
    Description    string
    SecurityScheme *SecuritySchemeDoc
    Headers        []HeaderDoc
}

type SecuritySchemeDoc struct {
    Type string // "bearer", "apiKey", "basic"
    Name string // "Authorization"
    In   string // "header", "query", "cookie"
}

type HeaderDoc struct {
    Name        string
    Description string
    Required    bool
    Type        string // "string" default
}

type MiddlewareOption func(*MiddlewareInfo)

func WithSecurity(schemeType, name, in string) MiddlewareOption
func WithRequiredHeader(name, description string, required bool) MiddlewareOption
func WithMiddlewareDescription(desc string) MiddlewareOption
```

## Middleware Metadata — Registration

```go
// Variadic option — backward compatible
func (app *App) RegisterMiddlewareFunc(key string, handler func(http.Handler) http.Handler, opts ...MiddlewareOption) {
    app.middlewares[key] = handler
    if len(opts) > 0 {
        info := MiddlewareInfo{}
        for _, opt := range opts {
            opt(&info)
        }
        app.middlewareDocs[key] = info
    }
}
```

Pemakaian:

```go
app.RegisterMiddlewareFunc("auth_jwt", middlewares.JWTAuth,
    WithSecurity("bearer", "Authorization", "header"),
    WithMiddlewareDescription("JWT Bearer authentication"),
)

app.RegisterMiddlewareFunc("validate_tenant", tenantMW,
    WithRequiredHeader("X-Tenant-ID", "Tenant identifier", true),
)
```

## Auto-Inject — ControllerImpl Tracking

### ControllerImpl — rekam key otomatis

```go
type ControllerImpl struct {
    registeredMiddlewares map[string]any
    usedMiddlewareKeys    []string
}

func (ctrl *ControllerImpl) UseMiddleware(key string) func(http.Handler) http.Handler {
    ctrl.usedMiddlewareKeys = append(ctrl.usedMiddlewareKeys, key)
    // ... return handler (existing)
}

func (ctrl *ControllerImpl) GetUsedMiddlewareKeys() []string {
    return ctrl.usedMiddlewareKeys
}
```

### AddController — merge metadata

```go
func (app *App) AddController(ctrl Controller) {
    // ... existing: inject registered middlewares

    config := ctrl.GetConfig()

    var middlewareKeys []string
    if impl, ok := ctrl.(interface{ GetUsedMiddlewareKeys() []string }); ok {
        middlewareKeys = impl.GetUsedMiddlewareKeys()
    }

    app.registerRoutes(config.Path, config.Middlewares, config.Routes, middlewareKeys)
}
```

### registerRoutes — auto-inject ke RouteDoc

```go
func (app *App) registerRoutes(
    prefix string,
    parentMiddlewares *[]func(http.Handler) http.Handler,
    routes []Route,
    middlewareKeys []string,
) {
    for _, route := range routes {
        fullPath := prefix + route.Path

        if len(route.SubRoutes) > 0 {
            merged := mergeMiddlewares(parentMiddlewares, route.Middlewares)
            app.registerRoutes(fullPath, merged, route.SubRoutes, middlewareKeys)
            continue
        }

        // Auto-inject middleware metadata ke RouteDoc
        if len(middlewareKeys) > 0 {
            if route.Doc == nil {
                route.Doc = &RouteDoc{}
            }
            for _, key := range middlewareKeys {
                if info, ok := app.middlewareDocs[key]; ok {
                    if info.SecurityScheme != nil {
                        route.Doc.Security = info.SecurityScheme
                    }
                    route.Doc.Headers = mergeHeaders(route.Doc.Headers, info.Headers)
                }
            }
        }

        // ... existing: register handler
    }
}
```

## OpenAPI — Entry Point

```go
func WithOpenAPI() Options {
    return func(app *App) {
        app.enableOpenAPI = true
    }
}
```

Pas `Init()`, jika `enableOpenAPI`, jalanin `buildOpenAPISpec()` yang:
1. Iterate `app.routeRegistry` (accumulated during registerRoutes)
2. Convert tiap entry ke `openapi3.PathItem`
3. Generate JSON Schema dari Go struct (reflection + `json` + `validate` tags)
4. Simpan hasil di `app.openAPISpec`

Pas `Start()`:
- Serve `/docs` → Swagger UI HTML (fetch spec from `/docs/openapi.json`)
- Serve `/docs/openapi.json` → raw JSON spec

## Schema Generation

Go struct → OpenAPI Schema Object via reflection:

```go
type InsertRequest struct {
    EmployeeID int64  `json:"employee_id" validate:"required"`
    Date       string `json:"date" validate:"required"`
    Reason     string `json:"reason"`
}
```

Hasil:
- `required` fields dari tag `validate:"required"`
- `type` dari Go type (`int64` → `"integer"`, `string` → `"string"`)
- `description` dari tag `json`
- Support nested struct, `time.Time` → `"string" format:"date-time"`
- Support pointer types sebagai nullable fields

## Pemakaian Akhir

```go
// Registry (1x)
app.RegisterMiddlewareFunc("auth_jwt", middlewares.JWTAuth,
    WithSecurity("bearer", "Authorization", "header"),
)
app.RegisterMiddlewareFunc("validate_tenant", tenantMW,
    WithRequiredHeader("X-Tenant-ID", "Tenant identifier", true),
)

// NewApp — cukup tambah WithOpenAPI()
api.NewApp(
    api.WithOpenAPI(),
    api.WithAppPort(8000),
)

// Controller — existing, nggak berubah
func (a *AbsenceController) GetConfig() api.ControllerConfig {
    return api.ControllerConfig{
        Path: "/employee",
        Routes: []api.Route{
            api.NewRoute("GET", a.GetList,
                api.WithPath("/absence"),
                api.WithSummary("List absences"),
                api.WithTags("Absence"),
            ),
            api.NewRoute("POST", a.Insert,
                api.WithPath("/absence"),
                api.WithSummary("Create absence"),
                api.WithTags("Absence"),
                api.WithRequest(InsertRequest{}),
                api.WithResponse(InsertResponse{}),
                api.WithErrorResponse(422, "VALIDATION_ERROR", "Validation failed"),
            ),
        },
    }
}
```

## Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /docs` | Swagger UI (HTML) |
| `GET /docs/openapi.json` | Raw OpenAPI 3.0.3 spec |

## Edge Cases

| Skenario | Perilaku |
|----------|----------|
| Controller tanpa middleware | Route generasi tanpa security/headers |
| Route tanpa `Doc` (`nil`) | Skip dari OpenAPI spec |
| Middleware tanpa metadata di registry | Skip, gak ngaruh ke route |
| Public controller tanpa `UseMiddleware` | Nggak ada auto-inject |
| Nested struct di request/response | Direkursif sampe primitive types |
| `time.Time` field | OpenAPI type: `string` format: `date-time` |
| Pointer field | Nullable: `true` |
| `validate:"required"` | Field masuk ke `required[]` |

## Non-Goals

- Generate API docs dari source code comments (Swaggo-style)
- Edit/Create/Delete docs via UI
- Auth flow selain `bearer` & `apiKey`
- Export ke Postman collection
