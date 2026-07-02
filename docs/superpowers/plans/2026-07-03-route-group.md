# Route Group Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add recursive nested route groups to phastos `Route`/`FastRoute` with group-level middleware inheritance.

**Architecture:** Add `SubRoutes` field to `Route` (chi) and `FastRoute` (fasthttp). Extract recursive `registerRoutes()` from `AddController()` to handle tree traversal with middleware merging. Add `NewGroup()` helper. Both chi and fasthttp paths get parallel changes.

**Tech Stack:** Go, chi, fasthttp

## Global Constraints

- `SubRoutes` len > 0 → group route; `SubRoutes` empty → leaf route (backward compatible)
- Group routes: `Method` and `Handler` are ignored, only `Path` and `Middlewares` used
- Leaf routes without `SubRoutes`: existing validation unchanged
- Middleware order: Controller → Group → Route (applied in that order for chi; reverse for fasthttp middleware wrapping)
- Both chi and fasthttp paths must work identically

---
### Task 1: Add SubRoutes + NewGroup to Route (controller.go)

**Files:**
- Modify: `go/api/controller.go:36-42`
- No test changes (tested in Task 3)

**Interfaces:**
- Consumes: existing `Route` struct, `RouteOption`, `WithMiddleware`
- Produces: `Route` with `SubRoutes` field, `NewGroup()` function

- [ ] **Add `SubRoutes` field to `Route` struct**

```go
type Route struct {
	Method      string
	Path        string
	Handler     Handler
	Version     int
	Middlewares *[]func(http.Handler) http.Handler
	SubRoutes   []Route
}
```

- [ ] **Add `NewGroup` helper function**

```go
// NewGroup creates a group route with sub-routes sharing a common path prefix.
// Path is empty it defaults to the parent's current prefix.
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

- [ ] **Commit**

```bash
git add go/api/controller.go
git commit -m "feat(api): add SubRoutes field and NewGroup helper to Route"
```

---
### Task 2: Add SubRoutes + NewGroup to FastRoute (fasthttp_app.go)

**Files:**
- Modify: `go/api/fasthttp_app.go`

**Interfaces:**
- Consumes: existing `FastRoute` struct, `FastControllerConfig`
- Produces: `FastRoute` with `SubRoutes` field, same `NewGroup`-style pattern

- [ ] **Add `SubRoutes` field to `FastRoute`**

```go
type FastRoute struct {
	Method        string
	Path          string
	Handler       FastHandler
	DirectHandler FastDirectHandler
	Version       int
	Middlewares   []FastMiddleware
	SubRoutes     []FastRoute
}
```

- [ ] **Commit**

```bash
git add go/api/fasthttp_app.go
git commit -m "feat(api): add SubRoutes field to FastRoute for fasthttp"
```

---
### Task 3: Recursive registerRoutes for chi path (app.go)

**Files:**
- Modify: `go/api/app.go:610-629`
- No test changes (tested in Task 5)

**Interfaces:**
- Consumes: `Route` with `SubRoutes`, existing `registerHandler`
- Produces: `registerRoutes(prefix, parentMiddlewares, routes)` method, `mergeMiddlewares()` helper

- [ ] **Add `mergeMiddlewares` helper function**

```go
// mergeMiddlewares merges two middleware slices, preserving order.
// Parent (controller/outer) first, then child (group/inner).
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

- [ ] **Add `registerRoutes` recursive method**

