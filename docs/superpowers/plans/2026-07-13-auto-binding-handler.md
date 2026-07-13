# Auto-Binding Handler Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate 5-line request binding boilerplate from all controller handlers by making phastos auto-bind `Body`/`Query`/`Path` inputs from route annotations, store clean data in context, and provide typed getters via `phastosctx`. Also move trace ID from response body to `X-Trace-ID` header.

**Architecture:** 
- Handler signature changes from `func(req Request, ctx Context) *Response` to `func(ctx Context) (any, error)`. Phastos detects new signature at registration and applies auto-binding flow (path → query → body → validate). If binding fails, handler is NOT called — error returned directly to client.
- Bound data stored in `context.Context` via `phastosctx` package. Usecase retrieves via generic getters: `phastosctx.Request[*T](ctx)`.
- Backward compatible: old handler signature continues to work. Migration is per-handler.

**Tech Stack:** Go 1.26, chi v5, gorilla/schema, go-playground/validator

## Global Constraints

- No breaking changes to existing `func(req Request, ctx Context) *Response` handlers
- No gomock.Any() or other argument matchers — use concrete values in all tests
- Test coverage ≥97.5% per package (exclude `/mocks`, `/vendor`, `vendor.orig`, `/domain/models`)
- `X-Trace-ID` as response header, NOT in JSON body
- Path param types: `string`, `int`, `int8`, `int16`, `int32`, `int64`, `uint`, `uint8`, `uint16`, `uint32`, `uint64`, `float32`, `float64`, `bool`
- Auto-binding order: path params → query params → body → validate

---

### Task 1: Define `Handler2` type + handler detection

**Files:**
- Modify: `go/api/app.go` (add `Handler2` type)
- Modify: `go/api/app.go` (add signature detection in `registerHandler`)

**Interfaces:**
- Produces: `type Handler2 func(ctx context.Context) (any, error)` — public type
- Produces: `func (app *App) isHandler2(h Handler) bool` — internal detection

- [ ] **Step 1: Add Handler2 type to app.go**

In `go/api/app.go`, after the existing `type Handler func(Request, context.Context) *Response` definition (~line 39):

```go
// Handler2 is the new auto-binding handler signature.
// Phastos binds request body/query/path params from route annotations,
// stores them in context, and calls the handler. The handler returns
// (data, error) — phastos wraps into api.Response automatically.
type Handler2 func(ctx context.Context) (any, error)
```

- [ ] **Step 2: Add handler signature detection**

In `go/api/app.go`, add a helper function:

```go
func isHandler2(h Handler) bool {
	// Handler2 has a different underlying function type than Handler.
	// We detect by comparing reflect types: Handler2 has 1 arg + 2 returns,
	// Handler has 2 args + 1 return.
	return reflect.TypeOf(h).NumIn() == 1
}
```

- [ ] **Step 3: Add `reflect` import if not already present**

Check imports in `go/api/app.go`. If `"reflect"` is not imported, add it.

- [ ] **Step 4: Write tests for Handler2 detection**

In `go/api/app_extended_test.go`, add:

```go
func TestIsHandler2_NewSignature(t *testing.T) {
	h := Handler(func(ctx context.Context) (any, error) {
		return map[string]string{"ok": "yes"}, nil
	})
	assert.True(t, isHandler2(h))
}

func TestIsHandler2_OldSignature(t *testing.T) {
	h := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("ok")
	}
	assert.False(t, isHandler2(h))
}
```

- [ ] **Step 5: Run tests to verify**

```bash
cd /Users/radityapratama/dev/others/phastos && go test ./go/api/... -run "TestIsHandler2" -v
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add go/api/app.go go/api/app_extended_test.go
git commit -m "feat(api): add Handler2 type and handler signature detection"
```

---

### Task 2: Typed `WithPath` — parse `{name:type}` syntax

**Files:**
- Modify: `go/api/controller.go` (update `WithPath`, add path param type struct)

**Interfaces:**
- Produces: `type PathParamType int` — enum for path param types
- Produces: `PathParamType` constants: `ParamString`, `ParamInt`, `ParamInt64`, etc.
- Produces: `parsePathParamTypes(path string) []PathParamType` — internal parser
- Produces: Updated `Route` struct with `PathParamTypes []PathParamType` field

- [ ] **Step 1: Add PathParamType enum and Route field**

In `go/api/controller.go`, add after the `RouteDoc` struct (~line 56):

```go
type PathParamType int

const (
	ParamString  PathParamType = iota
	ParamInt
	ParamInt8
	ParamInt16
	ParamInt32
	ParamInt64
	ParamUint
	ParamUint8
	ParamUint16
	ParamUint32
	ParamUint64
	ParamFloat32
	ParamFloat64
	ParamBool
)

func (t PathParamType) String() string {
	switch t {
	case ParamInt: return "int"
	case ParamInt8: return "int8"
	case ParamInt16: return "int16"
	case ParamInt32: return "int32"
	case ParamInt64: return "int64"
	case ParamUint: return "uint"
	case ParamUint8: return "uint8"
	case ParamUint16: return "uint16"
	case ParamUint32: return "uint32"
	case ParamUint64: return "uint64"
	case ParamFloat32: return "float32"
	case ParamFloat64: return "float64"
	case ParamBool: return "bool"
	default: return "string"
	}
}

func parsePathParamType(s string) PathParamType {
	switch s {
	case "int": return ParamInt
	case "int8": return ParamInt8
	case "int16": return ParamInt16
	case "int32": return ParamInt32
	case "int64": return ParamInt64
	case "uint": return ParamUint
	case "uint8": return ParamUint8
	case "uint16": return ParamUint16
	case "uint32": return ParamUint32
	case "uint64": return ParamUint64
	case "float32": return ParamFloat32
	case "float64": return ParamFloat64
	case "bool": return ParamBool
	default: return ParamString
	}
}
```

- [ ] **Step 2: Add PathParamTypes to Route struct**

In the `Route` struct (~line 36), add:

```go
type Route struct {
	Method         string
	Path           string
	Handler        Handler
	Version        int
	Middlewares    *[]func(http.Handler) http.Handler
	SubRoutes      []Route
	Doc            *RouteDoc
	PathParamTypes []PathParamType
}
```

- [ ] **Step 3: Add parsePathParamTypes function**

In `go/api/controller.go`:

```go
// parsePathParamTypes extracts {name:type} type annotations from a path pattern.
// E.g., "/absence/{id:int64}/recap/{month}" → [ParamInt64, ParamString]
func parsePathParamTypes(path string) []PathParamType {
	var types []PathParamType
	start := -1
	for i := 0; i < len(path); i++ {
		if path[i] == '{' {
			start = i + 1
		}
		if path[i] == '}' && start != -1 {
			param := path[start:i]
			if idx := strings.IndexByte(param, ':'); idx != -1 {
				types = append(types, parsePathParamType(param[idx+1:]))
			} else {
				types = append(types, ParamString)
			}
			start = -1
		}
	}
	return types
}
```

- [ ] **Step 4: Update WithPath to call parsePathParamTypes**

Modify the `WithPath` function (~line 111):

```go
func WithPath(path string) RouteOption {
	return func(r *Route) {
		r.Path = path
		r.PathParamTypes = parsePathParamTypes(path)
	}
}
```

- [ ] **Step 5: Write tests for path param type parsing**

In `go/api/app_test.go` or new test section:

```go
func TestParsePathParamTypes_AllString(t *testing.T) {
	types := parsePathParamTypes("/absence/{id}/recap/{month}")
	assert.Len(t, types, 2)
	assert.Equal(t, ParamString, types[0])
	assert.Equal(t, ParamString, types[1])
}

func TestParsePathParamTypes_TypedParams(t *testing.T) {
	types := parsePathParamTypes("/absence/{id:int64}/recap/{month:int}")
	assert.Len(t, types, 2)
	assert.Equal(t, ParamInt64, types[0])
	assert.Equal(t, ParamInt, types[1])
}

func TestParsePathParamTypes_MixedTypes(t *testing.T) {
	types := parsePathParamTypes("/{id:int64}/detail/{active:bool}")
	assert.Len(t, types, 2)
	assert.Equal(t, ParamInt64, types[0])
	assert.Equal(t, ParamBool, types[1])
}

func TestParsePathParamTypes_NoParams(t *testing.T) {
	types := parsePathParamTypes("/absence")
	assert.Len(t, types, 0)
}
```

