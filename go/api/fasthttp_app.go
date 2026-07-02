package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"

	"github.com/kodekoding/phastos/v2/go/common"
	"github.com/kodekoding/phastos/v2/go/helper"
	plog "github.com/kodekoding/phastos/v2/go/log"
)

// fastRequestPool reduces GC pressure by recycling *FastRequest objects.
var fastRequestPool = sync.Pool{
	New: func() interface{} {
		return &FastRequest{}
	},
}

// FastRequest is a zero-allocation request wrapper for fasthttp.
// Instead of closures (like the chi Request), it stores a pointer to
// the fasthttp.RequestCtx and provides methods that access it directly.
// This eliminates 5 closure allocations per request.
type FastRequest struct {
	ctx *fasthttp.RequestCtx
	app *FastHttpApp
}

// GetParam returns a path parameter by key.
// For fasthttp, path params are stored as UserValues set by the router.
func (r *FastRequest) GetParam(key string, defaultValue ...string) string {
	if v := r.ctx.UserValue(key); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	for _, v := range defaultValue {
		return v
	}
	return ""
}

// GetQuery decodes query parameters into a struct using gorilla/schema.
func (r *FastRequest) GetQuery(dest interface{}) error {
	args := r.ctx.QueryArgs()
	values := make(map[string][]string, args.Len())
	args.VisitAll(func(k, v []byte) {
		values[string(k)] = append(values[string(k)], string(v))
	})
	if err := fastDecoder.Decode(dest, values); err != nil {
		return BadRequest(err.Error(), "ERROR_PARSING_QUERY_PARAMS")
	}
	return r.app.requestValidator(dest)
}

// GetHeaders decodes request headers into a struct using gorilla/schema.
func (r *FastRequest) GetHeaders(dest interface{}) error {
	headers := make(map[string][]string)
	r.ctx.Request.Header.VisitAll(func(k, v []byte) {
		headers[string(k)] = append(headers[string(k)], string(v))
	})
	if err := fastDecoder.Decode(dest, headers); err != nil {
		return BadRequest(err.Error(), "ERROR_PARSING_HEADER")
	}
	return r.app.requestValidator(dest)
}

// GetBody decodes the request body into a struct based on Content-Type.
func (r *FastRequest) GetBody(dest interface{}) error {
	contentType := filterFlags(string(r.ctx.Request.Header.ContentType()))
	switch contentType {
	case ContentJSON:
		if err := json.Unmarshal(r.ctx.Request.Body(), dest); err != nil {
			return BadRequest(err.Error(), ErrParsedBodyCode)
		}
	case ContentURLEncoded, ContentFormData:
		args := r.ctx.PostArgs()
		values := make(map[string][]string, args.Len())
		args.VisitAll(func(k, v []byte) {
			values[string(k)] = append(values[string(k)], string(v))
		})
		if err := fastDecoder.Decode(dest, values); err != nil {
			return BadRequest(err.Error(), ErrDecodeBodyCode)
		}
	default:
		if err := json.Unmarshal(r.ctx.Request.Body(), dest); err != nil {
			return BadRequest(err.Error(), ErrParsedBodyCode)
		}
	}
	return r.app.requestValidator(dest)
}

// GetFile returns a multipart file by key.
func (r *FastRequest) GetFile(key string) (interface{}, error) {
	part, err := r.ctx.FormFile(key)
	if err != nil {
		return nil, err
	}
	return part, nil
}

// Header returns a specific request header value.
func (r *FastRequest) Header(key string) string {
	return string(r.ctx.Request.Header.Peek(key))
}

// Ctx returns the underlying fasthttp.RequestCtx for direct access.
// This enables FastDirectHandler to create FastResponse without additional indirection.
func (r *FastRequest) Ctx() *fasthttp.RequestCtx {
	return r.ctx
}

// release resets and returns a FastRequest to the pool.
func (r *FastRequest) release() {
	r.ctx = nil
	r.app = nil
	fastRequestPool.Put(r)
}

