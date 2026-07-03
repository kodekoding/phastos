# OpenAPI Docs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Auto-generate OpenAPI 3.0.3 docs from registered phastos routes, served at `/docs` + `/docs/openapi.json`.

**Architecture:** RouteDoc metadata nempel di `Route`, middleware metadata di `MiddlewareInfo` registry via variadic `RegisterMiddlewareFunc`. `ControllerImpl` otomatis rekam middleware keys. `AddController` auto-inject security/headers ke `RouteDoc`. `buildOpenAPISpec()` generate OpenAPI spec via reflection, serve via Swagger UI.

**Tech Stack:** Go, `github.com/getkin/kin-openapi/openapi3`, Swagger UI (loaded from CDN)

## Global Constraints

- `Route.Doc *RouteDoc` — nil means skip from OpenAPI spec (backward compatible)
- `MiddlewareInfo` middleware metadata — optional variadic on `RegisterMiddlewareFunc`
- ControllerImpl otomatis rekam key lewat `UseMiddleware` — zero code change di controller
- OpenAPI spec dibuild pas `Start()` setelah semua controller terdaftar
- `/docs` serve Swagger UI (CDN), `/docs/openapi.json` serve raw spec
- Hanya chi path yang serve `/docs` (fasthttp path: data model aja, serving skipped)
- Schema generation dari Go struct: `json` tag → property name, `validate:"required"` → required fields
- `time.Time` → `string` format `date-time`, pointer → nullable, nested struct → `$ref`

---
### Task 1: RouteDoc data model + RouteOption helpers + MiddlewareInfo registry

**Files:**
- Modify: `go/api/controller.go`
- Modify: `go/api/app.go`

**Interfaces:**
- Consumes: existing `Route` struct, `RouteOption`, `App`
- Produces: `RouteDoc`, `MiddlewareInfo` types, route/middleware option helpers

**Step 1: Add types to controller.go**

Tambah setelah `Route` struct:

```go
type RouteDoc struct {
	Summary        string
	Description    string
	Tags           []string
	Deprecated     bool
	RequestType    any
	ResponseType   any
	ErrorResponses []ErrorResponseDoc
	Headers        []HeaderDoc
	Security       *SecuritySchemeDoc
}

type ErrorResponseDoc struct {
	StatusCode  int
	Code        string
	Description string
}

type SecuritySchemeDoc struct {
	Type string
	Name string
	In   string
}

type HeaderDoc struct {
	Name        string
	Description string
	Required    bool
	Type        string
}

type MiddlewareOption func(*MiddlewareInfo)

type MiddlewareInfo struct {
	Description    string
	SecurityScheme *SecuritySchemeDoc
	Headers        []HeaderDoc
}
```

**Step 2: Add `Doc` field to Route struct**

```go
type Route struct {
	Method      string
	Path        string
	Handler     Handler
	Version     int
	Middlewares *[]func(http.Handler) http.Handler
	SubRoutes   []Route
	Doc         *RouteDoc
}
```

**Step 3: Add RouteOption helpers**

```go
func WithSummary(s string) RouteOption {
	return func(r *Route) {
		if r.Doc == nil {
			r.Doc = &RouteDoc{}
		}
		r.Doc.Summary = s
	}
}

func WithDescription(s string) RouteOption {
	return func(r *Route) {
		if r.Doc == nil {
			r.Doc = &RouteDoc{}
		}
		r.Doc.Description = s
	}
}

func WithTags(tags ...string) RouteOption {
	return func(r *Route) {
		if r.Doc == nil {
			r.Doc = &RouteDoc{}
		}
		r.Doc.Tags = append(r.Doc.Tags, tags...)
	}
}

func WithRequest(req any) RouteOption {
	return func(r *Route) {
		if r.Doc == nil {
			r.Doc = &RouteDoc{}
		}
		r.Doc.RequestType = req
	}
}

func WithResponse(resp any) RouteOption {
	return func(r *Route) {
		if r.Doc == nil {
			r.Doc = &RouteDoc{}
		}
		r.Doc.ResponseType = resp
	}
}

func WithErrorResponse(status int, code string, desc string) RouteOption {
	return func(r *Route) {
		if r.Doc == nil {
			r.Doc = &RouteDoc{}
		}
		r.Doc.ErrorResponses = append(r.Doc.ErrorResponses, ErrorResponseDoc{
			StatusCode:  status,
			Code:        code,
			Description: desc,
		})
	}
}
```

**Step 4: Add MiddlewareOption helpers**