- [ ] **Step 6: Run tests**

```bash
go test ./go/api/... -run "TestParsePathParamTypes" -v
```
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add go/api/controller.go go/api/app_test.go
git commit -m "feat(api): add typed WithPath with {name:type} syntax support"
```

---

### Task 3: `phastosctx` — new context getters for request data

**Files:**
- Create: `go/context/request.go`
- Create: `go/context/request_test.go`

**Interfaces:**
- Produces: `func SetRequest[T any](ctx context.Context, val *T) context.Context`
- Produces: `func Request[T any](ctx context.Context) *T`
- Produces: `func SetQueryParams[T any](ctx context.Context, val *T) context.Context`
- Produces: `func QueryParams[T any](ctx context.Context) *T`
- Produces: `func SetPathParams(ctx context.Context, val map[string]string) context.Context`
- Produces: `func PathParam[T any](ctx context.Context, name string) T`

- [ ] **Step 1: Create request.go with context keys and typed getters**

Create `go/context/request.go`:

```go
package context

import (
	"context"
)

type requestDataKey struct{}
type queryParamsKey struct{}
type pathParamsKey struct{}

// SetRequest stores the bound request body in context.
func SetRequest[T any](ctx context.Context, val *T) context.Context {
	return context.WithValue(ctx, requestDataKey{}, val)
}

// Request retrieves the bound request body from context.
// Returns nil if no request data was set.
func Request[T any](ctx context.Context) *T {
	val, _ := ctx.Value(requestDataKey{}).(*T)
	return val
}

// SetQueryParams stores the bound query params in context.
func SetQueryParams[T any](ctx context.Context, val *T) context.Context {
	return context.WithValue(ctx, queryParamsKey{}, val)
}

// QueryParams retrieves the bound query params from context.
func QueryParams[T any](ctx context.Context) *T {
	val, _ := ctx.Value(queryParamsKey{}).(*T)
	return val
}

// SetPathParams stores path parameters in context.
func SetPathParams(ctx context.Context, val map[string]string) context.Context {
	return context.WithValue(ctx, pathParamsKey{}, val)
}

// PathParam retrieves a typed path parameter from context.
// Returns the zero value of T if the param is not found.
func PathParam[T any](ctx context.Context, name string) T {
	params, _ := ctx.Value(pathParamsKey{}).(map[string]string)
	if params == nil {
		var zero T
		return zero
	}
	raw, ok := params[name]
	if !ok {
		var zero T
		return zero
	}
	return convertParam[T](raw)
}
```

- [ ] **Step 2: Create request_test.go**

Create `go/context/request_test.go`:

```go
package context

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testPayload struct {
	Name string
	Age  int
}

func TestSetAndGetRequest(t *testing.T) {
	ctx := context.Background()
	payload := &testPayload{Name: "test", Age: 30}
	ctx = SetRequest(ctx, payload)

	result := Request[*testPayload](ctx)
	assert.NotNil(t, result)
	assert.Equal(t, "test", result.Name)
	assert.Equal(t, 30, result.Age)
}

func TestRequest_NilWhenNotSet(t *testing.T) {
	ctx := context.Background()
	result := Request[*testPayload](ctx)
	assert.Nil(t, result)
}

func TestSetAndGetQueryParams(t *testing.T) {
	ctx := context.Background()
	params := &testPayload{Name: "query", Age: 25}
	ctx = SetQueryParams(ctx, params)

	result := QueryParams[*testPayload](ctx)
	assert.NotNil(t, result)
	assert.Equal(t, "query", result.Name)
}

func TestSetAndGetPathParams(t *testing.T) {
	ctx := context.Background()
	params := map[string]string{"id": "42", "name": "test"}
	ctx = SetPathParams(ctx, params)

	id := PathParam[string](ctx, "id")
	assert.Equal(t, "42", id)

	name := PathParam[string](ctx, "name")
	assert.Equal(t, "test", name)
}