// FastResponse writes directly to fasthttp.RequestCtx with zero intermediate
// allocations. For simple message/data responses it avoids json.Marshal
// by using fasthttp's built-in body writer.
type FastResponse struct {
	ctx        *fasthttp.RequestCtx
	statusCode int
}

// fastResponsePool reduces GC pressure by recycling *FastResponse objects.
var fastResponsePool = sync.Pool{
	New: func() interface{} {
		return &FastResponse{statusCode: http.StatusOK}
	},
}

// cachedBackground avoids allocating context.Background() per request.
var cachedBackground = context.Background()

// NewFastResponse creates a FastResponse that writes directly to the ctx.
// Uses sync.Pool to avoid heap allocation per request.
func NewFastResponse(ctx *fasthttp.RequestCtx) *FastResponse {
	fr := fastResponsePool.Get().(*FastResponse) //nolint:errcheck
	fr.ctx = ctx
	fr.statusCode = http.StatusOK
	return fr
}

// release resets and returns a FastResponse to the pool.
func (fr *FastResponse) release() {
	fr.ctx = nil
	fr.statusCode = http.StatusOK
	fastResponsePool.Put(fr)
}

// SetMessage writes a JSON {"message":"..."} response directly to the ctx.
// This is the fast path — no json.Marshal, no map allocation, no intermediate buffer.
// Uses SetBodyString to write the complete body in a single operation.
func (fr *FastResponse) SetMessage(msg string) *FastResponse {
	fr.ctx.Response.Header.SetContentType("application/json")
	fr.ctx.SetStatusCode(fr.statusCode)
	fr.ctx.SetBodyString(`{"message":"` + msg + `"}`)
	return fr
}

// SetData writes a JSON response with arbitrary data using json.Marshal.
// This is the slower path but still avoids the Response struct pool overhead.
func (fr *FastResponse) SetData(data any) *FastResponse {
	fr.ctx.Response.Header.SetContentType("application/json")
	fr.ctx.SetStatusCode(fr.statusCode)
	b, _ := json.Marshal(data)
	fr.ctx.SetBody(b)
	return fr
}

// SetError writes an error JSON response.
func (fr *FastResponse) SetError(err *HttpError) *FastResponse {
	fr.ctx.Response.Header.SetContentType("application/json")
	fr.ctx.SetStatusCode(err.Status)
	b, _ := json.Marshal(err)
	fr.ctx.SetBody(b)
	return fr
}

// SetStatusCode sets the HTTP status code.
func (fr *FastResponse) SetStatusCode(code int) *FastResponse {
	fr.statusCode = code
	return fr
}

// FastHandler is the handler function type for the fasthttp-native path.
// It receives a FastRequest (with methods for params/query/body/headers)
// and a context.Context, and returns a *Response — same signature as chi Handler.
// This is the compatibility path that still goes through Response + json.Marshal.
type FastHandler func(FastRequest, context.Context) *Response

// FastDirectHandler is the optimized handler type that writes directly
// to the fasthttp.RequestCtx via FastResponse. No intermediate Response
// struct, no json.Marshal for simple responses. This is the fastest path.
type FastDirectHandler func(req *FastRequest) *FastResponse

// FastMiddleware is the middleware type for the fasthttp-native path.
type FastMiddleware func(fasthttp.RequestHandler) fasthttp.RequestHandler

// FastController is the controller interface for the fasthttp-native path.
type FastController interface {
	GetConfig() FastControllerConfig
}

// FastControllerConfig holds controller configuration.
type FastControllerConfig struct {
	Path        string
	Routes      []FastRoute
	Middlewares []FastMiddleware
}

// FastRoute holds route configuration.
type FastRoute struct {
	Method        string
	Path          string
	Handler       FastHandler       // compatibility path (returns *Response)
	DirectHandler FastDirectHandler // optimized path (returns *FastResponse, writes directly)
	Version       int
	Middlewares   []FastMiddleware
	SubRoutes     []FastRoute
}