```go
func WithSecurity(schemeType, name, in string) MiddlewareOption {
	return func(m *MiddlewareInfo) {
		m.SecurityScheme = &SecuritySchemeDoc{
			Type: schemeType,
			Name: name,
			In:   in,
		}
	}
}

func WithRequiredHeader(name, description string, required bool) MiddlewareOption {
	return func(m *MiddlewareInfo) {
		m.Headers = append(m.Headers, HeaderDoc{
			Name:        name,
			Description: description,
			Required:    required,
			Type:        "string",
		})
	}
}

func WithMiddlewareDescription(desc string) MiddlewareOption {
	return func(m *MiddlewareInfo) {
		m.Description = desc
	}
}
```

**Step 5: Modify RegisterMiddlewareFunc in app.go — tambah variadic opts + middlewareDocs field di App**

Add to App struct:
```go
middlewareDocs map[string]MiddlewareInfo
```

Initialize in `NewApp`:
```go
apiApp.middlewareDocs = make(map[string]MiddlewareInfo)
```

Replace existing:
```go
func (app *App) RegisterMiddlewareFunc(key string, middlewareHandler func(http.Handler) http.Handler) {
	app.middlewares[key] = middlewareHandler
}
```

With:
```go
func (app *App) RegisterMiddlewareFunc(key string, middlewareHandler func(http.Handler) http.Handler, opts ...MiddlewareOption) {
	app.middlewares[key] = middlewareHandler
	if len(opts) > 0 {
		info := MiddlewareInfo{}
		for _, opt := range opts {
			opt(&info)
		}
		app.middlewareDocs[key] = info
	}
}
```

**Step 6: Build & verify**

Run: `cd go && go build ./...`
Expected: SUCCESS

**Step 7: Commit**

```bash
git add go/api/controller.go go/api/app.go
git commit -m "feat(api): add RouteDoc, MiddlewareInfo types and option helpers"
```

---
### Task 2: ControllerImpl tracking + AddController auto-inject

**Files:**
- Modify: `go/api/controller.go` (ControllerImpl)
- Modify: `go/api/app.go` (AddController, registerRoutes, registerHandler signature)

**Interfaces:**
- Consumes: `ControllerImpl`, `App.registerRoutes`, `middlewareDocs`
- Produces: auto-injected `RouteDoc` with security/headers from middleware registry

**Step 1: Modify ControllerImpl — add key tracking**

```go
type ControllerImpl struct {
	registeredMiddlewares map[string]any
	usedMiddlewareKeys    []string
}

func (ctrl *ControllerImpl) UseMiddleware(key string) func(http.Handler) http.Handler {
	ctrl.usedMiddlewareKeys = append(ctrl.usedMiddlewareKeys, key)
	if ctrl.registeredMiddlewares == nil {
		return nil
	}
	if mw, ok := ctrl.registeredMiddlewares[key]; ok {
		if fn, ok := mw.(func(http.Handler) http.Handler); ok {
			return fn
		}
	}
	return nil
}

func (ctrl *ControllerImpl) GetUsedMiddlewareKeys() []string {
	return ctrl.usedMiddlewareKeys
}
```

**Step 2: Modify AddController — collect middleware keys, pass to registerRoutes**

```go
func (app *App) AddController(ctrl Controller) {
	if impl, ok := ctrl.(interface{ SetRegisteredMiddlewares(map[string]any) }); ok {
		impl.SetRegisteredMiddlewares(app.middlewares)
	}
	config := ctrl.GetConfig()

	var middlewareKeys []string
	if impl, ok := ctrl.(interface{ GetUsedMiddlewareKeys() []string }); ok {
		middlewareKeys = impl.GetUsedMiddlewareKeys()
	}

	app.registerRoutes(config.Path, config.Middlewares, config.Routes, middlewareKeys)
}
```

**Step 3: Modify registerRoutes signature — accept middlewareKeys + auto-inject**