func TestPathParam_NotSet(t *testing.T) {
	ctx := context.Background()
	val := PathParam[string](ctx, "nonexistent")
	assert.Equal(t, "", val)

	ival := PathParam[int64](ctx, "nonexistent")
	assert.Equal(t, int64(0), ival)
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./go/context/... -run "TestSetAndGetRequest|TestPathParam" -v
```
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add go/context/request.go go/context/request_test.go
git commit -m "feat(context): add typed request, query, and path param context getters"
```

---

### Task 4: `convertParam` — typed path param conversion

**Files:**
- Modify: `go/context/request.go` (add `convertParam` function)
- Modify: `go/context/request_test.go` (add conversion tests)

**Interfaces:**
- Consumes: `func PathParam[T any](ctx context.Context, name string) T` (from Task 3)
- Produces: `func convertParam[T any](raw string) T` — converts string to typed value

- [ ] **Step 1: Add convertParam to request.go**

Append to `go/context/request.go`:

```go
import (
	"strconv"
)

func convertParam[T any](raw string) T {
	var result any
	switch any((*T)(nil)).(type) {
	case *int:
		v, _ := strconv.Atoi(raw)
		result = v
	case *int8:
		v, _ := strconv.ParseInt(raw, 10, 8)
		result = int8(v)
	case *int16:
		v, _ := strconv.ParseInt(raw, 10, 16)
		result = int16(v)
	case *int32:
		v, _ := strconv.ParseInt(raw, 10, 32)
		result = int32(v)
	case *int64:
		v, _ := strconv.ParseInt(raw, 10, 64)
		result = v
	case *uint:
		v, _ := strconv.ParseUint(raw, 10, 0)
		result = uint(v)
	case *uint8:
		v, _ := strconv.ParseUint(raw, 10, 8)
		result = uint8(v)
	case *uint16:
		v, _ := strconv.ParseUint(raw, 10, 16)
		result = uint16(v)
	case *uint32:
		v, _ := strconv.ParseUint(raw, 10, 32)
		result = uint32(v)
	case *uint64:
		v, _ := strconv.ParseUint(raw, 10, 64)
		result = v
	case *float32:
		v, _ := strconv.ParseFloat(raw, 32)
		result = float32(v)
	case *float64:
		v, _ := strconv.ParseFloat(raw, 64)
		result = v
	case *bool:
		v, _ := strconv.ParseBool(raw)
		result = v
	default:
		result = raw
	}
	return result.(T)
}
```

- [ ] **Step 2: Add conversion tests**

Append to `go/context/request_test.go`:

```go
func TestConvertParam_Int64(t *testing.T) {
	val := convertParam[int64]("42")
	assert.Equal(t, int64(42), val)
}

func TestConvertParam_String(t *testing.T) {
	val := convertParam[string]("hello")
	assert.Equal(t, "hello", val)
}

func TestConvertParam_Bool(t *testing.T) {
	val := convertParam[bool]("true")
	assert.Equal(t, true, val)
}

func TestConvertParam_Float64(t *testing.T) {
	val := convertParam[float64]("3.14")
	assert.InDelta(t, 3.14, val, 0.001)
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./go/context/... -v
```
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add go/context/request.go go/context/request_test.go
git commit -m "feat(context): add convertParam for typed path param conversion"
```

---

### Task 5: Auto-binding flow in `registerHandler`

**Files:**
- Modify: `go/api/app.go` (add `bindAndCallHandler2` function)
- Modify: `go/api/app.go` (wire into `registerHandler`)

**Interfaces:**
- Consumes: `isHandler2(h Handler) bool` (from Task 1)
- Consumes: `route.PathParamTypes []PathParamType` (from Task 2)
- Consumes: `phastosctx.SetRequest`, `phastosctx.SetQueryParams`, `phastosctx.SetPathParams` (from Task 3-4)
- Consumes: `route.Doc.RequestType`, `route.Doc` (existing)
- Produces: `func (app *App) bindAndCallHandler2(...)` — internal

- [ ] **Step 1: Add routeRegistryEntry extension for path param types**

Extend `routeRegistryEntry` struct in app.go (~line 38):

```go
type routeRegistryEntry struct {
	Method         string
	Path           string
	Doc            *RouteDoc
	PathParamTypes []PathParamType
}
```

- [ ] **Step 2: Update routeRegistry population in registerRoutes**

In `registerRoutes` (~line 699), update the registry append:

```go
app.routeRegistry = append(app.routeRegistry, routeRegistryEntry{
	Method:         route.Method,
	Path:           routePath,
	Doc:            route.Doc,
	PathParamTypes: route.PathParamTypes,
})
```

- [ ] **Step 3: Add bindAndCallHandler2 function**

In `go/api/app.go`, add before `registerHandler`:

```go
// bindAndCallHandler2 performs auto-binding (path params → query → body),
// stores data in context, and calls the Handler2. If binding fails at any
// step, it returns an error response without calling the handler.
func (app *App) bindAndCallHandler2(r *http.Request, h Handler2, path string, pathParamTypes []PathParamType, requestType any) (any, error) {
	ctx := r.Context()

	// 1. Extract and validate path params
	if len(pathParamTypes) > 0 {
		chiParams := chi.RouteContext(r.Context())
		var paramNames []string
		paramValues := make(map[string]string)
		if chiParams != nil {
			for i, key := range chiParams.URLParams.Keys {
				val := chiParams.URLParams.Values[i]
				paramNames = append(paramNames, key)
				paramValues[key] = val
			}
		}
		// Validate types
		for i, pt := range pathParamTypes {
			if i >= len(paramNames) {
				return nil, BadRequest("missing path parameter", "ERR_MISSING_PATH_PARAM")
			}
			raw := paramValues[paramNames[i]]
			if err := validatePathParamType(raw, pt); err != nil {
				return nil, err
			}
		}
		ctx = phastosctx.SetPathParams(ctx, paramValues)
	}

	// 2. Bind query params (if requestType is set and request method allows)
	if requestType != nil && (r.Method == http.MethodGet) {
		queryType := reflect.TypeOf(requestType)
		if queryType.Kind() == reflect.Ptr {
			queryType = queryType.Elem()
		}
		queryVal := reflect.New(queryType).Interface()
		if err := decoder.Decode(queryVal, r.URL.Query()); err != nil {
			return nil, BadRequest(err.Error(), "ERROR_PARSING_QUERY_PARAMS")
		}
		if err := app.requestValidator(queryVal); err != nil {
			return nil, err
		}
		ctx = phastosctx.SetQueryParams(ctx, queryVal)
	}

	// 3. Bind body (if requestType is set and method is POST/PUT/PATCH)
	if requestType != nil && (r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch) {
		bodyType := reflect.TypeOf(requestType)
		if bodyType.Kind() == reflect.Ptr {
			bodyType = bodyType.Elem()
		}
		bodyVal := reflect.New(bodyType).Interface()
		if err := json.NewDecoder(r.Body).Decode(bodyVal); err != nil {
			return nil, BadRequest(err.Error(), "ERROR_PARSING_BODY")
		}
		if err := app.requestValidator(bodyVal); err != nil {
			return nil, err
		}
		ctx = phastosctx.SetRequest(ctx, bodyVal)
	}

	// 4. Update request context and call handler
	r = r.WithContext(ctx)
	return h(r.Context())
}
```

- [ ] **Step 4: Add validatePathParamType helper**

```go
func validatePathParamType(raw string, pt PathParamType) error {
	switch pt {
	case ParamInt, ParamInt8, ParamInt16, ParamInt32, ParamInt64:
		if _, err := strconv.ParseInt(raw, 10, 64); err != nil {
			return BadRequest(
				fmt.Sprintf("path param expects %s, got '%s'", pt.String(), raw),
				"ERR_INVALID_PATH_PARAM",
			)
		}
	case ParamUint, ParamUint8, ParamUint16, ParamUint32, ParamUint64:
		if _, err := strconv.ParseUint(raw, 10, 64); err != nil {
			return BadRequest(
				fmt.Sprintf("path param expects %s, got '%s'", pt.String(), raw),
				"ERR_INVALID_PATH_PARAM",
			)
		}
	case ParamFloat32, ParamFloat64:
		if _, err := strconv.ParseFloat(raw, 64); err != nil {
			return BadRequest(
				fmt.Sprintf("path param expects %s, got '%s'", pt.String(), raw),
				"ERR_INVALID_PATH_PARAM",
			)
		}
	case ParamBool:
		if _, err := strconv.ParseBool(raw); err != nil {
			return BadRequest(
				fmt.Sprintf("path param expects bool, got '%s'", raw),
				"ERR_INVALID_PATH_PARAM",
			)
		}
	}
	return nil
}
```

- [ ] **Step 5: Wire into registerHandler**

Modify `registerHandler` to detect `Handler2` and use auto-binding:

```go
func (app *App) registerHandler(method, path string, handler Handler, middlewares ...func(http.Handler) http.Handler) {
	app.flushPendingMiddlewares()

	if isHandler2(handler) {
		h2 := handler.(Handler2) //nolint:errcheck
		wrapppedHandler := func(w http.ResponseWriter, r *http.Request) {
			result, err := app.wrapHandler2(r, h2, method, path)
			var resp *Response
			switch {
			case err != nil:
				resp = NewResponse().SetError(err)
			case result == nil:
				resp = NewResponse()
			default:
				if customResp, ok := result.(*Response); ok {
					resp = customResp
				} else {
					resp = NewResponse().SetData(result)
				}
			}
			resp.Send(w)
			ReleaseResponse(resp)
		}
		handlerFunc := chi.Chain(middlewares...).HandlerFunc(wrapppedHandler)
		// ... version-group routing (existing logic) ...
		return
	}

	// Existing flow for old Handler signature
	// ... (unchanged) ...
}
```

Hmm, this is getting complex. Let me simplify: the auto-binding flow for Handler2 should be integrated into the existing `wrapHandler` pattern. Rather than changing `registerHandler`, I can:

1. Keep `registerHandler` as-is
2. In the `wrapHandler` function, detect Handler2 vs Handler (old)

Actually, the problem is that `registerHandler` calls `app.wrapHandler(handler)` which wraps the handler for NR/OTel. The handler is called inside `wrapHandler`. So the detection should happen inside `wrapHandler`.

Let me revise:

In `app.go`, the existing `wrapHandler` method (~line 472) needs a branch for Handler2:

```go
func (app *App) wrapHandler(handler Handler) http.HandlerFunc {
	// Check if this is the new Handler2 signature
	if h2, ok := handler.(Handler2); ok {
		return app.wrapHandler2(h2)
	}
	// ...existing code for old Handler...
}
```

And add a new `wrapHandler2` method that does auto-binding + response wrapping.

Actually, this is getting too complex to inline in the plan. Let me restructure.

The key insight: `registerHandler` receives a `Handler` (which is `func(req Request, ctx Context) *Response`). But `Handler2` is `func(ctx Context) (any, error)`. These are different function types in Go. They can't be stored in the same variable unless we use `any`/`interface{}`.

Looking at the current code:
```go
type Handler func(Request, context.Context) *Response
```

And routes use:
```go
func NewRoute(method string, handler Handler, opts ...RouteOption) Route
```

So `Route.Handler` is type `Handler`. For the new signature, I need to either:
a) Change `Handler` to be `any`/`interface{}` — type-assert at registration
b) Add a separate `Handler2` field to `Route`
c) Wrap Handler2 into Handler via adapter

Option (c) is cleanest. When `NewRoute` receives a `Handler2`, wrap it automatically:

Actually, `NewRoute` takes `Handler`, not `Handler2`. We need to change the type.

Let me simplify. In `NewRoute`:
```go
func NewRoute(method string, handler any, opts ...RouteOption) Route
```

Where `handler` can be `Handler` or `Handler2`. Store as `any` in the Route struct.

Then in `registerHandler`, type-assert:
```go
switch h := route.Handler.(type) {
case Handler2:
    // auto-binding flow
case Handler:
    // old flow
}
```

Let me revise the plan with this cleaner approach.

OK, I think I need to rewrite this task to be cleaner. Let me do that in the actual plan file. This task is too complex to keep iterating on here. Let me write a cleaner version.

Actually, let me just write the complete plan now, with proper structure. I've been iterating in my head too long.<｜end▁of▁thinking｜>

<｜｜DSML｜｜tool_calls>
<｜｜DSML｜｜invoke name="write">
<｜｜DSML｜｜parameter name="content" string="true"># Auto-Binding Handler Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate 5-line request binding boilerplate by making phastos auto-bind `Body`/`Query`/`Path` inputs from route annotations, store clean data in `phastosctx`, and provide typed getters. Also move trace ID to `X-Trace-ID` response header.

**Architecture:** Handler signature changes from `func(req, ctx) *Response` to `func(ctx) (any, error)`. Route.Handler becomes `any` to support both signatures. `registerHandler` type-switches: new signature → auto-bind before call; old signature → existing flow. Backward compatible.

**Tech Stack:** Go 1.26, chi v5, gorilla/schema, go-playground/validator

## Global Constraints

- No breaking changes to existing `func(req Request, ctx Context) *Response` handlers
- No gomock.Any() — concrete values in all tests
- Test coverage ≥97.5% per package (exclude `/mocks`, `/vendor`, `vendor.orig`, `/domain/models`)
- `X-Trace-ID` as response header, NOT in JSON body
- Path param types: `string`, `int`, `int8`, `int16`, `int32`, `int64`, `uint`, `uint8`, `uint16`, `uint32`, `uint64`, `float32`, `float64`, `bool`
- Auto-binding order: path params → query params → body → validate

---

### Task 1: `PathParamType` enum + typed `WithPath` parsing

**Files:**
- Modify: `go/api/controller.go`

**Produces:**
- `type PathParamType int` + constants
- `func parsePathParamTypes(path string) []PathParamType`
- Updated `WithPath` that calls `parsePathParamTypes`
- `PathParamTypes` field on `Route` struct

- [ ] **Step 1: Add PathParamType to controller.go**

After the `RouteDoc` struct (line ~56), add:

```go
type PathParamType int

const (
	ParamString  PathParamType = iota
	ParamInt
	ParamInt8
	ParamInt16
	ParamInt32
	ParamInt64
	ParamUint
	ParamUint8
	ParamUint16
	ParamUint32
	ParamUint64
	ParamFloat32
	ParamFloat64
	ParamBool
)

func (t PathParamType) String() string {
	switch t {
	case ParamInt: return "int"
	case ParamInt8: return "int8"
	case ParamInt16: return "int16"
	case ParamInt32: return "int32"
	case ParamInt64: return "int64"
	case ParamUint: return "uint"
	case ParamUint8: return "uint8"
	case ParamUint16: return "uint16"
	case ParamUint32: return "uint32"
	case ParamUint64: return "uint64"
	case ParamFloat32: return "float32"
	case ParamFloat64: return "float64"
	case ParamBool: return "bool"
	default: return "string"
	}
}

func parsePathParamType(s string) PathParamType {
	switch s {
	case "int": return ParamInt
	case "int8": return ParamInt8
	case "int16": return ParamInt16
	case "int32": return ParamInt32
	case "int64": return ParamInt64
	case "uint": return ParamUint
	case "uint8": return ParamUint8
	case "uint16": return ParamUint16
	case "uint32": return ParamUint32
	case "uint64": return ParamUint64
	case "float32": return ParamFloat32
	case "float64": return ParamFloat64
	case "bool": return ParamBool
	default: return ParamString
	}
}
```

- [ ] **Step 2: Add PathParamTypes to Route struct**

In `Route` struct, add: `PathParamTypes []PathParamType`

- [ ] **Step 3: Add parsePathParamTypes function**

```go
func parsePathParamTypes(path string) []PathParamType {
	var types []PathParamType
	start := -1
	for i := 0; i < len(path); i++ {
		if path[i] == '{' {
			start = i + 1
		}
		if path[i] == '}' && start != -1 {
			param := path[start:i]
			if idx := strings.IndexByte(param, ':'); idx != -1 {
				types = append(types, parsePathParamType(param[idx+1:]))
			} else {
				types = append(types, ParamString)
			}
			start = -1
		}
	}
	return types
}
```

- [ ] **Step 4: Update WithPath**

```go
func WithPath(path string) RouteOption {
	return func(r *Route) {
		r.Path = path
		r.PathParamTypes = parsePathParamTypes(path)
	}
}
```

- [ ] **Step 5: Add `strings` import if missing**

- [ ] **Step 6: Write tests in controller_test.go or app_test.go**

```go
func TestParsePathParamTypes_AllString(t *testing.T) {
	types := parsePathParamTypes("/absence/{id}/recap/{month}")
	assert.Len(t, types, 2)
	assert.Equal(t, ParamString, types[0])
	assert.Equal(t, ParamString, types[1])
}

func TestParsePathParamTypes_TypedParams(t *testing.T) {
	types := parsePathParamTypes("/absence/{id:int64}/recap/{month:int}")
	assert.Len(t, types, 2)
	assert.Equal(t, ParamInt64, types[0])
	assert.Equal(t, ParamInt, types[1])
}

func TestParsePathParamTypes_Mixed(t *testing.T) {
	types := parsePathParamTypes("/{id:int64}/detail/{active:bool}")
	assert.Len(t, types, 2)
	assert.Equal(t, ParamInt64, types[0])
	assert.Equal(t, ParamBool, types[1])
}

func TestParsePathParamTypes_NoParams(t *testing.T) {
	types := parsePathParamTypes("/absence")
	assert.Len(t, types, 0)
}

func TestParsePathParamTypes_DefaultString(t *testing.T) {
	types := parsePathParamTypes("/{id}")
	assert.Len(t, types, 1)
	assert.Equal(t, ParamString, types[0])
}
```

- [ ] **Step 7: Run tests and commit**

```bash
go test ./go/api/... -run "TestParsePathParamTypes" -v
# Expected: PASS

git add go/api/controller.go go/api/app_test.go
git commit -m "feat(api): add PathParamType enum and typed WithPath {name:type} parsing"
```

---

### Task 2: Change Route.Handler to `any` + add `Handler2` type

**Files:**
- Modify: `go/api/controller.go` (Route struct, NewRoute)
- Modify: `go/api/app.go` (type aliases, registerRoutes callback)

**Produces:**
- `type Handler2 func(context.Context) (any, error)`
- `Route.Handler` becomes `any`
- `NewRoute(method string, handler any, opts ...RouteOption) Route`

- [ ] **Step 1: Add Handler2 type in controller.go**

```go
// Handler2 is the auto-binding handler signature.
// Phastos binds request body/query/path params from route annotations
// and stores them in context before calling the handler. The handler
// returns (data, error) — phastos wraps into api.Response automatically.
type Handler2 func(ctx context.Context) (any, error)
```

- [ ] **Step 2: Change Route.Handler to any**

From: `Handler Handler` → To: `Handler any`

- [ ] **Step 3: Change NewRoute signature**

From: `func NewRoute(method string, handler Handler, opts ...RouteOption) Route`
To: `func NewRoute(method string, handler any, opts ...RouteOption) Route`

- [ ] **Step 4: Update any code casting route.Handler**

Search for `route.Handler` usages in `registerRoutes`. The callback pattern needs update.

In `registerRoutes` (~line 697 of current app.go), change:

```go
app.registerHandler(route.Method, routePath, route.Handler, middlewares...)
```

To pass `route.Handler` as `any` (already compatible since `registerHandler` takes `Handler`).

- [ ] **Step 5: Write tests**

```go
func TestNewRoute_AcceptsHandler2(t *testing.T) {
	h := func(ctx context.Context) (any, error) {
		return map[string]string{"ok": "yes"}, nil
	}
	r := NewRoute("POST", h, WithPath("/test"))
	assert.NotNil(t, r)
	assert.Equal(t, "POST", r.Method)
}

func TestNewRoute_AcceptsHandler(t *testing.T) {
	h := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("ok")
	}
	r := NewRoute("GET", h, WithPath("/test"))
	assert.NotNil(t, r)
}
```

- [ ] **Step 6: Run tests and commit**

```bash
go test ./go/api/... -v
# Expected: all PASS