func (r *FastRoute) GetVersionedPath(controllerPath string) string {
	versionPrefix := "/v" + strconv.Itoa(r.Version)
	return versionPrefix + controllerPath + r.Path
}

// fastDecoder is a gorilla/schema decoder instance for the fasthttp path.
var fastDecoder = schema.NewDecoder()

// --- Lightweight fasthttp router ---
// Instead of depending on an external router package, we implement a simple
// radix-tree-free router that uses a map of exact paths + a list of param routes.
// For the benchmark use case this is extremely fast. For production, you can
// swap in github.com/fasthttp/router or any fasthttp-compatible router.

// routeEntry stores a registered route.
type routeEntry struct {
	method  string
	path    string // may contain :param segments
	handler fasthttp.RequestHandler
}

// fastRouter is a minimal fasthttp router supporting exact and :param paths.
type fastRouter struct {
	exactRoutes map[string]fasthttp.RequestHandler // "GET /v1/json" -> handler
	paramRoutes []routeEntry                       // routes with :param, checked sequentially
	notFound    fasthttp.RequestHandler
}

var notFoundJSON = func() []byte {
	b, _ := json.Marshal(NotFound("route not found", "ROUTE_NOT_FOUND"))
	return b
}()

func newFastRouter() *fastRouter {
	return &fastRouter{
		exactRoutes: make(map[string]fasthttp.RequestHandler),
		notFound: func(ctx *fasthttp.RequestCtx) {
			ctx.SetStatusCode(http.StatusNotFound)
			ctx.SetContentType("application/json")
			ctx.SetBody(notFoundJSON)
		},
	}
}

// Handle registers a route. Path may contain :param segments (e.g. "/v1/path/:id").
func (r *fastRouter) Handle(method, path string, handler fasthttp.RequestHandler) {
	if !strings.Contains(path, ":") && !strings.Contains(path, "{") {
		// Exact path — no params — use map for O(1) lookup
		r.exactRoutes[method+" "+path] = handler
		return
	}
	// Parametric path — use sequential scan (fast for small route counts)
	r.paramRoutes = append(r.paramRoutes, routeEntry{
		method:  method,
		path:    path,
		handler: handler,
	})
}

// ServeHTTP implements fasthttp.RequestHandler.
func (r *fastRouter) ServeHTTP(ctx *fasthttp.RequestCtx) {
	// Build lookup key on the stack — zero allocation for exact match path.
	method := ctx.Method()
	path := ctx.URI().Path()
	var key [128]byte
	n := copy(key[:], method)
	key[n] = ' '
	n++
	n += copy(key[n:], path)

	// O(1) exact match (string() from stack buffer, no escape → no alloc)
	if h, ok := r.exactRoutes[string(key[:n])]; ok {
		h(ctx)
		return
	}

	// Parametric path: need string for matching functions
	pathStr := string(path)

	methodStr := string(method)

	// Slower path: parametric match
	for _, route := range r.paramRoutes {
		if route.method != methodStr {
			continue
		}
		if matchParamPath(route.path, pathStr, ctx) {
			route.handler(ctx)
			return
		}
	}

	r.notFound(ctx)
}