```go
// registerRoutes recursively registers routes, handling nested groups.
// prefix accumulates path from parent groups; parentMiddlewares accumulates middleware chain.
func (app *App) registerRoutes(prefix string, parentMiddlewares *[]func(http.Handler) http.Handler, routes []Route) {
	log := plog.Get()
	for _, route := range routes {
		fullPath := prefix + route.Path

		if len(route.SubRoutes) > 0 {
			merged := mergeMiddlewares(parentMiddlewares, route.Middlewares)
			app.registerRoutes(fullPath, merged, route.SubRoutes)
			continue
		}

		if route.SubRoutes != nil {
			log.Warn().Str("path", fullPath).Msg("route group has no sub-routes, skipping")
			continue
		}

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

- [ ] **Replace `AddController` body to delegate to `registerRoutes`**

```go
func (app *App) AddController(ctrl Controller) {
	if impl, ok := ctrl.(interface{ SetRegisteredMiddlewares(map[string]any) }); ok {
		impl.SetRegisteredMiddlewares(app.middlewares)
	}
	config := ctrl.GetConfig()
	app.registerRoutes(config.Path, config.Middlewares, config.Routes)
}
```

- [ ] **Commit**

```bash
git add go/api/app.go
git commit -m "feat(api): add recursive registerRoutes with group middleware merge"
```

---
### Task 4: Recursive registerRoutes for fasthttp path (fasthttp_app.go)

**Files:**
- Modify: `go/api/fasthttp_app.go:432-460`

**Interfaces:**
- Consumes: `FastRoute` with `SubRoutes`
- Produces: same traversal behavior as chi path

- [ ] **Add `registerFastRoutes` recursive method**

```go
func (app *FastHttpApp) registerFastRoutes(prefix string, parentMiddlewares []FastMiddleware, routes []FastRoute) {
	for _, route := range routes {
		fullPath := prefix + route.Path

		if len(route.SubRoutes) > 0 {
			merged := append(parentMiddlewares, route.Middlewares...)
			app.registerFastRoutes(fullPath, merged, route.SubRoutes)
			continue
		}

		if route.SubRoutes != nil {
			plog.Get().Warn().Str("path", fullPath).Msg("route group has no sub-routes, skipping")
			continue
		}

		routePath := route.GetVersionedPath(prefix)

		var handler fasthttp.RequestHandler
		if route.DirectHandler != nil {
			handler = app.wrapDirectHandler(route.DirectHandler)
		} else {
			handler = app.wrapFastHandler(route.Handler)
		}

		allMiddlewares := append(parentMiddlewares, route.Middlewares...)
		for i := len(allMiddlewares) - 1; i >= 0; i-- {
			handler = allMiddlewares[i](handler)
		}

		app.router.Handle(route.Method, routePath, handler)
		app.TotalEndpoints++
	}
}
```

- [ ] **Replace `AddController` body**

```go
func (app *FastHttpApp) AddController(ctrl FastController) {
	app.flushPendingMiddlewares()

	config := ctrl.GetConfig()
	app.registerFastRoutes(config.Path, config.Middlewares, config.Routes)
}
```

- [ ] **Commit**

```bash
git add go/api/fasthttp_app.go
git commit -m "feat(api): add recursive registerFastRoutes for fasthttp route groups"
```

---
### Task 5: Tests for chi route groups (app_extended_test.go)

**Files:**
- Modify: `go/api/app_extended_test.go`
- Modify: `go/api/app_test.go` (backward compat)

- [ ] **Add Group flattening test in app_test.go**

```go
func TestApp_AddController_WithRouteGroup(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("hello")
	}
	mw := func(next http.Handler) http.Handler { return next }

	ctrl := &mockController{
		config: ControllerConfig{
			Path: "/employee",
			Routes: []Route{
				{
					Path: "/absence",
					SubRoutes: []Route{
						NewRoute(http.MethodGet, handler, WithPath("/list")),
						NewRoute(http.MethodGet, handler, WithPath("/today")),
						NewRoute(http.MethodGet, handler, WithPath("/{id}")),
					},
					Middlewares: &[]func(http.Handler) http.Handler{mw},
				},
			},
		},
	}

	app.AddController(ctrl)
	assert.Equal(t, 4, app.TotalEndpoints) // 3 group routes + /ping
}
```

- [ ] **Add Nested group test**

```go
func TestApp_AddController_WithNestedRouteGroup(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("hello")
	}

	ctrl := &mockController{
		config: ControllerConfig{
			Path: "/employee",
			Routes: []Route{
				{
					Path: "/absence",
					SubRoutes: []Route{
						NewRoute(http.MethodGet, handler),
						{
							Path: "/attendance",
							SubRoutes: []Route{
								NewRoute(http.MethodGet, handler, WithPath("/summary")),
							},
						},
					},
				},
			},
		},
	}

	app.AddController(ctrl)
	assert.Equal(t, 3, app.TotalEndpoints) // 2 leaf routes + /ping
}
```

- [ ] **Add Middleware inheritance test**

```go
func TestApp_AddController_GroupMiddlewareInheritance(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("hello")
	}
	groupMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Group-MW", "applied")
			next.ServeHTTP(w, r)
		})
	}

	ctrl := &mockController{
		config: ControllerConfig{
			Path: "/test",
			Routes: []Route{
				{
					Path: "/group",
					SubRoutes: []Route{
						NewRoute(http.MethodGet, handler, WithPath("/leaf")),
					},
					Middlewares: &[]func(http.Handler) http.Handler{groupMW},
				},
			},
		},
	}

	app.AddController(ctrl)
	app.Start()

	req := httptest.NewRequest(http.MethodGet, "/v1/test/group/leaf", nil)
	w := httptest.NewRecorder()
	app.Handler.ServeHTTP(w, req)

	assert.Equal(t, "applied", w.Header().Get("X-Group-MW"))
	assert.Equal(t, 2, app.TotalEndpoints) // 1 route + /ping
}
```

- [ ] **Add Empty group test**

```go
func TestApp_AddController_EmptyRouteGroup(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	ctrl := &mockController{
		config: ControllerConfig{
			Path: "/test",
			Routes: []Route{
				{
					Path:      "/empty",
					SubRoutes: []Route{},
				},
			},
		},
	}

	app.AddController(ctrl)
	assert.Equal(t, 1, app.TotalEndpoints) // only /ping
}
```

- [ ] **Add Mixed flat + group test**

```go
func TestApp_AddController_MixedFlatAndGroup(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("hello")
	}

	ctrl := &mockController{
		config: ControllerConfig{
			Path: "/api",
			Routes: []Route{
				NewRoute(http.MethodGet, handler, WithPath("/health")),
				{
					Path: "/users",
					SubRoutes: []Route{
						NewRoute(http.MethodGet, handler, WithPath("/list")),
					},
				},
			},
		},
	}

	app.AddController(ctrl)
	assert.Equal(t, 3, app.TotalEndpoints) // 2 routes + /ping
}
```

- [ ] **Add NewGroup helper test**

```go
func TestApp_AddController_WithNewGroupHelper(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()

	handler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("hello")
	}

	ctrl := &mockController{
		config: ControllerConfig{
			Path: "/api",
			Routes: []Route{
				NewGroup("/items", []Route{
					NewRoute(http.MethodGet, handler, WithPath("/list")),
				}),
			},
		},
	}

	app.AddController(ctrl)
	assert.Equal(t, 2, app.TotalEndpoints) // 1 route + /ping
}
```

- [ ] **Run all chi tests**

Run: `cd go && go test ./api/ -run "TestApp_AddController" -v -count=1`
Expected: All existing + new tests PASS

- [ ] **Commit**

```bash
git add go/api/app_test.go go/api/app_extended_test.go
git commit -m "test(api): add route group test cases for chi path"
```

---
### Task 6: Tests for fasthttp route groups (fasthttp_coverage_test.go)

**Files:**
- Modify: `go/api/fasthttp_app_test.go`
- Modify: `go/api/fasthttp_coverage_test.go`

- [ ] **Add fasthttp group flattening test**

```go
func TestFastHttpApp_AddController_WithRouteGroup(t *testing.T) {
	app := newTestFastApp()
	app.Init()
	app.InitServer()

	handler := func(req FastRequest, ctx context.Context) *Response {
		return NewResponse().SetMessage("hello")
	}
	mw := func(next fasthttp.RequestHandler) fasthttp.RequestHandler { return next }

	ctrl := &mockFastController{
		config: FastControllerConfig{
			Path: "/employee",
			Routes: []FastRoute{
				{
					Path: "/absence",
					SubRoutes: []FastRoute{
						NewFastRoute(http.MethodGet, handler, WithPath("/list")),
						NewFastRoute(http.MethodGet, handler, WithPath("/today")),
					},
					Middlewares: []FastMiddleware{mw},
				},
			},
		},
	}

	app.AddController(ctrl)
	assert.Equal(t, 3, app.TotalEndpoints) // 2 routes + /ping
}
```

- [ ] **Add fasthttp nested group test**

```go
func TestFastHttpApp_AddController_WithNestedRouteGroup(t *testing.T) {
	app := newTestFastApp()
	app.Init()
	app.InitServer()

	handler := func(req FastRequest, ctx context.Context) *Response {
		return NewResponse().SetMessage("hello")
	}

	ctrl := &mockFastController{
		config: FastControllerConfig{
			Path: "/employee",
			Routes: []FastRoute{
				{
					Path: "/absence",
					SubRoutes: []FastRoute{
						NewFastRoute(http.MethodGet, handler),
						{
							Path: "/attendance",
							SubRoutes: []FastRoute{
								NewFastRoute(http.MethodGet, handler, WithPath("/summary")),
							},
						},
					},
				},
			},
		},
	}

	app.AddController(ctrl)
	assert.Equal(t, 3, app.TotalEndpoints) // 2 routes + /ping
}
```

- [ ] **Add fasthttp empty group test**

```go
func TestFastHttpApp_AddController_EmptyRouteGroup(t *testing.T) {
	app := newTestFastApp()
	app.Init()
	app.InitServer()

	ctrl := &mockFastController{
		config: FastControllerConfig{
			Path: "/test",
			Routes: []FastRoute{
				{
					Path:      "/empty",
					SubRoutes: []FastRoute{},
				},
			},
		},
	}

	app.AddController(ctrl)
	assert.Equal(t, 1, app.TotalEndpoints) // only /ping
}
```

- [ ] **Run all fasthttp tests**

Run: `cd go && go test ./api/ -run "TestFastHttp" -v -count=1`
Expected: All existing + new tests PASS

- [ ] **Commit**

```bash
git add go/api/fasthttp_app_test.go go/api/fasthttp_coverage_test.go
git commit -m "test(api): add route group test cases for fasthttp path"
```

---
### Task 7: Final verification & lint

- [ ] **Run all tests**

Run: `cd go && go test ./api/ -v -count=1`
Expected: All tests PASS

- [ ] **Run linter**

Run: `cd go && golangci-lint run ./api/`
Expected: No new lint errors

- [ ] **Run build**

Run: `cd go && go build ./...`
Expected: Build succeeds

- [ ] **Run full test suite**

Run: `cd go && go test ./... -count=1`
Expected: All packages pass

- [ ] **Commit any final fixes**

```bash
git add -A
git commit -m "chore(api): final cleanup after route group implementation"
```