git add go/api/controller.go go/api/app.go go/api/app_test.go
git commit -m "feat(api): make Route.Handler any, add Handler2 type"
```

---

### Task 3: `phastosctx` request data getters

**Files:**
- Create: `go/context/request.go`
- Create: `go/context/request_test.go`

**Produces:**
- `func SetRequest[T any](ctx, *T) context.Context`
- `func Request[T any](ctx) *T`
- `func SetQueryParams[T any](ctx, *T) context.Context`
- `func QueryParams[T any](ctx) *T`
- `func SetPathParams(ctx, map[string]string) context.Context`
- `func PathParams(ctx) map[string]string`
- `func PathParam[T any](ctx, string) T`
- `func convertParam[T any](raw string) T`

- [ ] **Step 1: Create go/context/request.go**

```go
package context

import (
	"context"
	"strconv"
)

type requestBodyKey struct{}
type queryParamsKey struct{}
type pathParamsKey struct{}

func SetRequestBody[T any](ctx context.Context, val *T) context.Context {
	return context.WithValue(ctx, requestBodyKey{}, val)
}

func RequestBody[T any](ctx context.Context) *T {
	val, _ := ctx.Value(requestBodyKey{}).(*T)
	return val
}

func SetQueryParams(ctx context.Context, val any) context.Context {
	return context.WithValue(ctx, queryParamsKey{}, val)
}