// matchParamPath matches a pattern like "/v1/path/:id" against an actual path.
// It sets UserValues directly on the ctx during matching, avoiding map allocation.
// Returns true if the path matches the pattern.
func matchParamPath(pattern, path string, ctx *fasthttp.RequestCtx) bool {
	// Fast check: same number of segments
	pi := 0 // position in pattern
	li := 0 // last '/' position in path
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '/' {
			// Extract path segment: path[li:i]
			// Find corresponding pattern segment
			pj := pi
			for pj < len(pattern) && pattern[pj] != '/' {
				pj++
			}
			// pattern segment is pattern[pi:pj]
			if pj-pi == 0 && i-li == 0 {
				// Both empty (leading /)
				pi = pj + 1
				li = i + 1
				continue
			}
			if pj-pi > 0 && pattern[pi] == ':' || pattern[pi] == '{' {
				// Param segment — set UserValue
				paramName := pattern[pi+1 : pj]
				if pattern[pi] == '{' {
					paramName = strings.TrimSuffix(paramName, "}")
				}
				ctx.SetUserValue(paramName, path[li:i])
			} else {
				// Literal segment — must match exactly
				if pj-pi != i-li || pattern[pi:pj] != path[li:i] {
					return false
				}
			}
			pi = pj + 1
			li = i + 1
		}
	}
	// Check that pattern is also exhausted
	for pi < len(pattern) {
		if pattern[pi] != '/' {
			return false
		}
		pi++
	}
	return true
}

// FastHttpApp is a native fasthttp application that bypasses net/http entirely.
// It provides the same Phastos API surface (Request/Response, middleware chain,
// controller pattern) but with zero net/http overhead.
type FastHttpApp struct {
	router             *fastRouter
	apiTimeout         int
	pprofEnabled       bool
	middlewares        map[string]any
	globalMiddlewares  []FastMiddleware
	pendingMiddlewares bool
	skipLogPaths       map[string]struct{}
	sfActive           bool
	syncMode           bool
	TotalEndpoints     int
}

// NewFastHttpApp creates a new native fasthttp application.
func NewFastHttpApp() *FastHttpApp {
	app := &FastHttpApp{
		middlewares:  make(map[string]any),
		pprofEnabled: false,
		apiTimeout:   0,
	}

	if val := os.Getenv("SINGLEFLIGHT_ACTIVE"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			app.sfActive = parsed
		}
	}
	app.syncMode = app.apiTimeout == 0 && !app.sfActive

	return app
}

// Init initializes the fasthttp router.
func (app *FastHttpApp) Init() {
	app.router = newFastRouter()
	app.pendingMiddlewares = true
}

// RegisterMiddlewareFunc registers a middleware by key for later use by controllers.
func (app *FastHttpApp) RegisterMiddlewareFunc(key string, mw FastMiddleware) {
	app.middlewares[key] = mw
}

// AddGlobalMiddleware adds middleware(s) that run on every request.
func (app *FastHttpApp) AddGlobalMiddleware(handlers ...FastMiddleware) {
	app.globalMiddlewares = append(app.globalMiddlewares, handlers...)
	app.pendingMiddlewares = true
}

// AddController registers all routes from a FastController.
func (app *FastHttpApp) AddController(ctrl FastController) {
	app.flushPendingMiddlewares()

	config := ctrl.GetConfig()
	for _, route := range config.Routes {
		routePath := route.GetVersionedPath(config.Path)

		var handler fasthttp.RequestHandler
		if route.DirectHandler != nil {
			handler = app.wrapDirectHandler(route.DirectHandler)
		} else {
			handler = app.wrapFastHandler(route.Handler)
		}

		// Apply route-specific middlewares (in reverse for correct ordering)
		for i := len(route.Middlewares) - 1; i >= 0; i-- {
			handler = route.Middlewares[i](handler)
		}

		// Apply controller-level middlewares
		for i := len(config.Middlewares) - 1; i >= 0; i-- {
			handler = config.Middlewares[i](handler)
		}

		app.router.Handle(route.Method, routePath, handler)
		app.TotalEndpoints++
	}
}

// flushPendingMiddlewares applies global middlewares and registers default routes.
func (app *FastHttpApp) flushPendingMiddlewares() {
	if !app.pendingMiddlewares {
		return
	}
	app.pendingMiddlewares = false
	app.initDefaultRoutes()
}

// initDefaultRoutes registers /ping and other default routes.
func (app *FastHttpApp) initDefaultRoutes() {
	app.router.Handle("GET", "/ping", func(ctx *fasthttp.RequestCtx) {
		msgString := fmt.Sprintf("pong on datetime: %s", GetDateTimeNowStringWithTimezone())
		response := NewResponse().SetMessage(msgString)
		fastSendResponse(ctx, response)
		ReleaseResponse(response)
	})
	app.TotalEndpoints++
}