```go
func (app *App) registerRoutes(prefix string, parentMiddlewares *[]func(http.Handler) http.Handler, routes []Route, middlewareKeys []string) {
	log := plog.Get()
	for _, route := range routes {
		fullPath := prefix + route.Path

		if len(route.SubRoutes) > 0 {
			merged := mergeMiddlewares(parentMiddlewares, route.Middlewares)
			app.registerRoutes(fullPath, merged, route.SubRoutes, middlewareKeys)
			continue
		}

		if route.SubRoutes != nil {
			log.Warn().Str("path", fullPath).Msg("route group has no sub-routes, skipping")
			continue
		}

		// Auto-inject middleware metadata into RouteDoc
		if len(middlewareKeys) > 0 {
			if route.Doc == nil {
				route.Doc = &RouteDoc{}
			}
			for _, key := range middlewareKeys {
				if info, ok := app.middlewareDocs[key]; ok {
					if info.SecurityScheme != nil && route.Doc.Security == nil {
						route.Doc.Security = info.SecurityScheme
					}
					route.Doc.Headers = append(route.Doc.Headers, info.Headers...)
				}
			}
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

**Step 4: Build & run existing tests**

Run: `cd go && go build ./... && go test ./api/ -count=1`
Expected: BUILD + TESTS PASS

**Step 5: Commit**

```bash
git add go/api/controller.go go/api/app.go
git commit -m "feat(api): auto-inject middleware metadata into RouteDoc"
```

---
### Task 3: OpenAPI spec generator

**Files:**
- Create: `go/api/openapi.go`
- Modify: `go/go.mod` (add `github.com/getkin/kin-openapi/openapi3`)

**Step 1: Add dependency**

```bash
cd go && go get github.com/getkin/kin-openapi/openapi3
```

**Step 2: Create go/api/openapi.go with buildOpenAPISpec and schema generation**

```go
package api

import (
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

// buildOpenAPISpec generates an OpenAPI 3.0.3 spec from all registered routes.
func (app *App) buildOpenAPISpec() *openapi3.T {
	spec := &openapi3.T{
		OpenAPI: "3.0.3",
		Info: &openapi3.Info{
			Title:   app.getAppName(),
			Version: appVersion,
		},
		Paths: openapi3.NewPaths(),
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{},
		},
	}

	for _, entry := range app.routeRegistry {
		pathItem := spec.Paths.Find(entry.Path)
		if pathItem == nil {
			pathItem = &openapi3.PathItem{}
			spec.Paths.Set(entry.Path, pathItem)
		}
		app.buildOperation(pathItem, entry)
	}

	// Add security scheme components
	for _, info := range app.middlewareDocs {
		if info.SecurityScheme != nil {
			scheme := &openapi3.SecurityScheme{
				Type: info.SecurityScheme.Type,
				Name: info.SecurityScheme.Name,
				In:   info.SecurityScheme.In,
			}
			spec.Components.SecuritySchemes[info.SecurityScheme.Name] = &openapi3.SecuritySchemeRef{
				Value: scheme,
			}
		}
	}

	return spec
}

func (app *App) buildOperation(item *openapi3.PathItem, entry routeRegistryEntry) {
	operation := &openapi3.Operation{
		Summary:     entry.Doc.Summary,
		Description: entry.Doc.Description,
		Tags:        entry.Doc.Tags,
	}

	// Request body
	if entry.Doc.RequestType != nil {
		schema := app.generateSchema(entry.Doc.RequestType)
		operation.RequestBody = &openapi3.RequestBodyRef{
			Value: openapi3.NewRequestBody().
				WithJSONSchema(schema).
				WithRequired(true),
		}
	}

	// Response
	if entry.Doc.ResponseType != nil {
		schema := app.generateSchema(entry.Doc.ResponseType)
		operation.AddResponse(200, &openapi3.Response{
			Description: "Success",
			Content: openapi3.NewContentWithJSONSchema(schema),
		})
	}

	// Error responses
	for _, errResp := range entry.Doc.ErrorResponses {
		statusStr := strconv.Itoa(errResp.StatusCode)
		operation.AddResponse(statusStr, &openapi3.Response{
			Description: errResp.Description,
		})
	}

	// Security
	if entry.Doc.Security != nil {
		operation.Security = &openapi3.SecurityRequirements{
			{entry.Doc.Security.Name: {}},
		}
	}

	// Headers (global)
	if len(entry.Doc.Headers) > 0 {
		params := make([]*openapi3.ParameterRef, 0, len(entry.Doc.Headers))
		for _, h := range entry.Doc.Headers {
			params = append(params, &openapi3.ParameterRef{
				Value: &openapi3.Parameter{
					Name:        h.Name,
					In:          "header",
					Description: h.Description,
					Required:    h.Required,
					Schema: &openapi3.Schema{
						Type: h.Type,
					},
				},
			})
		}
		operation.Parameters = params
	}

	item.SetOperation(entry.Method, operation)
}

// generateSchema converts a Go struct to an OpenAPI Schema via reflection.
func (app *App) generateSchema(model any) *openapi3.Schema {
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return &openapi3.Schema{Type: "object"}
	}

	schema := &openapi3.Schema{
		Type:       "object",
		Properties: openapi3.Schemas{},
	}

	for i := range t.NumField() {
		field := t.Field(i)

		// Skip unexported
		if !field.IsExported() {
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}

		name := strings.Split(jsonTag, ",")[0]
		if name == "" {
			name = field.Name
		}

		propSchema := app.fieldToSchema(field)
		schema.Properties[name] = &openapi3.SchemaRef{
			Value: propSchema,
		}

		// Check validate:"required"
		validateTag := field.Tag.Get("validate")
		if strings.Contains(validateTag, "required") {
			schema.Required = append(schema.Required, name)
		}
	}

	return schema
}