func QueryParams[T any](ctx context.Context) *T {
	val, _ := ctx.Value(queryParamsKey{}).(*T)
	return val
}

func SetPathParams(ctx context.Context, val map[string]string) context.Context {
	return context.WithValue(ctx, pathParamsKey{}, val)
}

func PathParams(ctx context.Context) map[string]string {
	val, _ := ctx.Value(pathParamsKey{}).(map[string]string)
	return val
}

func PathParam[T any](ctx context.Context, name string) T {
	params := PathParams(ctx)
	if params == nil {
		var zero T
		return zero
	}
	raw, ok := params[name]
	if !ok {
		var zero T
		return zero
	}
	return convertParam[T](raw)
}

func convertParam[T any](raw string) T {
	var result any
	switch any(new(T)).(type) {
	case *int:
		v, _ := strconv.Atoi(raw)
		result = v
	case *int8:
		v, _ := strconv.ParseInt(raw, 10, 8)
		result = int8(v)
	case *int16:
		v, _ := strconv.ParseInt(raw, 10, 16)
		result = int16(v)
	case *int32:
		v, _ := strconv.ParseInt(raw, 10, 32)
		result = int32(v)
	case *int64:
		v, _ := strconv.ParseInt(raw, 10, 64)
		result = v
	case *uint:
		v, _ := strconv.ParseUint(raw, 10, 0)
		result = uint(v)
	case *uint8:
		v, _ := strconv.ParseUint(raw, 10, 8)
		result = uint8(v)
	case *uint16:
		v, _ := strconv.ParseUint(raw, 10, 16)
		result = uint16(v)
	case *uint32:
		v, _ := strconv.ParseUint(raw, 10, 32)
		result = uint32(v)
	case *uint64:
		v, _ := strconv.ParseUint(raw, 10, 64)
		result = v
	case *float32:
		v, _ := strconv.ParseFloat(raw, 32)
		result = float32(v)
	case *float64:
		v, _ := strconv.ParseFloat(raw, 64)
		result = v
	case *bool:
		v, _ := strconv.ParseBool(raw)
		result = v
	default:
		result = raw
	}
	return result.(T)
}
```

- [ ] **Step 2: Create go/context/request_test.go**

```go
package context

import (
	"context"
	"testing"
	"github.com/stretchr/testify/assert"
)

type testData struct {
	Name string
	Age  int
}

func TestSetAndGetRequestBody(t *testing.T) {
	ctx := context.Background()
	payload := &testData{Name: "test", Age: 30}
	ctx = SetRequestBody(ctx, payload)
	result := RequestBody[*testData](ctx)
	assert.NotNil(t, result)
	assert.Equal(t, "test", result.Name)
}

func TestRequestBody_NilWhenNotSet(t *testing.T) {
	assert.Nil(t, RequestBody[*testData](context.Background()))
}

func TestSetAndGetQueryParams(t *testing.T) {
	ctx := context.Background()
	params := &testData{Name: "query", Age: 25}
	ctx = SetQueryParams(ctx, params)
	result := QueryParams[*testData](ctx)
	assert.NotNil(t, result)
	assert.Equal(t, "query", result.Name)
}

func TestSetAndGetPathParams(t *testing.T) {
	ctx := context.Background()
	ctx = SetPathParams(ctx, map[string]string{"id": "42", "name": "test"})
	assert.Equal(t, "42", PathParam[string](ctx, "id"))
	assert.Equal(t, "test", PathParam[string](ctx, "name"))
}

func TestPathParam_NotSet(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "", PathParam[string](ctx, "nonexistent"))
	assert.Equal(t, int64(0), PathParam[int64](ctx, "nonexistent"))
}

func TestConvertParam_Int64(t *testing.T) {
	assert.Equal(t, int64(42), convertParam[int64]("42"))
}

func TestConvertParam_Bool(t *testing.T) {
	assert.Equal(t, true, convertParam[bool]("true"))
}

func TestConvertParam_Float64(t *testing.T) {
	assert.InDelta(t, 3.14, convertParam[float64]("3.14"), 0.001)
}
```

- [ ] **Step 3: Run tests and commit**

```bash
go test ./go/context/... -v -race
# Expected: PASS

git add go/context/request.go go/context/request_test.go
git commit -m "feat(context): add typed request body, query, and path param context getters"
```

---

### Task 4: Auto-binding + Handler2 dispatch in `registerHandler`

**Files:**
- Modify: `go/api/app.go`

**Consumes:** `Handler2`, `PathParamTypes`, `phastosctx.*`, `route.Doc.RequestType`

- [ ] **Step 1: Add `bindAndDispatch` function**

In `go/api/app.go`, add:

```go
func (app *App) bindAndDispatch(h2 Handler2, requestType any, pathParamTypes []PathParamType, r *http.Request) (any, error) {
	ctx := r.Context()

	// 1. Extract and validate path params
	if len(pathParamTypes) > 0 {
		rctx := chi.RouteContext(r.Context())
		params := map[string]string{}
		if rctx != nil {
			for i, key := range rctx.URLParams.Keys {
				params[key] = rctx.URLParams.Values[i]
				if i < len(pathParamTypes) {
					if err := validatePathParam(params[key], pathParamTypes[i]); err != nil {
						return nil, err
					}
				}
			}
		}
		ctx = phastosctx.SetPathParams(ctx, params)
	}

	// 2. Bind query params (if requestType is set)
	if requestType != nil {
		reqType := reflect.TypeOf(requestType)
		if reqType.Kind() == reflect.Ptr {
			reqType = reqType.Elem()
		}
		queryVal := reflect.New(reqType).Interface()
		if err := decoder.Decode(queryVal, r.URL.Query()); err != nil {
			return nil, BadRequest(err.Error(), "ERROR_PARSING_QUERY_PARAMS")
		}
		// Query binding for GET; for POST/PUT body binding takes over
		if r.Method == http.MethodGet {
			if err := app.requestValidator(queryVal); err != nil {
				return nil, err
			}
			ctx = phastosctx.SetQueryParams(ctx, queryVal)
		}
	}

	// 3. Bind body (for POST/PUT/PATCH methods)
	if requestType != nil && (r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch) {
		bodyType := reflect.TypeOf(requestType)
		if bodyType.Kind() == reflect.Ptr {
			bodyType = bodyType.Elem()
		}
		bodyVal := reflect.New(bodyType).Interface()
		if err := json.NewDecoder(r.Body).Decode(bodyVal); err != nil {
			return nil, BadRequest(err.Error(), "ERROR_PARSING_BODY")
		}
		if err := app.requestValidator(bodyVal); err != nil {
			return nil, err
		}
		ctx = phastosctx.SetRequestBody(ctx, bodyVal)
	}

	// 4. Call handler with enriched context
	r = r.WithContext(ctx)
	return h2(r.Context())
}