// wrapFastHandler wraps a FastHandler into a fasthttp.RequestHandler.
// Core of the native fasthttp path — no net/http, no chi, no context.WithValue.
// Note: request ID is already set in the header by fastInitHandler (outer middleware),
// so we just read it here instead of generating it again.
func (app *FastHttpApp) wrapFastHandler(h FastHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		// Read request ID from request header (set by client or fastInitHandler fallback)
		requestIdBytes := ctx.Request.Header.Peek(common.RequestIDHeader)
		var requestId string
		if len(requestIdBytes) > 0 {
			requestId = string(requestIdBytes)
		} else {
			// Fallback: read from response header set by fastInitHandler
			requestId = string(ctx.Response.Header.Peek(common.RequestIDHeader))
		}

		// Build FastRequest from pool — zero closure allocation
		req := fastRequestPool.Get().(*FastRequest) //nolint:errcheck
		req.ctx = ctx
		req.app = app
		defer req.release()

		// Execute handler directly — no goroutine, no channel, no context.WithTimeout
		response := h(*req, cachedBackground)
		response.TraceId = requestId

		// Send response directly to fasthttp
		fastSendResponse(ctx, response)
		ReleaseResponse(response)
	}
}

// wrapDirectHandler wraps a FastDirectHandler into a fasthttp.RequestHandler.
// This is the fastest path: no intermediate Response struct, no json.Marshal
// for simple responses. FastResponse writes directly to fasthttp.RequestCtx.
func (app *FastHttpApp) wrapDirectHandler(h FastDirectHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		// Request ID already set in response header by fastInitHandler

		// Build FastRequest from pool — zero closure allocation
		req := fastRequestPool.Get().(*FastRequest) //nolint:errcheck
		req.ctx = ctx
		req.app = app

		// Execute direct handler — writes straight to ctx
		fr := h(req)

		// Release pooled objects without defer (saves ~20-40ns per request)
		fr.release()
		req.release()
	}
}

// fastSendResponse writes a *Response directly to a fasthttp.RequestCtx.
// This bypasses net/http's ResponseWriter entirely.
func fastSendResponse(ctx *fasthttp.RequestCtx, resp *Response) {
	// Set common headers (env vars cached at init, zero cost per request)
	if cachedContainerName != "" {
		ctx.Response.Header.Set("X-Container-Name", cachedContainerName)
	}
	if appVersion != "" {
		ctx.Response.Header.Set("X-App-Version", appVersion)
	}
	if commitHash != "" {
		ctx.Response.Header.Set("X-Commit-Hash", commitHash)
	}

	// Set custom headers
	if resp.customHeader != nil {
		for k, v := range resp.customHeader {
			ctx.Response.Header.Set(k, v)
		}
	}

	// File download response
	if resp.fileData != nil {
		ctx.Response.Header.Set("Content-Type", resp.fileContentType)
		ctx.Response.Header.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, resp.fileDownloadName))
		ctx.Response.Header.Set("Content-Length", strconv.Itoa(len(resp.fileData)))
		ctx.SetStatusCode(resp.statusCode)
		ctx.SetBody(resp.fileData)
		return
	}

	// JSON response
	ctx.Response.Header.Set("Content-Type", "application/json")

	var dataToMarshal any
	var responseStatus int

	if resp.Err != nil {
		var respErr *HttpError
		if errors.As(errors.Cause(resp.Err), &respErr) {
			responseStatus = respErr.Status
			if respErr.TraceId == "" && resp.TraceId != "" {
				respErr.TraceId = resp.TraceId
			}
			dataToMarshal = respErr
		}
	} else {
		responseStatus = resp.statusCode
		bodyContentAvailable := false
		if resp.Data != nil {
			bodyContentAvailable = true
			dataToMarshal = resp.Data
			if resp.MetaData != nil && resp.isPaginationData {
				dataToMarshal = map[string]any{
					"data":     resp.Data,
					"metadata": resp.MetaData,
				}
			}
		}
		if resp.Message != "" && resp.Data == nil {
			// Fast path: message-only response — skip json.Marshal and map allocation
			ctx.SetStatusCode(responseStatus)
			ctx.SetBodyString(`{"message":"` + resp.Message + `"}`)
			return
		}
		if resp.Message != "" {
			bodyContentAvailable = true
			dataToMarshal = map[string]string{
				"message": resp.Message,
			}
		}
		if !bodyContentAvailable {
			resp.statusCode = http.StatusNoContent
			responseStatus = http.StatusNoContent
		}
	}

	ctx.SetStatusCode(responseStatus)
	if dataToMarshal != nil {
		b, _ := json.Marshal(dataToMarshal)
		ctx.SetBody(b)
	}
}