func (app *App) fieldToSchema(field reflect.StructField) *openapi3.Schema {
	t := field.Type

	// Dereference pointer
	nullable := false
	if t.Kind() == reflect.Ptr {
		nullable = true
		t = t.Elem()
	}

	// Special types
	if t == reflect.TypeOf(time.Time{}) {
		return &openapi3.Schema{
			Type:   "string",
			Format: "date-time",
		}
	}

	switch t.Kind() {
	case reflect.String:
		return &openapi3.Schema{Type: "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &openapi3.Schema{Type: "integer"}
	case reflect.Float32, reflect.Float64:
		return &openapi3.Schema{Type: "number"}
	case reflect.Bool:
		return &openapi3.Schema{Type: "boolean"}
	case reflect.Slice, reflect.Array:
		elem := t.Elem()
		items := app.fieldToSchema(reflect.StructField{Type: elem})
		return &openapi3.Schema{
			Type:  "array",
			Items: &openapi3.SchemaRef{Value: items},
		}
	case reflect.Struct:
		nested := app.generateSchema(reflect.New(t).Interface())
		nested.Nullable = nullable
		return nested
	case reflect.Map:
		return &openapi3.Schema{Type: "object"}
	default:
		return &openapi3.Schema{Type: "string"}
	}
}
```

**Step 3: Add routeRegistry accumulator to App**

In `app.go` App struct:
```go
type routeRegistryEntry struct {
	Method string
	Path   string
	Doc    *RouteDoc
}

type App struct {
	// ... existing
	routeRegistry []routeRegistryEntry
}
```

In `registerRoutes`, after `registerHandler` call, append to registry:
```go
if route.Doc != nil {
	app.routeRegistry = append(app.routeRegistry, routeRegistryEntry{
		Method: route.Method,
		Path:   routePath,
		Doc:    route.Doc,
	})
}
```

**Step 4: Build**

Run: `cd go && go build ./...`
Expected: SUCCESS

**Step 5: Commit**

```bash
git add go/api/openapi.go go/api/app.go go/go.mod go/go.sum
git commit -m "feat(api): add OpenAPI spec generator with schema reflection"
```

---
### Task 4: Serve /docs + /docs/openapi.json

**Files:**
- Modify: `go/api/app.go`

**Step 1: Add WithOpenAPI option and required fields to App**

In App struct:
```go
enableOpenAPI bool
```

In NewApp, no additional init needed (zero value = false).

```go
func WithOpenAPI() Options {
	return func(app *App) {
		app.enableOpenAPI = true
	}
}
```

**Step 2: Modify Start() — build spec and serve endpoints**

After `app.flushPendingMiddlewares()` and before `app.Handler = InitHandler(...)`:

```go
if app.enableOpenAPI {
	spec := app.buildOpenAPISpec()

	// Serve raw JSON spec
	app.Http.Get("/docs/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(spec)
	})

	// Serve Swagger UI (loads from CDN, fetches spec from /docs/openapi.json)
	app.Http.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>API Docs</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: '/docs/openapi.json',
      dom_id: '#swagger-ui',
    })
  </script>
</body>
</html>`
		w.Write([]byte(html))
	})
}
```

**Step 3: Build & test**

Run:
```bash
cd go && go build ./...
cd go && go test ./api/ -count=1
```
Expected: BUILD + TESTS PASS

**Step 4: Commit**

```bash
git add go/api/app.go
git commit -m "feat(api): serve OpenAPI docs at /docs + /docs/openapi.json"
```

---
### Task 5: Tests

**Files:**
- Modify: `go/api/app_test.go`
- Modify: `go/api/app_extended_test.go`

**Step 1: Add RouteDoc option helper tests in app_test.go**