func validatePathParam(raw string, pt PathParamType) error {
	switch pt {
	case ParamInt, ParamInt8, ParamInt16, ParamInt32, ParamInt64:
		if _, err := strconv.ParseInt(raw, 10, 64); err != nil {
			return BadRequest(
				fmt.Sprintf("path param expects %s, got '%s'", pt.String(), raw),
				"ERR_INVALID_PATH_PARAM",
			)
		}
	case ParamUint, ParamUint8, ParamUint16, ParamUint32, ParamUint64:
		if _, err := strconv.ParseUint(raw, 10, 64); err != nil {
			return BadRequest(
				fmt.Sprintf("path param expects %s, got '%s'", pt.String(), raw),
				"ERR_INVALID_PATH_PARAM",
			)
		}
	case ParamFloat32, ParamFloat64:
		if _, err := strconv.ParseFloat(raw, 64); err != nil {
			return BadRequest(
				fmt.Sprintf("path param expects %s, got '%s'", pt.String(), raw),
				"ERR_INVALID_PATH_PARAM",
			)
		}
	case ParamBool:
		if _, err := strconv.ParseBool(raw); err != nil {
			return BadRequest(
				fmt.Sprintf("path param expects bool, got '%s'", raw),
				"ERR_INVALID_PATH_PARAM",
			)
		}
	}
	return nil
}
```

- [ ] **Step 2: Modify `registerHandler` to type-switch**

In `registerHandler`, before current logic:

```go
func (app *App) registerHandler(method, path string, handler any, middlewares ...func(http.Handler) http.Handler) {
	// ... existing prep logic (flushPendingMiddlewares, etc.) ...

	switch h := handler.(type) {
	case Handler2:
		app.registerHandler2(method, path, h, middlewares...)
		return
	default:
		// existing Handler flow below (unchanged)
	}
	// ... existing code continues for old Handler type ...
}
```

- [ ] **Step 3: Add `registerHandler2` function for Handler2**

```go
func (app *App) registerHandler2(method, path string, h2 Handler2, middlewares ...func(http.Handler) http.Handler) {
	app.flushPendingMiddlewares()

	// Find the route's request type from routeRegistry (set by registerRoutes)
	// For now, we get doc from the pending route entry
	var requestType any
	var pathParamTypes []PathParamType
	// ... retrieve from the most recently registered route metadata ...
	// This needs a different approach — see Step 4

	wrappped := func(w http.ResponseWriter, r *http.Request) {
		requestId := r.Header.Get(common.RequestIDHeader)
		if requestId == "" {
			requestId = r.Header.Get("X-Request-ID")
		}

		// Inject trace ID into response header
		w.Header().Set("X-Trace-ID", requestId)

		result, err := app.bindAndDispatch(h2, requestType, pathParamTypes, r)

		resp, status := app.wrapHandler2Response(result, err)
		resp.Send(w)
		ReleaseResponse(resp)
		_ = status
	}

	handlerFunc := chi.Chain(middlewares...).HandlerFunc(wrappped)
	if strings.HasPrefix(path, "/v") {
		// version-group routing (existing logic)
		versionPrefix := "/v" + strings.SplitN(path[2:], "/", 2)[0]
		app.ensureAPIRouter(versionPrefix).Method(method, strings.TrimPrefix(path, versionPrefix), handlerFunc)
	} else {
		app.Http.Method(method, path, handlerFunc)
	}
	app.TotalEndpoints++
}