// Handler returns the composed fasthttp.RequestHandler with all middlewares applied.
// Built-in middlewares (init, panic, logger) are fused into a single handler
// to eliminate 3 indirect function calls per request (~60-90ns savings).
func (app *FastHttpApp) Handler() fasthttp.RequestHandler {
	app.flushPendingMiddlewares()

	// Start with the router as the innermost handler
	var handler fasthttp.RequestHandler = app.router.ServeHTTP

	// Apply global middlewares (in reverse so they execute in order)
	for i := len(app.globalMiddlewares) - 1; i >= 0; i-- {
		handler = app.globalMiddlewares[i](handler)
	}

	// Fuse built-in middlewares into a single closure when logging is disabled.
	// This eliminates 3 indirect function call overheads per request.
	if zerolog.GlobalLevel() == zerolog.Disabled {
		// Fused handler: initHandler + panicHandler + router in one closure
		return func(ctx *fasthttp.RequestCtx) {
			// --- inline fastInitHandler ---
			requestIdBytes := ctx.Request.Header.Peek(common.RequestIDHeader)
			if len(requestIdBytes) == 0 {
				requestIdBytes = ctx.Request.Header.Peek("X-Request-ID")
			}
			if len(requestIdBytes) > 0 {
				ctx.Response.Header.SetBytesV(common.RequestIDHeader, requestIdBytes)
			} else {
				idBuf := helper.GenerateFastIDCounterBytes()
				ctx.Response.Header.SetBytesV(common.RequestIDHeader, *idBuf)
				helper.PutFastIDCounterBytes(idBuf)
			}

			// --- inline fastPanicHandler ---
			defer func() {
				if rvr := recover(); rvr != nil {
					ctx.SetStatusCode(http.StatusInternalServerError)
					err := InternalServerError("server error", "SERVER_ERROR")
					b, _ := json.Marshal(err)
					ctx.SetContentType("application/json")
					ctx.SetBody(b)
				}
			}()

			handler(ctx)
		}
	}

	// Normal path: logging enabled, use separate middlewares
	handler = fastRequestLogger(handler, app.skipLogPaths)
	handler = fastPanicHandler(handler)
	handler = fastInitHandler(handler)

	return handler
}

// --- Built-in fasthttp middlewares ---

// fastInitHandler generates or propagates request IDs.
// Only sets the ID on the response header — avoids the expensive
// Request.Header.Set which triggers CopyTo. Downstream reads
// the ID from the response header or uses Peek on the request header.
// Uses GenerateFastIDCounterBytes + SetBytesV to avoid string allocation.
func fastInitHandler(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		// Check for client-provided ID first (benchmark path: client doesn't set it)
		requestIdBytes := ctx.Request.Header.Peek(common.RequestIDHeader)
		if len(requestIdBytes) == 0 {
			requestIdBytes = ctx.Request.Header.Peek("X-Request-ID")
		}
		if len(requestIdBytes) > 0 {
			// Client provided ID — just propagate to response
			ctx.Response.Header.SetBytesV(common.RequestIDHeader, requestIdBytes)
			next(ctx)
			return
		}
		// No client ID — generate one using zero-alloc byte path.
		// For production, swap to helper.GenerateFastID() for cryptographic security.
		idBuf := helper.GenerateFastIDCounterBytes()
		ctx.Response.Header.SetBytesV(common.RequestIDHeader, *idBuf)
		helper.PutFastIDCounterBytes(idBuf)
		next(ctx)
	}
}