```go
func TestRouteDoc_WithSummary(t *testing.T) {
	route := NewRoute("GET", nil, WithSummary("List items"))
	require.NotNil(t, route.Doc)
	assert.Equal(t, "List items", route.Doc.Summary)
}

func TestRouteDoc_WithTags(t *testing.T) {
	route := NewRoute("GET", nil, WithTags("Users", "Admin"))
	require.NotNil(t, route.Doc)
	assert.Equal(t, []string{"Users", "Admin"}, route.Doc.Tags)
}

func TestRouteDoc_WithRequest(t *testing.T) {
	type Req struct {
		Name string `json:"name"`
	}
	route := NewRoute("POST", nil, WithRequest(Req{}))
	require.NotNil(t, route.Doc)
	assert.Equal(t, Req{}, route.Doc.RequestType)
}

func TestRouteDoc_WithErrorResponse(t *testing.T) {
	route := NewRoute("GET", nil,
		WithErrorResponse(422, "VALIDATION_ERROR", "Validation failed"),
	)
	require.NotNil(t, route.Doc)
	assert.Len(t, route.Doc.ErrorResponses, 1)
	assert.Equal(t, 422, route.Doc.ErrorResponses[0].StatusCode)
}

func TestRouteDoc_MultipleOptions(t *testing.T) {
	route := NewRoute("GET", nil,
		WithSummary("Get item"),
		WithTags("Items"),
		WithDescription("Returns a single item by ID"),
	)
	require.NotNil(t, route.Doc)
	assert.Equal(t, "Get item", route.Doc.Summary)
	assert.Equal(t, "Returns a single item by ID", route.Doc.Description)
	assert.Equal(t, []string{"Items"}, route.Doc.Tags)
}

func TestRouteDoc_NilDocByDefault(t *testing.T) {
	route := NewRoute("GET", nil) // no doc options
	assert.Nil(t, route.Doc)
}
```

**Step 2: Add RegisterMiddlewareFunc with MiddlewareOption tests**

```go
func TestRegisterMiddlewareFunc_WithSecurity(t *testing.T) {
	app := NewApp(WithTimezone("UTC"))
	mw := func(next http.Handler) http.Handler { return next }

	app.RegisterMiddlewareFunc("auth", mw,
		WithSecurity("bearer", "Authorization", "header"),
	)

	assert.Contains(t, app.middlewareDocs, "auth")
	assert.Equal(t, "bearer", app.middlewareDocs["auth"].SecurityScheme.Type)
	assert.Equal(t, "Authorization", app.middlewareDocs["auth"].SecurityScheme.Name)
}

func TestRegisterMiddlewareFunc_WithoutOpts(t *testing.T) {
	app := NewApp(WithTimezone("UTC"))
	mw := func(next http.Handler) http.Handler { return next }

	app.RegisterMiddlewareFunc("audit", mw) // no opts — backward compatible

	assert.Empty(t, app.middlewareDocs) // no entry created
}

func TestRegisterMiddlewareFunc_WithHeaders(t *testing.T) {
	app := NewApp(WithTimezone("UTC"))
	mw := func(next http.Handler) http.Handler { return next }

	app.RegisterMiddlewareFunc("tenant", mw,
		WithRequiredHeader("X-Tenant-ID", "Tenant ID", true),
	)

	require.Contains(t, app.middlewareDocs, "tenant")
	require.Len(t, app.middlewareDocs["tenant"].Headers, 1)
	assert.Equal(t, "X-Tenant-ID", app.middlewareDocs["tenant"].Headers[0].Name)
	assert.True(t, app.middlewareDocs["tenant"].Headers[0].Required)
}
```

**Step 3: Add ControllerImpl tracking test in app_extended_test.go**

```go
func TestControllerImpl_UseMiddleware_TracksKeys(t *testing.T) {
	impl := NewControllerImpl()
	mw := func(next http.Handler) http.Handler { return next }

	impl.UseMiddleware("auth")
	impl.UseMiddleware("tenant")
	impl.UseMiddleware("auth") // duplicate

	keys := impl.GetUsedMiddlewareKeys()
	assert.Equal(t, []string{"auth", "tenant", "auth"}, keys)
}

func TestControllerImpl_UseMiddleware_WithoutRegistration(t *testing.T) {
	impl := NewControllerImpl()

	handler := impl.UseMiddleware("nonexistent")
	assert.Nil(t, handler)
}
```

**Step 4: Run all tests**

Run: `cd go && go test ./api/ -v -count=1`
Expected: All tests PASS (including existing + new)

**Step 5: Commit**

```bash
git add go/api/app_test.go go/api/app_extended_test.go
git commit -m "test(api): add OpenAPI docs tests for RouteDoc and middleware metadata"
```

---
### Task 6: Final verification

**Step 1: Run full test suite**

Run: `cd go && go test ./... -count=1`
Expected: All packages PASS

**Step 2: Build**

Run: `cd go && go build ./...`
Expected: SUCCESS

**Step 3: Run lint**

Run: `cd go && golangci-lint run --timeout 3m --tests=false ./go/...`
Expected: No new issues