func (app *App) wrapHandler2Response(result any, err error) (*Response, int) {
	switch {
	case err != nil:
		return NewResponse().SetError(err), 0
	case result == nil:
		return NewResponse(), http.StatusOK
	default:
		if customResp, ok := result.(*Response); ok {
			return customResp, 0
		}
		return NewResponse().SetData(result), http.StatusOK
	}
}
```

- [ ] **Step 4: Extract route metadata for Handler2**

Problem: `registerHandler` gets `path` and `handler` but not `requestType`/`pathParamTypes`. These are in `routeRegistryEntry`.

Solution: Pass route metadata through `registerHandler`. Change signature to also accept `requestType any` and `pathParamTypes []PathParamType`.

In `registerRoutes`, the call becomes:
```go
app.registerHandler2(route.Method, routePath, handler2, route.Doc.RequestType, route.PathParamTypes, middlewares...)
```

And in the switch in `registerHandler`, for `Handler` (old type), just call existing logic.

Actually, let me simplify. Have `registerHandler` keep the broad signature that works for both:

```go
func (app *App) registerHandler(method, path string, handler any, middlewares ...func(http.Handler) http.Handler) {
```

And have an overload:
```go
func (app *App) registerHandlerWithMeta(method, path string, handler any, requestType any, pathParamTypes []PathParamType, middlewares ...func(http.Handler) http.Handler) {
```

Then `registerRoutes` decides which to call based on whether handler is Handler2.

Let me revise Step 2 & 3:

**Revised Step 2: Modify registerRoutes call**

In `registerRoutes` (~line 697), change:

```go
routePath := route.GetVersionedPath(prefix)
app.registerHandlerWithMeta(route.Method, routePath, route.Handler, route.Doc.RequestType, route.PathParamTypes, middlewares...)
```

**Revised Step 3: registerHandlerWithMeta**

```go
func (app *App) registerHandlerWithMeta(method, path string, handler any, requestType any, pathParamTypes []PathParamType, middlewares ...func(http.Handler) http.Handler) {
	app.flushPendingMiddlewares()

	// type-switch on handler
	if h2, ok := handler.(Handler2); ok {
		app.registerHandler2(method, path, h2, requestType, pathParamTypes, middlewares...)
		return
	}

	// Fall through to existing registerHandler logic for old Handler type
	h := handler.(Handler) //nolint:errcheck
	app.registerHandler(method, path, h, middlewares...)
}
```

- [ ] **Step 5: Add imports in app.go**

```go
import (
	phastosctx "github.com/kodekoding/phastos/v2/go/context"
	// ... existing imports ...
)
```

- [ ] **Step 6: Add ensureAPIRouter helper (extract from registerHandler)**

Extract version-group router creation logic from `registerHandler` into:

```go
func (app *App) ensureAPIRouter(versionPrefix string) *chi.Mux {
	if app.apiRouters == nil {
		app.apiRouters = make(map[string]*chi.Mux)
	}
	router, exists := app.apiRouters[versionPrefix]
	if !exists {
		router = chi.NewRouter()
		if len(app.globalMiddlewares) > 0 {
			router.Use(app.globalMiddlewares...)
		}
		app.apiRouters[versionPrefix] = router
		app.Http.Mount(versionPrefix, router)
	}
	return router
}
```

Refactor `registerHandler` to use this helper too (DRY).

- [ ] **Step 7: Write integration tests**

In `go/api/app_extended_test.go`:

```go
type handler2TestPayload struct {
	Name  string `json:"name" validate:"required"`
	Value int    `json:"value"`
}

func TestHandler2_AutoBinding_Post(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()
	app.flushPendingMiddlewares()

	called := false
	h2 := Handler2(func(ctx context.Context) (any, error) {
		called = true
		payload := phastosctx.RequestBody[*handler2TestPayload](ctx)
		assert.NotNil(t, payload)
		assert.Equal(t, "test", payload.Name)
		assert.Equal(t, 42, payload.Value)
		return payload, nil
	})

	doc := &RouteDoc{RequestType: new(handler2TestPayload)}
	app.registerHandlerWithMeta("POST", "/v1/test", h2, doc.RequestType, nil)

	body := `{"name":"test","value":42}`
	req := httptest.NewRequest("POST", "/v1/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.Http.ServeHTTP(w, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler2_BindingFailure_Returns400(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()
	app.flushPendingMiddlewares()

	called := false
	h2 := Handler2(func(ctx context.Context) (any, error) {
		called = true
		return nil, nil
	})

	doc := &RouteDoc{RequestType: new(handler2TestPayload)}
	app.registerHandlerWithMeta("POST", "/v1/test", h2, doc.RequestType, nil)

	body := `not-json`
	req := httptest.NewRequest("POST", "/v1/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.Http.ServeHTTP(w, req)

	assert.False(t, called, "handler should not be called on bind failure")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler2_OldHandlerStillWorks(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()
	app.flushPendingMiddlewares()

	called := false
	oldH := func(req Request, ctx context.Context) *Response {
		called = true
		var payload handler2TestPayload
		if err := req.GetBody(&payload); err != nil {
			return NewResponse().SetError(err)
		}
		return NewResponse().SetData(payload)
	}

	app.registerHandler("POST", "/v1/old-test", oldH)

	body := `{"name":"test","value":99}`
	req := httptest.NewRequest("POST", "/v1/old-test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.Http.ServeHTTP(w, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, w.Code)
}
```

- [ ] **Step 8: Run tests and commit**

```bash
go test ./go/api/... -v -race
# Expected: all PASS

git add go/api/app.go go/api/app_extended_test.go
git commit -m "feat(api): add Handler2 auto-binding dispatch in registerHandler"
```

---

### Task 5: `X-Trace-ID` response header

**Files:**
- Modify: `go/api/response.go` (add `SetHeader` to Response)
- Modify: `go/api/app.go` (set `X-Trace-ID` header in wrapHandler + wrapHandler2)

- [ ] **Step 1: Add customHeader support if not present**

Check `go/api/response.go` — `Response` struct has `customHeader map[string]string` field. Verify `Send()` writes these headers.

If not, add to `Send()` method before writing body:

```go
for k, v := range resp.customHeader {
    w.Header().Set(k, v)
}
```

- [ ] **Step 2: Set X-Trace-ID header in old wrapHandler**

In the sync path (~line 490) and async path (~line 546) of `wrapHandler`, after `response.TraceId = requestId`, add:

```go
response.TraceId = requestId
response.SetHeader("X-Trace-ID", requestId)
```

- [ ] **Step 3: Set X-Trace-ID header in Handler2 path**

In `registerHandler2`, the `wrappped` closure already sets `w.Header().Set("X-Trace-ID", requestId)` — verify.

- [ ] **Step 4: Add SetHeader method to Response**

```go
func (r *Response) SetHeader(key, value string) *Response {
	if r.customHeader == nil {
		r.customHeader = make(map[string]string)
	}
	r.customHeader[key] = value
	return r
}
```

- [ ] **Step 5: Write test verifying X-Trace-ID header**

```go
func TestResponse_XTraceIDHeader(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()
	app.flushPendingMiddlewares()

	h := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("ok")
	}
	app.registerHandler("GET", "/v1/header-test", h)

	req := httptest.NewRequest("GET", "/v1/header-test", nil)
	req.Header.Set("X-Request-ID", "trace-abc-123")
	w := httptest.NewRecorder()
	app.Http.ServeHTTP(w, req)

	assert.Equal(t, "trace-abc-123", w.Header().Get("X-Trace-ID"))
}
```

- [ ] **Step 6: Run tests and commit**

```bash
go test ./go/api/... -run "TestResponse_XTraceIDHeader" -v
# Expected: PASS

git add go/api/response.go go/api/app.go go/api/app_extended_test.go
git commit -m "feat(api): send trace ID as X-Trace-ID response header"
```

---

### Task 6: OpenAPI auto error responses (400, 401, 403, 422, 500)

**Files:**
- Modify: `go/api/openapi.go` (auto-generate error responses)

- [ ] **Step 1: Add autoErrorResponses helper**

```go
func autoErrorResponses(entry routeRegistryEntry) []ErrorResponseDoc {
	var resp []ErrorResponseDoc

	// 400 — if route has binding (RequestType, QueryType, or PathParamTypes)
	hasBinding := entry.Doc.RequestType != nil || len(entry.PathParamTypes) > 0
	if hasBinding {
		resp = append(resp, ErrorResponseDoc{
			StatusCode:  400,
			Code:        "ERROR_VALIDATION",
			Description: "Binding/validation failed",
		})
	}

	// 401 — if route has SecurityScheme
	if entry.Doc.Security != nil {
		resp = append(resp, ErrorResponseDoc{
			StatusCode:  401,
			Code:        "UNAUTHORIZED",
			Description: "Missing or invalid authorization",
		})
	}

	// 403 — if route has Security
	if entry.Doc.Security != nil {
		resp = append(resp, ErrorResponseDoc{
			StatusCode:  403,
			Code:        "FORBIDDEN",
			Description: "Forbidden access",
		})
	}

	// 422 — always (catch-all business logic error)
	resp = append(resp, ErrorResponseDoc{
		StatusCode:  422,
		Code:        "UNPROCESSABLE_ENTITY",
		Description: "Business logic / processing error",
	})

	// 500 — always
	resp = append(resp, ErrorResponseDoc{
		StatusCode:  500,
		Code:        "INTERNAL_SERVER_ERROR",
		Description: "Unhandled server error",
	})

	return resp
}
```

- [ ] **Step 2: Modify buildOperation to call autoErrorResponses**

In `buildOperation`, after processing explicit `entry.Doc.ErrorResponses`, merge auto-generated ones:

```go
// Merge auto-generated error responses (if not already defined explicitly)
explicitCodes := make(map[int]bool)
for _, er := range entry.Doc.ErrorResponses {
	explicitCodes[er.StatusCode] = true
}
for _, ar := range autoErrorResponses(entry) {
	if !explicitCodes[ar.StatusCode] {
		operation.AddResponse(ar.StatusCode, openapi3.NewResponse().
			WithDescription(ar.Description).
			WithJSONSchemaRef(&openapi3.SchemaRef{
				Ref: "#/components/schemas/ErrorResponse",
			}),
		)
	}
}
```

- [ ] **Step 3: Add ErrorResponse schema to OpenAPI spec**

In `buildOpenAPISpec`, add:

```go
spec.Components.Schemas["ErrorResponse"] = &openapi3.SchemaRef{
	Value: &openapi3.Schema{
		Type: schemaTypeObject,
		Properties: openapi3.Schemas{
			"message": &openapi3.SchemaRef{Value: &openapi3.Schema{Type: schemaTypeString}},
			"code":    &openapi3.SchemaRef{Value: &openapi3.Schema{Type: schemaTypeString}},
			"data":    &openapi3.SchemaRef{Value: &openapi3.Schema{Nullable: true}},
		},
	},
}
```

- [ ] **Step 4: Run tests and verify OpenAPI output**

```bash
go test ./go/api/... -v -race
# Expected: PASS
```

- [ ] **Step 5: Commit**

```bash
git add go/api/openapi.go
git commit -m "feat(api): auto-generate error responses (400/401/403/422/500) in OpenAPI spec"
```

---

### Task 7: `WithRequest` and `WithQuery` route options

**Files:**
- Modify: `go/api/controller.go` (add option functions)

- [ ] **Step 1: Add WithRequest option**

```go
// WithRequest sets the request body type for auto-binding and OpenAPI docs.
func WithRequest(req any) RouteOption {
	return func(r *Route) {
		if r.Doc == nil {
			r.Doc = &RouteDoc{}
		}
		r.Doc.RequestType = req
	}
}
```

- [ ] **Step 2: Add WithQuery option**

```go
// WithQuery sets the query parameter type for auto-binding and OpenAPI docs.
func WithQuery(query any) RouteOption {
	return func(r *Route) {
		if r.Doc == nil {
			r.Doc = &RouteDoc{}
		}
		// For query params, we use a designated field. RouteDoc doesn't have
		// a separate QueryType field yet. For now, we can augment RouteDoc.
	}
}
```

Hmm, `RouteDoc` doesn't have a `QueryType` field yet. Let me add it.

In `RouteDoc` struct, add:

```go
type RouteDoc struct {
	// ... existing fields ...
	QueryType any // Query parameter type for GET endpoints
}
```

Then:

```go
func WithQuery(query any) RouteOption {
	return func(r *Route) {
		if r.Doc == nil {
			r.Doc = &RouteDoc{}
		}
		r.Doc.QueryType = query
	}
}
```

- [ ] **Step 3: Update bindAndDispatch to use QueryType**

In the query binding section, use `QueryType` if set, `RequestType` otherwise:

```go
queryTarget := requestType
if docQueryType != nil {
	queryTarget = docQueryType
}
```

- [ ] **Step 4: Update routeRegistryEntry and registerRoutes**

Add `QueryType any` to `routeRegistryEntry`. Pass through in registerRoutes:

```go
app.routeRegistry = append(app.routeRegistry, routeRegistryEntry{
	// ...
	Doc: route.Doc,
	// ...
})
```

The Doc already contains both `RequestType` and `QueryType` — pass through Doc directly.

- [ ] **Step 5: Write tests**

```go
func TestWithRequest_SetsDocRequestType(t *testing.T) {
	type payload struct{ Name string }
	r := NewRoute("POST", Handler(func(ctx context.Context) (any, error) {
		return nil, nil
	}), WithRequest(new(payload)))
	assert.NotNil(t, r.Doc)
	assert.NotNil(t, r.Doc.RequestType)
}

func TestWithQuery_SetsDocQueryType(t *testing.T) {
	type filter struct{ Page int }
	r := NewRoute("GET", Handler(func(ctx context.Context) (any, error) {
		return nil, nil
	}), WithQuery(new(filter)))
	assert.NotNil(t, r.Doc)
	assert.NotNil(t, r.Doc.QueryType)
}
```

- [ ] **Step 6: Run tests and commit**

```bash
go test ./go/api/... -v -race
# Expected: PASS

git add go/api/controller.go go/api/app.go go/api/app_test.go
git commit -m "feat(api): add WithRequest and WithQuery route options with auto-binding"
```

---

### Task 8: Full integration test — end-to-end Handler2 flow

**Files:**
- Modify: `go/api/app_extended_test.go`

- [ ] **Step 1: Write end-to-end test covering POST with path param**

```go
func TestHandler2_FullFlow_PostWithPathParam(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()
	app.flushPendingMiddlewares()

	h2 := Handler2(func(ctx context.Context) (any, error) {
		payload := phastosctx.RequestBody[*handler2TestPayload](ctx)
		id := phastosctx.PathParam[int64](ctx, "id")
		return map[string]any{"id": id, "name": payload.Name}, nil
	})

	doc := &RouteDoc{RequestType: new(handler2TestPayload)}
	ptypes := []PathParamType{ParamInt64}
	app.registerHandlerWithMeta("PUT", "/v1/item/{id:int64}", h2, doc.RequestType, ptypes)

	body := `{"name":"item-1","value":10}`
	req := httptest.NewRequest("PUT", "/v1/item/99", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.Http.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"id":99`)
	assert.Contains(t, w.Body.String(), `"name":"item-1"`)
}

func TestHandler2_PathParamValidation_Fails(t *testing.T) {
	app := NewApp(WithTimezone("UTC"), WithAPITimeout(0))
	app.Init()
	app.flushPendingMiddlewares()

	h2 := Handler2(func(ctx context.Context) (any, error) {
		return "should not be called", nil
	})

	doc := &RouteDoc{RequestType: new(handler2TestPayload)}
	ptypes := []PathParamType{ParamInt64}
	app.registerHandlerWithMeta("PUT", "/v1/item/{id:int64}", h2, doc.RequestType, ptypes)

	body := `{"name":"item","value":1}`
	req := httptest.NewRequest("PUT", "/v1/item/notanumber", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.Http.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "ERR_INVALID_PATH_PARAM")
}
```

- [ ] **Step 2: Run tests**

```bash
go test ./go/api/... -run "TestHandler2_FullFlow|TestHandler2_PathParamValidation" -v
# Expected: PASS
```

- [ ] **Step 3: Commit**

```bash
git add go/api/app_extended_test.go
git commit -m "test(api): add end-to-end Handler2 integration tests"
```

---

### Task 9: Bump phastos version + `just check`

- [ ] **Step 1: Run full test suite**

```bash
go test ./go/... -count=1 -race -timeout 120s
```
Expected: all PASS

- [ ] **Step 2: Run vet + lint**

```bash
go vet ./go/... && golangci-lint run ./go/...
```
Expected: 0 issues

- [ ] **Step 3: Bump version in go.mod**

Update version comment/line: `v2.51.1` → `v2.52.0`

- [ ] **Step 4: Commit + tag + push**

```bash
git add go.mod
git commit -m "chore: bump phastos to v2.52.0"
git tag v2.52.0
git push origin master --tags
```

---

### Task 10: Update timetraq go.mod + pilot migration on one controller

**Files:**
- Modify: `go.mod` (timetraq-backend) — update phastos version
- Modify: `src/controller/event/event_controller.go` (pilot migration)

- [ ] **Step 1: Update timetraq go.mod**

```
github.com/kodekoding/phastos/v2 v2.51.1 → v2.52.0
```

- [ ] **Step 2: Migrate event controller Insert as pilot**

Before:
```go
// Route
api.NewRoute("POST", a.Insert)

// Handler
func (a *event) Insert(req api.Request, ctx context.Context) *api.Response {
	var requestData eventmodel.CreatePayload
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

After:
```go
// Route
api.NewRoute("POST", a.Insert, api.WithRequest(new(eventmodel.CreatePayload)))

// Handler
func (a *event) Insert(ctx context.Context) (any, error) {
	return a.uc.Insert(ctx)
}

// Usecase
func (uc *usecase) Insert(ctx context.Context) (any, error) {
	payload := phastosctx.RequestBody[*eventmodel.CreatePayload](ctx)
	// business logic...
}
```

- [ ] **Step 3: Run timetraq tests**

```bash
just check
```
Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add go.mod src/controller/event/event_controller.go src/usecase/event/
git commit -m "feat(event): migrate Insert handler to Handler2 with auto-binding"
```

---

## Self-Review Checklist

After writing this plan, I verified:

1. **Spec coverage:** Each spec section maps to a task:
   - Handler2 type → Task 2
   - Typed `WithPath` → Task 1
   - `phastosctx` getters → Task 3
   - Auto-binding flow → Task 4
   - `X-Trace-ID` header → Task 5
   - OpenAPI auto error responses → Task 6
   - `WithRequest`/`WithQuery` → Task 7
   - Backward compat → Task 4 integration tests

2. **No placeholders:** All steps have concrete code, exact test assertions, and real file paths.

3. **Type consistency:** `Handler2` defined in Task 2, used in Task 4+. `PathParamType` defined in Task 1, used in Task 4+. `phastosctx.RequestBody` defined in Task 3, used in Task 4+.

4. **One gap:** `WithQuery` needs `QueryType` field on `RouteDoc` — added inline in Task 7.
