# Route Group — Nested Group Routes for Phastos Controller

**Date:** 2026-07-03
**Status:** Draft
**Author:** brainstorming session

## Problem

Currently `ControllerConfig.Routes` hanya mendukung flat list of `Route`. Setiap route harus menulis full path relatif terhadap `ControllerConfig.Path`. Untuk endpoint dengan prefix yang sama (e.g. `/absence`, `/absence/today`, `/absence/{id}`), terjadi repetisi path yang tidak perlu.

Belum ada mekanisme untuk:
- Grouping route dengan sub-prefix bersama
- Middleware yang diinherit ke semua route dalam satu grup
- Nested group (group di dalam group)

## Data Model

### Route struct — tambahan field `SubRoutes`

```go
type Route struct {
    Method      string                                // kosong untuk group route
    Path        string                                // prefix group / path endpoint
    Handler     Handler                               // nil untuk group route
    Version     int                                   // default 1
    Middlewares *[]func(http.Handler) http.Handler     // group → inherit ke child
    SubRoutes   []Route                               // nil/empty untuk leaf route
}
```

Aturan:
- `SubRoutes` len > 0 → **group route**: `Method` dan `Handler` diabaikan, `Path` jadi prefix
- `SubRoutes` empty/nil → **leaf route**: wajib `Method` + `Handler` (existing behavior)
- `NewRoute()` tetap untuk leaf route — backward compatible

### Helper `NewGroup`

```go
func NewGroup(path string, subRoutes []Route, opts ...RouteOption) Route {
    r := Route{
        Path:      path,
        SubRoutes: subRoutes,
    }
    for _, opt := range opts {
        opt(&r)
    }
    return r
}
```

Kedua bentuk valid — struct literal langsung atau `NewGroup`.

## Pemakaian

```go
api.ControllerConfig{
    Path: "/employee",
    Routes: []api.Route{
        api.NewGroup("/absence", []api.Route{
            api.NewRoute("GET", a.GetList),
            api.NewRoute("GET", a.GetAbsenceToday, api.WithPath("/today")),
            api.NewRoute("GET", a.GetDetailById, api.WithPath("/{id}")),
            api.NewRoute("POST", a.Insert, api.WithMiddleware(a.UseMiddleware("audit_log"))),
            api.NewRoute("PUT", a.Update, api.WithPath("/{id}"), api.WithMiddleware(a.UseMiddleware("audit_log"))),
            api.NewRoute("DELETE", a.Delete, api.WithPath("/{id}"), api.WithMiddleware(a.UseMiddleware("audit_log"))),
        }, api.WithMiddleware(a.UseMiddleware("audit_log"))),
    },
}
```

Nested group:
```go
api.NewGroup("/absence", []api.Route{
    api.NewRoute("GET", a.GetList),
    api.NewGroup("/attendance", []api.Route{
        api.NewRoute("GET", a.GetSummary, api.WithPath("/summary")),
    }),
}, api.WithMiddleware(a.UseMiddleware("audit_log"))),
```

## Route Registration — Rekursif

### `AddController`

```go
func (app *App) AddController(ctrl Controller) {
    if impl, ok := ctrl.(interface{ SetRegisteredMiddlewares(map[string]any) }); ok {
        impl.SetRegisteredMiddlewares(app.middlewares)
    }
    config := ctrl.GetConfig()
    app.registerRoutes(config.Path, config.Middlewares, config.Routes)
}
```

### `registerRoutes` — method baru

```go
func (app *App) registerRoutes(
    prefix string,
    parentMiddlewares *[]func(http.Handler) http.Handler,
    routes []Route,
) {
    for _, route := range routes {
        fullPath := prefix + route.Path

        if len(route.SubRoutes) > 0 {
            // Group: merge middlewares, recurse
            merged := mergeMiddlewares(parentMiddlewares, route.Middlewares)
            app.registerRoutes(fullPath, merged, route.SubRoutes)
            continue
        }

        // Leaf route
        var middlewares []func(http.Handler) http.Handler
        if parentMiddlewares != nil {
            middlewares = append(middlewares, *parentMiddlewares...)
        }
        if route.Middlewares != nil {
            middlewares = append(middlewares, *route.Middlewares...)
        }
        routePath := route.GetVersionedPath(prefix)
        app.registerHandler(route.Method, routePath, route.Handler, middlewares...)
    }
}
```

### Middleware merge

```go
func mergeMiddlewares(a, b *[]func(http.Handler) http.Handler) *[]func(http.Handler) http.Handler {
    var merged []func(http.Handler) http.Handler
    if a != nil {
        merged = append(merged, *a...)
    }
    if b != nil {
        merged = append(merged, *b...)
    }
    return &merged
}
```

**Order aplikasi middleware:** Controller → Group → Route

### `GetVersionedPath` — unchanged

Method ini tetap sama. `prefix` yang masuk sekarang sudah include akumulasi path dari semua parent group.

## Edge Cases

| Skenario | Perilaku |
|----------|----------|
| Group dengan `SubRoutes` kosong | Warning log, skip — no endpoint registered |
| Group dengan `Path` kosong (`""`) | Tetap jalan — grouping middleware tanpa sub-path |
| Leaf route tanpa `Method` | Error di `registerHandler` (existing behavior) |
| Group tanpa `Path` + tanpa `SubRoutes` | Warning + skip |
| Hanya group (0 leaf route) | Warning, 0 endpoint registered |

## Testing

Test cases baru di `go/api/app_extended_test.go`:

1. **Group flattening** — 1 group, 3 leaf routes → verify path `/v1/employee/absence`, `/v1/employee/absence/today`, `/v1/employee/absence/{id}`
2. **Nested group** — verify path akumulasi: `/v1/employee/absence/attendance/summary`
3. **Middleware inheritance** — verify urutan: controller → group → route
4. **Empty group** — warning, no panic, no endpoint
5. **Mixed flat + group** — existing flat route + group route di controller sama
6. **Backward compat** — ControllerConfig tanpa SubRoutes tetap jalan (regression)
7. **Fasthttp parallel** — test yang sama buat `FastRoute`

## Files Changed

| File | Change |
|------|--------|
| `go/api/controller.go` | Tambah field `SubRoutes` ke `Route`, tambah `NewGroup()` |
| `go/api/app.go` | Tambah `registerRoutes()` + `mergeMiddlewares()`, modif `AddController()` |
| `go/api/fasthttp_app.go` | Tambah `SubRoutes` ke `FastRoute`, traversal rekursif |
| `go/api/app_extended_test.go` | Test cases baru |
| `go/api/fasthttp_coverage_test.go` | Test cases parallel fasthttp |

## Non-Goals

- WebSocket route grouping
- Regex path patterns di group
- Dynamic path prefix (runtime)
