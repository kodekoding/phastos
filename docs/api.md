# API Framework

Phastos provides a full-featured HTTP API framework built on top of [chi](https://github.com/go-chi/chi). It includes routing, middleware, request binding, OpenAPI doc generation, error handling, monitoring, and built-in support for JWT auth, rate limiting, and SSE.

```go
import "github.com/kodekoding/phastos/v2/go/api"
```

## Creating an App

Create an application with `api.NewApp(opts ...Options)`, call `app.Init()` to initialize the router and plugins, then `app.Start()` to serve.

```go
app := api.NewApp(
    api.WithAPITimeout(30),
    api.WithOpenAPI(),
)
app.Init()

// register controllers...
app.AddController(myController)

if err := app.Start(); err != nil {
    log.Fatal().Err(err).Msg("server error")
}
```

### Options

| Option | Signature | Default | Description |
|--------|-----------|---------|-------------|
| `WithAppPort` | `(port int)` | `8000` | HTTP listen port |
| `WithAPITimeout` | `(seconds int)` | `3` | Per-request timeout in seconds. When `0`, handlers run synchronously on the same goroutine (no goroutine + channel overhead). |
| `ReadTimeout` | `(seconds int)` | `3` | HTTP server read timeout |
| `WriteTimeout` | `(seconds int)` | `3` | HTTP server write timeout |
| `WithTimezone` | `(timezone string)` | `"Asia/Jakarta"` | Timezone for date/time operations |
| `WithNewRelic` | `()` | off | Enables New Relic APM tracing |
| `WithOTel` | `()` | off | Enables OpenTelemetry tracing |
| `WithOpenAPI` | `()` | off | Enables auto-generated OpenAPI 3.0.3 spec at `/docs` |
| `WithFastHttp` | `()` | off | Use valyala/fasthttp instead of net/http |
| `WithSSE` | `()` | off | Enable Server-Sent Events hub at `/events` |
| `WithCronJob` | `(...timezone string)` | off | Enable cron scheduler |
| `WithPprof` | `(enabled bool)` | `true` | Enable pprof profiling at `/debug/pprof/` |
| `WithGlobalMiddleware` | `(handlers ...func(http.Handler) http.Handler)` | none | Global middleware applied to ALL routes |
| `WithSkipLogPaths` | `(paths ...string)` | none | Paths that skip request logging (`/ping` is always skipped) |

### Init

```go
func (app *App) Init()
```

`Init()` creates the chi router, caches environment variables, initializes built-in middleware (request logger, recoverer, panic handler), loads notification platforms, and initializes database connections.

### Start

```go
func (app *App) Start() error
```

`Start()` serves the HTTP server with graceful shutdown. If `WithOpenAPI()` was enabled, it also serves:
- `GET /docs/openapi.json` — the OpenAPI 3.0.3 spec
- `GET /docs` — Swagger UI

Security headers (XSS filter, content-type nosniff) and CORS are applied automatically. CORS origins and headers are configured via `CORS_ORIGIN` and `CORS_HEADER` environment variables.

### Server Configuration (TLS)

TLS is configured via the `Config` struct (embedded in `App`):

```go
app.Config.CertFile = "/path/to/cert.pem"
app.Config.KeyFile = "/path/to/key.pem"
```

When both `CertFile` and `KeyFile` are set, the server starts with TLS.

## Controllers & Routes

### Controller Interface

```go
type Controller interface {
    GetConfig() ControllerConfig
}

type ControllerConfig struct {
    Path        string
    Routes      []Route
    Middlewares *[]func(http.Handler) http.Handler
}
```

### Route

```go
type Route struct {
    Method         string
    Path           string
    Handler        any
    Version        int
    Middlewares    *[]func(http.Handler) http.Handler
    SubRoutes      []Route
    Doc            *RouteDoc
    PathParamTypes []PathParamType
}
```

#### Leaf Routes

```go
func NewRoute(method string, handler any, opts ...RouteOption) Route
```

```go
api.NewRoute("GET", handler,
    api.WithPath("/users"),
    api.WithVersion(1),
)
```

#### Route Groups (Nested Routes)

```go
func NewGroup(path string, subRoutes []Route, opts ...RouteOption) Route
```

Group middleware is inherited by all child routes:

```go
api.NewGroup("/admin", []api.Route{
    api.NewRoute("GET", listUsers, api.WithPath("/users")),
    api.NewRoute("GET", getReport, api.WithPath("/reports/{id:int64}")),
}, api.WithMiddleware(authMiddleware))
```

Groups can be nested arbitrarily deep. Middleware chains merge: parent (controller/outer) first, then child (group/inner).

#### Path Parameter Types

Phastos supports static-type annotations in URL patterns that are validated at runtime:

| Pattern | Type |
|---------|------|
| `{id:int}` | `ParamInt` |
| `{id:int8}` | `ParamInt8` |
| `{id:int16}` | `ParamInt16` |
| `{id:int32}` | `ParamInt32` |
| `{id:int64}` | `ParamInt64` |
| `{id:uint}` | `ParamUint` |
| `{id:uint8}` | `ParamUint8` |
| `{id:uint16}` | `ParamUint16` |
| `{id:uint32}` | `ParamUint32` |
| `{id:uint64}` | `ParamUint64` |
| `{id:float32}` | `ParamFloat32` |
| `{id:float64}` | `ParamFloat64` |
| `{id:bool}` | `ParamBool` |
| `{slug}` | `ParamString` (default) |

Type annotations are stripped from the chi route pattern at registration time. Invalid values produce a `400 Bad Request` response with code `ERR_INVALID_PATH_PARAM`.

#### Adding Controllers

```go
app.AddController(ctrl)        // register a single controller
app.AddControllers(ctrls)      // register multiple controllers via Controllers interface
```

### ControllerImpl

`api.ControllerImpl` provides base controller functionality with middleware registration:

```go
type ControllerImpl struct { ... }

func NewControllerImpl() *ControllerImpl
func (ctrl *ControllerImpl) UseMiddleware(key string) func(http.Handler) http.Handler
func (ctrl *ControllerImpl) JoinMiddleware(handlers ...func(http.Handler) http.Handler) *[]func(http.Handler) http.Handler
```

Usage in a custom controller:

```go
type UserController struct {
    *api.ControllerImpl
    userSvc UserService
}

func NewUserController(userSvc UserService) *UserController {
    return &UserController{
        ControllerImpl: api.NewControllerImpl(),
        userSvc:        userSvc,
    }
}

func (c *UserController) GetConfig() api.ControllerConfig {
    return api.ControllerConfig{
        Path: "/users",
        Middlewares: c.JoinMiddleware(
            c.UseMiddleware("jwt"),
        ),
        Routes: []api.Route{
            api.NewRoute("GET", api.HandlerV2(c.List),
                api.WithPath("/list"),
                api.WithSummary("List users"),
                api.WithTags("Users"),
                api.WithResponse([]UserResponse{}),
            ),
            api.NewRoute("POST", api.HandlerV2(c.Create),
                api.WithPath("/create"),
                api.WithSummary("Create user"),
                api.WithTags("Users"),
                api.WithRequest(CreateUserRequest{}),
                api.WithResponse(UserResponse{}),
            ),
        },
    }
}
```

## Handlers

Phastos supports two handler signatures.

### Legacy Handler

```go
type Handler func(Request, context.Context) *Response
```

Full control over the request and response lifecycle:

```go
func (c *UserController) GetByID(req api.Request, ctx context.Context) *api.Response {
    id := req.GetParams("id")
    user, err := c.userSvc.FindByID(ctx, id)
    if err != nil {
        return api.NewResponse().SetError(err)
    }
    return api.NewResponse().SetData(user)
}
```

### Request Binding

The `Request` struct provides helpers for extracting parameters:

```go
type Request struct {
    GetParams  func(key string, defaultValue ...string) string
    GetFile    func(key string) (multipart.File, *multipart.FileHeader, error)
    GetQuery   func(interface{}) error
    GetBody    func(interface{}) error
    GetHeaders func(interface{}) error
}
```

`GetQuery`, `GetBody`, and `GetHeaders` decode into a struct using `gorilla/schema`. `GetBody` auto-detects content type (JSON, form-urlencoded, multipart/form-data) and supports struct validation via `validate` tags.

```go
func (c *UserController) Search(req api.Request, ctx context.Context) *api.Response {
    var filter struct {
        Name   string `schema:"name"`
        Status string `schema:"status" validate:"required"`
    }
    if err := req.GetQuery(&filter); err != nil {
        return api.NewResponse().SetError(err)
    }
    // ...
}
```

### HandlerV2 (Auto-Binding)

```go
type HandlerV2 func(ctx context.Context) (any, error)
```

Simpler signature with automatic request binding via `RouteDoc` annotations:

```go
func (c *UserController) Create(ctx context.Context) (any, error) {
    // query params auto-injected via phastosctx.GetQueryParams(ctx)
    // body auto-injected via phastosctx.GetRequestBody(ctx)
    // path params auto-injected via phastosctx.GetPathParams(ctx)
    // all validated against validate tags

    req := phastosctx.GetRequestBody(ctx).(*CreateUserRequest)
    user, err := c.userSvc.Create(ctx, req)
    if err != nil {
        return nil, err
    }
    return user, nil
}
```

When `HandlerV2` is used with a `RouteDoc` containing `RequestType`/`QueryType`, the framework:
1. Validates path params against `PathParamTypes`
2. Binds query params (GET) or request body (POST/PUT/PATCH)
3. Runs struct validation
4. Stores bound values in context

The handler receives the enriched context. Return `(nil, err)` for errors; return `(data, nil)` for success. The framework wraps the return value as JSON.

## Route Documentation & OpenAPI

Enable with `api.WithOpenAPI()`. Phastos auto-generates an OpenAPI 3.0.3 specification from route annotations.

### RouteDoc

```go
type RouteDoc struct {
    Summary            string
    Description        string
    Tags               []string
    Deprecated         bool
    RequestType        any
    ResponseType       any
    QueryType          any
    SelectResponseType any
    ErrorResponses     []ErrorResponseDoc
    Headers            []HeaderDoc
    Security           *SecuritySchemeDoc
}
```

### Route Options for Documentation

```go
api.WithSummary("Get user by ID")
api.WithDescription("Returns a single user by their unique identifier")
api.WithTags("Users", "Public")
api.WithRequest(CreateUserRequest{})           // request body schema
api.WithQuery(UserQueryParams{})               // query parameter schema
api.WithResponse(UserResponse{})               // 200 response schema
api.WithMessageResponse()                      // {message: string} response
api.WithSelectResponse([]UserResponse{})       // wrapped select response
api.WithErrorResponse(404, "USER_NOT_FOUND", "The requested user was not found")
```

Example route with full documentation:

```go
api.NewRoute("POST", api.HandlerV2(handler),
    api.WithPath("/users/create"),
    api.WithSummary("Create a new user"),
    api.WithDescription("Creates a user account and sends a welcome email"),
    api.WithTags("Users"),
    api.WithRequest(CreateUserRequest{}),
    api.WithResponse(UserResponse{}),
    api.WithErrorResponse(400, "VALIDATION_ERROR", "Invalid request body"),
    api.WithErrorResponse(409, "EMAIL_EXISTS", "Email already registered"),
)
```

### Swagger UI

When OpenAPI is enabled, two endpoints are served:
- `GET /docs/openapi.json` — raw OpenAPI spec
- `GET /docs` — Swagger UI (loaded from CDN)

### Middleware Documentation

Middleware metadata is auto-injected into route docs during registration:

```go
app.RegisterMiddlewareFunc("jwt", middlewares.JWTAuth,
    api.WithSecurity("http", "Authorization", "header"),
    api.WithMiddlewareDescription("JWT Bearer token authentication"),
)

app.RegisterMiddlewareFunc("platform", middlewares.StaticAuth,
    api.WithRequiredHeader("X-Platform-Key", "Platform identifier", true),
    api.WithMiddlewareDescription("Platform-level static token auth"),
)
```

Global middleware metadata can be registered for headers that apply to ALL routes:

```go
app.AddGlobalMiddlewareMeta(
    api.WithRequiredHeader("X-Platform-Key", "Platform identifier", true),
)
```

## Middleware

### Built-in Middleware

Phastos ships with three built-in middleware in the `middlewares` package:

```go
import "github.com/kodekoding/phastos/v2/go/middlewares"
```

#### JWTAuth

```go
func JWTAuth(next http.Handler) http.Handler
```

Validates JWT Bearer tokens from the `Authorization` header. Requires `JWT_SIGNING_KEY` environment variable. On success, stores `JWTClaimData` in context. Returns `401 Unauthorized` on missing, invalid, or expired tokens.

#### StaticAuth

```go
func StaticAuth(next http.Handler) http.Handler
```

Validates a static secret token from the `X-Phastos-Secret` header against the `SERVICE_SECRET` environment variable. Returns `401 Unauthorized` on mismatch.

#### NewRateLimiter

```go
func NewRateLimiter(opts ...RateLimiterOption) func(http.Handler) http.Handler
```

Token-bucket rate limiter. Defaults to 10 req/s with burst of 20, keyed by IP address. Configurable via options.

```go
middlewares.NewRateLimiter(
    middlewares.WithRate(5.0, 10),
    middlewares.WithKeyExtractor(func(r *http.Request) string { ... }),
    middlewares.WithSkipPaths("/health", "/metrics"),
)
```

### Registering Middleware

```go
func (app *App) RegisterMiddlewareFunc(key string, middlewareHandler func(http.Handler) http.Handler, opts ...MiddlewareOption)
```

Register a named middleware for lookup by controllers:

```go
app.RegisterMiddlewareFunc("jwt", middlewares.JWTAuth,
    api.WithSecurity("http", "Authorization", "header"),
)
```

### Global Middleware

```go
// In NewApp():
app := api.NewApp(api.WithGlobalMiddleware(corsMiddleware))

// After Init() (for dependency-injected middleware):
app.AddGlobalMiddleware(loggingMiddleware)
```

### Per-Controller Middleware

Set in `ControllerConfig.Middlewares`:

```go
return api.ControllerConfig{
    Path: "/admin",
    Middlewares: c.JoinMiddleware(
        c.UseMiddleware("jwt"),
        c.UseMiddleware("rateLimit"),
    ),
    Routes: ...,
}
```

### Per-Route Middleware

```go
api.NewRoute("GET", handler,
    api.WithPath("/secret"),
    api.WithMiddleware(auditLog),
)
```

### Middleware Option Helpers

```go
api.WithSecurity(schemeType, name, in string)        // "http", "Authorization", "header"
api.WithRequiredHeader(name, desc string, required bool)
api.WithMiddlewareDescription(desc string)
```

## Response

### Creating Responses

Responses use a `sync.Pool` for zero-allocation reuse. Always call `ReleaseResponse` after `Send()`.

```go
resp := api.NewResponse()
resp.Send(w)
api.ReleaseResponse(resp)
```

### Builder Methods

```go
resp.SetData(user)                                    // JSON response with data
resp.SetData(user, true)                              // paginated data with metadata
resp.SetMessage("success")                            // JSON response with message field
resp.SetError(err)                                    // error response
resp.SetHTTPError(httpErr)                            // use a pre-built HttpError
resp.SetStatusCode(http.StatusCreated)                // custom status code
resp.SetFileDownload(data, "report.pdf", "application/pdf") // file download
```

All setter methods return `*Response` for chaining:

```go
return api.NewResponse().SetData(user).SetStatusCode(http.StatusCreated)
```

### Error Handling

`SetError(err)` automatically detects error types:
- `*HttpError` — used as-is (hides internal message for 500s)
- `*custerr.RequestError` with conflict status — maps PostgreSQL constraint names to friendly messages via the constraint registry
- Other errors — wrapped as `INTERNAL_SERVER_ERROR`

### Response Headers

Responses automatically include:
- `X-Container-Name` (from `CONTAINER_NAME` env)
- `X-App-Version` (from `APP_VERSION` env)
- `X-Commit-Hash` (from `COMMIT_HASH` env)
- `X-Trace-ID` (from request)

Custom headers can be added via `resp.SetCustomHeader(key, value)`.

## HTTP Errors

### HttpError

```go
type HttpError struct {
    Message    string      `json:"message"`
    Code       string      `json:"code"`
    Status     int         `json:"-"`
    TraceId    string      `json:"trace_id"`
    Data       interface{} `json:"data,omitempty"`
    CallerPath string      `json:"caller_path,omitempty"`
}
```

### Creating Errors with NewErr

```go
api.NewErr(
    api.WithErrorCode("VALIDATION_ERROR"),
    api.WithErrorMessage("validation error"),
    api.WithErrorStatus(http.StatusUnprocessableEntity),
    api.WithTraceId("abc123"),
    api.WithErrorData(details),
    api.WithErrorCallerPath("usecase.CreateUser"),
)
```

### Convenience Constructors

| Function | HTTP Status |
|----------|-------------|
| `BadRequest(msg, code)` | 400 |
| `Unauthorized(msg, code)` | 401 |
| `Forbidden(msg, code)` | 403 |
| `NotFound(msg, code)` | 404 |
| `MethodNotAllowed(msg, code)` | 405 |
| `ConflictError(msg, code)` | 409 |
| `UnprocessableEntity(msg, code)` | 422 |
| `TooManyRequest(msg, code)` | 429 |
| `InternalServerError(msg, code)` | 500 |

```go
if user == nil {
    return api.NewResponse().SetError(api.NotFound("user not found", "USER_NOT_FOUND"))
}
```

## Testing Handlers

### Testing HandlerV2 with WrapHandlerV2Meta

The exported `WrapHandlerV2Meta` wraps a `HandlerV2` with auto-binding metadata into an `http.HandlerFunc` for use with `httptest`:

```go
func TestCreateUser(t *testing.T) {
    app := api.NewApp(api.WithAPITimeout(0)) // sync mode recommended for tests
    app.Init()

    ctrl := NewUserController()

    body := `{"name":"Alice","email":"alice@example.com"}`
    req := httptest.NewRequest("POST", "/v1/users/create", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()

    wrapped := app.WrapHandlerV2Meta(api.HandlerV2Meta{
        H:           ctrl.Create,
        RequestType: CreateUserRequest{},
    })
    wrapped(rec, req)

    assert.Equal(t, http.StatusOK, rec.Code)
}
```

### Using httptest.Server (Full Integration)

For legacy handlers or full integration testing, spin up an `httptest.Server` with the app:

```go
func TestIntegration(t *testing.T) {
    app := api.NewApp(api.WithAppPort(0), api.WithAPITimeout(0))
    app.Init()
    app.AddController(NewUserController())

    srv := httptest.NewServer(app.Handler)
    defer srv.Close()

    resp, _ := http.Get(srv.URL + "/v1/users/list")
    assert.Equal(t, http.StatusOK, resp.StatusCode)
}
```

## Full Example

```go
package main

import (
    "context"

    "github.com/kodekoding/phastos/v2/go/api"
    "github.com/kodekoding/phastos/v2/go/middlewares"
)

type User struct {
    ID   int64  `json:"id"`
    Name string `json:"name"`
}

type UserController struct {
    *api.ControllerImpl
}

func NewUserController() *UserController {
    return &UserController{ControllerImpl: api.NewControllerImpl()}
}

func (c *UserController) GetConfig() api.ControllerConfig {
    return api.ControllerConfig{
        Path: "/users",
        Middlewares: c.JoinMiddleware(
            c.UseMiddleware("jwt"),
        ),
        Routes: []api.Route{
            api.NewRoute("GET", api.HandlerV2(c.List),
                api.WithPath("/list"),
                api.WithSummary("List all users"),
                api.WithTags("Users"),
                api.WithQuery(UserQuery{}),
                api.WithResponse([]User{}),
                api.WithErrorResponse(401, "UNAUTHORIZED", "Invalid token"),
            ),
            api.NewRoute("POST", api.HandlerV2(c.Create),
                api.WithPath("/create"),
                api.WithSummary("Create a user"),
                api.WithTags("Users"),
                api.WithRequest(CreateUserRequest{}),
                api.WithResponse(User{}),
            ),
            api.NewRoute("GET", c.GetByID,
                api.WithPath("/{id:int64}"),
                api.WithSummary("Get user by ID"),
                api.WithTags("Users"),
            ),
        },
    }
}

func (c *UserController) List(ctx context.Context) (any, error) {
    users := []User{{ID: 1, Name: "Alice"}}
    return users, nil
}

func (c *UserController) Create(ctx context.Context) (any, error) {
    return User{ID: 2, Name: "Bob"}, nil
}

func (c *UserController) GetByID(req api.Request, ctx context.Context) *api.Response {
    id := req.GetParams("id")
    return api.NewResponse().SetData(User{ID: 1, Name: "Alice (ID=" + id + ")"})
}

type UserQuery struct {
    Status string `schema:"status" validate:"omitempty"`
}

type CreateUserRequest struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
}

func main() {
    app := api.NewApp(
        api.WithAppPort(8080),
        api.WithAPITimeout(30),
        api.WithOpenAPI(),
    )

    app.Init()

    app.RegisterMiddlewareFunc("jwt", middlewares.JWTAuth,
        api.WithSecurity("http", "Authorization", "header"),
    )

    app.AddController(NewUserController())

    if err := app.Start(); err != nil {
        panic(err)
    }
}
```