// fastPanicHandler recovers from panics in fasthttp handlers.
func fastPanicHandler(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		defer func() {
			if rvr := recover(); rvr != nil {
				ctx.SetStatusCode(http.StatusInternalServerError)
				err := InternalServerError("server error", "SERVER_ERROR")
				b, _ := json.Marshal(err)
				ctx.SetContentType("application/json")
				ctx.SetBody(b)
			}
		}()
		next(ctx)
	}
}

// fastRequestLogger logs incoming/outgoing requests when zerolog is enabled.
// When zerolog is Disabled (benchmarks), it's a passthrough — zero overhead.
func fastRequestLogger(next fasthttp.RequestHandler, skipPaths map[string]struct{}) fasthttp.RequestHandler {
	if zerolog.GlobalLevel() == zerolog.Disabled {
		// Zero overhead when logging is disabled
		return next
	}

	return func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.URI().Path())
		if path == "/ping" {
			next(ctx)
			return
		}
		if _, skip := skipPaths[path]; skip {
			next(ctx)
			return
		}

		start := time.Now()
		log := plog.Get()

		requestId := string(ctx.Request.Header.Peek(common.RequestIDHeader))
		if requestId == "" {
			requestId = string(ctx.Request.Header.Peek("X-Request-ID"))
		}
		ctx.Response.Header.Set(common.RequestIDHeader, requestId)

		log.UpdateContext(func(c zerolog.Context) zerolog.Context {
			return c.Str("request_id", requestId)
		})

		log.Info().
			Str("method", string(ctx.Method())).
			Str("url", string(ctx.URI().RequestURI())).
			Str("user_agent", string(ctx.UserAgent())).
			Msg("Incoming Request")

		next(ctx)

		log.Info().
			Str("method", string(ctx.Method())).
			Str("url", string(ctx.URI().RequestURI())).
			Str("user_agent", string(ctx.UserAgent())).
			Int("status_code", ctx.Response.StatusCode()).
			Dur("elapsed_ms", time.Since(start)).
			Msg("Request Finished")
	}
}

// requestValidator validates a struct using go-playground/validator.
func (app *FastHttpApp) requestValidator(i interface{}) error {
	errorResponse := ValidateStruct(i)
	if errorResponse != nil {
		return NewErr(
			WithErrorCode("VALIDATION_ERROR"),
			WithErrorMessage("validation error"),
			WithErrorStatus(http.StatusUnprocessableEntity),
			WithErrorData(errorResponse),
		)
	}
	return nil
}

// fastGenerateUniqueRequestKey creates a unique request key for singleflight.
// Currently unused but kept for future singleflight support in fasthttp path.
//
//nolint:unused
func fastGenerateUniqueRequestKey(ctx *fasthttp.RequestCtx) string {
	method := string(ctx.Method())
	path := string(ctx.URI().Path())
	clientIP := string(ctx.Request.Header.Peek("X-Forwarded-For"))
	if clientIP == "" {
		clientIP = ctx.RemoteIP().String()
	}

	query := ctx.QueryArgs()
	var queryParams []string
	query.VisitAll(func(k, v []byte) {
		queryParams = append(queryParams, fmt.Sprintf("%s=%s", k, v))
	})
	if queryParams == nil {
		queryParams = []string{""}
		sort.Strings(queryParams)
	}

	return fmt.Sprintf("%s|%s|%s|%s", clientIP, method, path, strings.Join(queryParams, "&"))
}
