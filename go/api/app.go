package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/gorilla/schema"
	"github.com/kodekoding/phastos/v2/go/sse"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/pkg/errors"
	"github.com/unrolled/secure"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/singleflight"

	"github.com/kodekoding/phastos/v2/go/common"
	phastosctx "github.com/kodekoding/phastos/v2/go/context"
	"github.com/kodekoding/phastos/v2/go/cron"
	"github.com/kodekoding/phastos/v2/go/database"
	plog "github.com/kodekoding/phastos/v2/go/log"
	"github.com/kodekoding/phastos/v2/go/monitoring"
	"github.com/kodekoding/phastos/v2/go/server"
)

var decoder = schema.NewDecoder()

type routeRegistryEntry struct {
	Method         string
	Path           string
	Doc            *RouteDoc
	PathParamTypes []PathParamType
}

type HandlerV2 func(ctx context.Context) (any, error)

func isHandlerV2(h any) bool {
	return reflect.TypeOf(h).NumIn() == 1
}

var TimezoneLocation *time.Location
var appVersion string
var commitHash string

type (
	Apps interface {
		LoadModules()
		LoadWrapper()
	}

	App struct {
		Http *chi.Mux
		*server.Config
		TotalEndpoints     int
		apiTimeout         int
		wrapper            []Wrapper
		cron               cron.Engines
		db                 database.ISQL
		trx                database.Transactions
		newRelic           *newrelic.Application
		timezoneRegion     string
		pprofEnabled       bool
		sf                 singleflight.Group
		middlewares        map[string]any
		globalMiddlewares  []func(http.Handler) http.Handler
		pendingMiddlewares bool
		sseEvent           *sse.Hub
		useFastHttp        bool                // if true, server runs with fasthttp
		sfActive           bool                // cached SINGLEFLIGHT_ACTIVE env var
		syncMode           bool                // true when apiTimeout==0 && !sfActive → sync handler path
		skipLogPaths       map[string]struct{} // paths that skip requestLogger
		otelTp             *sdktrace.TracerProvider
		otelSvcName        string
		nrProv             monitoring.Provider
		otelProv           monitoring.Provider
		middlewareDocs        map[string]MiddlewareInfo
		globalMiddlewareMetas []MiddlewareInfo
		routeRegistry         []routeRegistryEntry
		enableOpenAPI      bool
		apiRouters         map[string]*chi.Mux
	}

	Options func(api *App)

	Wrapper interface {
		WrapToHandler(handler http.Handler) http.Handler
		WrapToContext(ctx context.Context) context.Context
	}
)

func NewApp(opts ...Options) *App {
	apiApp := App{
		TotalEndpoints: 0,
		middlewares:    make(map[string]any),
		middlewareDocs: make(map[string]MiddlewareInfo),
	}

	apiApp.Config = new(server.Config)
	apiApp.Ctx = context.Background()
	apiApp.Port = 8000
	apiApp.ReadTimeout = 3
	apiApp.WriteTimeout = 3
	apiApp.apiTimeout = 3

	// set default timezone region
	apiApp.timezoneRegion = "Asia/Jakarta"

	// pprof profiling enabled by default
	apiApp.pprofEnabled = true

	apiVersionFromEnv := os.Getenv("APP_VERSION")
	commitHashFromEnv := os.Getenv("COMMIT_HASH")
	if apiVersionFromEnv != "" {
		// override apiVersion value to ENV value
		appVersion = apiVersionFromEnv
	}
	if commitHashFromEnv != "" {
		// override commitHash value to ENV value
		commitHash = commitHashFromEnv
	}
	apiApp.Version = appVersion

	for _, opt := range opts {
		opt(&apiApp)
	}

	var monitoringProviders []monitoring.Provider
	if apiApp.nrProv != nil {
		monitoringProviders = append(monitoringProviders, apiApp.nrProv)
	}
	if apiApp.otelProv != nil {
		monitoringProviders = append(monitoringProviders, apiApp.otelProv)
	}
	monitoring.SetProviders(monitoringProviders...)

	logOpts := []plog.LoggerOption{
		plog.WithNewRelicApp(apiApp.newRelic),
		plog.WithAppPort(apiApp.Port),
		plog.WithAppVersion(appVersion),
		plog.WithOTelLogEndpoint(),
	}
	log := plog.Get(logOpts...)
	var err error
	TimezoneLocation, err = time.LoadLocation(apiApp.timezoneRegion)
	if err != nil {
		log.Fatal().Err(err).Msg("Load TimeZone Failed")
	}

	return &apiApp
}

func WithAppPort(port int) Options {
	return func(app *App) {
		app.Port = port
	}
}

func ReadTimeout(readTimeout int) Options {
	return func(app *App) {
		app.ReadTimeout = readTimeout
	}
}

func WriteTimeout(writeTimeout int) Options {
	return func(app *App) {
		app.WriteTimeout = writeTimeout
	}
}

func WithAPITimeout(apiTimeout int) Options {
	return func(app *App) {
		app.apiTimeout = apiTimeout
	}
}

func WithTimezone(timezone string) Options {
	return func(app *App) {
		app.timezoneRegion = timezone
	}
}

func WithCronJob(timezone ...string) Options {
	return func(app *App) {
		cronTimeZone := "Asia/Jakarta"
		if len(timezone) > 0 {
			cronTimeZone = timezone[0]
		}
		cronOpts := cron.WithTimeZone(cronTimeZone)
		app.cron = cron.New(cronOpts)
	}
}

func WithPprof(enabled bool) Options {
	return func(app *App) {
		app.pprofEnabled = enabled
	}
}

func WithSSE() Options {
	return func(app *App) {
		app.sseEvent = sse.NewHub(context.Background())
	}
}

func WithFastHttp() Options {
	return func(app *App) {
		app.useFastHttp = true
	}
}

// WithSkipLogPaths configures paths that skip the requestLogger middleware.
// The /ping path is always skipped. Add additional paths (e.g. /health, /metrics)
// to avoid per-request logging overhead for lightweight endpoints.
func WithSkipLogPaths(paths ...string) Options {
	return func(app *App) {
		if app.skipLogPaths == nil {
			app.skipLogPaths = make(map[string]struct{}, len(paths))
		}
		for _, p := range paths {
			app.skipLogPaths[p] = struct{}{}
		}
	}
}

func WithOpenAPI() Options {
	return func(app *App) {
		app.enableOpenAPI = true
	}
}

// WithGlobalMiddleware registers middleware(s) that will be applied to ALL endpoints
// (both authenticated and unauthenticated). These run after the built-in middlewares
// (request logger, recoverer, panic handler) but before any route-specific middleware.
// Use this in NewApp() for middlewares that have no external dependencies.
func WithGlobalMiddleware(handlers ...func(http.Handler) http.Handler) Options {
	return func(app *App) {
		app.globalMiddlewares = append(app.globalMiddlewares, handlers...)
	}
}

// AddGlobalMiddleware appends middleware(s) to be applied to all endpoints.
// Use this for middlewares that depend on resources initialized after NewApp()
// (e.g. repository-dependent middlewares wired in loadModules).
// The middlewares are stored internally and applied lazily when the first route
// is registered, avoiding the chi restriction that middlewares must be defined
// before routes.
func (app *App) AddGlobalMiddleware(handlers ...func(http.Handler) http.Handler) {
	app.globalMiddlewares = append(app.globalMiddlewares, handlers...)
	app.pendingMiddlewares = true
}

// AddGlobalMiddlewareMeta registers metadata (headers/security) for global
// middlewares that are applied to ALL API routes. The metadata is injected
// into every route's OpenAPI doc so required headers and security schemes
// appear in Swagger UI.
func (app *App) AddGlobalMiddlewareMeta(opts ...MiddlewareOption) {
	info := MiddlewareInfo{}
	for _, opt := range opts {
		opt(&info)
	}
	if len(info.Headers) > 0 || info.SecurityScheme != nil || info.Description != "" {
		app.globalMiddlewareMetas = append(app.globalMiddlewareMetas, info)
	}
}

func WithNewRelic() Options {
	return func(app *App) {
		newRelicPlatform, nrProv := monitoring.InitNewRelicOnly()
		if newRelicPlatform == nil {
			return
		}
		app.newRelic = newRelicPlatform.GetApp()
		app.nrProv = nrProv
		app.WrapToApp(&newRelicHandlerWrapper{app: app.newRelic})
	}
}

func WithOTel() Options {
	return func(app *App) {
		serviceName := os.Getenv("OTEL_SERVICE_NAME")
		if serviceName == "" {
			serviceName = os.Getenv("APP_NAME")
		}
		cfg := monitoring.OTelConfig{
			ServiceName:    serviceName,
			ServiceVersion: os.Getenv("APP_VERSION"),
			Environment:    os.Getenv("APP_ENV"),
		}
		tp, otelProv, err := monitoring.InitOTelOnly(context.Background(), cfg)
		if err != nil {
			log := plog.Get()
			log.Fatal().Err(err).Msg("Failed to initialize OpenTelemetry SDK")
			return
		}
		app.otelTp = tp
		app.otelProv = otelProv
		app.otelSvcName = serviceName
		app.WrapToApp(&otelHandlerWrapper{serviceName: serviceName})
	}
}

type otelHandlerWrapper struct {
	serviceName string
}

func (w *otelHandlerWrapper) WrapToHandler(handler http.Handler) http.Handler {
	return monitoring.OTelHTTPMiddleware(w.serviceName)(handler)
}

func (w *otelHandlerWrapper) WrapToContext(ctx context.Context) context.Context {
	return ctx
}

type newRelicHandlerWrapper struct {
	app *newrelic.Application
}

func (w *newRelicHandlerWrapper) WrapToHandler(handler http.Handler) http.Handler {
	return monitoring.NewRelicHTTPMiddleware(w.app)(handler)
}

func (w *newRelicHandlerWrapper) WrapToContext(ctx context.Context) context.Context {
	return ctx
}

func (app *App) Init() {
	app.Http = chi.NewRouter()

	// Cache SINGLEFLIGHT_ACTIVE env var at startup instead of per-request.
	if val := os.Getenv("SINGLEFLIGHT_ACTIVE"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			app.sfActive = parsed
		}
	}

	// Enable sync handler path when no timeout is needed and singleflight is off.
	// This avoids goroutine + channel + context.WithTimeout per request.
	app.syncMode = app.apiTimeout == 0 && !app.sfActive

	// Pass skip paths to requestLogger middleware closure.
	setSkipLogPaths(app.skipLogPaths)

	app.initPlugins()

	// Mark that default routes are pending — they will be registered
	// in flushPendingMiddlewares() before the first user-defined route,
	// allowing AddGlobalMiddleware() to be called after Init().
	app.pendingMiddlewares = true

	// load Notifications if env config is exists
	app.loadNotification()
	app.loadResources()
}

func (app *App) DB() database.ISQL {
	return app.db
}

func (app *App) SSE() sse.Events {
	if app.sseEvent == nil {
		log := plog.Get()
		log.Fatal().Msg("SSE Event not initialized")
	}
	return app.sseEvent
}

func (app *App) SetVersion(version string) {
	app.Version = version
	appVersion = version
}

func (app *App) getAppName() string {
	if name := os.Getenv("APP_NAME"); name != "" {
		return name
	}
	return "Phastos"
}

func (app *App) Trx() database.Transactions {
	return app.trx
}

func (app *App) initPlugins() {
	app.Http.Use(
		requestLogger,
		middleware.Recoverer,
		PanicHandler,
	)

	// apply global middlewares registered via WithGlobalMiddleware option
	if len(app.globalMiddlewares) > 0 {
		app.Http.Use(app.globalMiddlewares...)
		// clear them so they won't be re-applied in flushPendingMiddlewares
		app.globalMiddlewares = nil
	}
}

// initRoutes registers default routes (NotFound, MethodNotAllowed, pprof, /ping, etc).
// It is called lazily before the first user-defined route to ensure all global
// middlewares (including those added via AddGlobalMiddleware) are applied first.
func (app *App) initRoutes() {
	app.Http.NotFound(RouteNotFoundHandler)
	app.Http.MethodNotAllowed(MethodNotAllowedHandler)

	app.initDefaultHandlers()

	// pprof profiling: env var overrides option
	pprofEnabled := app.pprofEnabled
	if envVal := os.Getenv("PPROF_ENABLED"); envVal != "" {
		if parsed, err := strconv.ParseBool(envVal); err == nil {
			pprofEnabled = parsed
		}
	}
	if pprofEnabled {
		app.Http.Mount("/debug", middleware.Profiler())
		pprofLog := plog.Get()
		pprofLog.Info().Msg("[PHASTOS] pprof profiling enabled at /debug/pprof/")
	}
}

func (app *App) initDefaultHandlers() {

	// register ping endpoint for the health checks
	app.registerHandler("GET", "/ping", func(request Request, ctx context.Context) *Response {
		msgString := "pong"
		msgString = fmt.Sprintf("%s on datetime: %s", msgString, GetDateTimeNowStringWithTimezone())

		return NewResponse().SetMessage(msgString)
	})

	if app.sseEvent != nil {
		app.Http.Get("/events", app.sseEvent.Handle)
		app.TotalEndpoints++
		app.registerHandler("GET", "/events/missed-msg", func(request Request, ctx context.Context) *Response {
			var req struct {
				ClientID       string "schema:\"client_id\" validate:\"required\""
				LastReceivedID string "schema:\"last_received_id\" validate:\"required\""
			}
			if err := request.GetQuery(&req); err != nil {
				return NewResponse().SetError(err)
			}

			// Retrieve missed messages
			missedMessages := app.sseEvent.GetMissedMessages(req.ClientID, req.LastReceivedID)
			data := Map{
				"client_id": req.ClientID,
				"messages":  missedMessages,
				"count":     len(missedMessages),
			}

			return NewResponse().SetData(data)
		})
	}
}

func (app *App) requestValidator(i interface{}) error {
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

type handler2WithMeta struct {
	h              HandlerV2
	requestType    any
	queryType      any
	pathParamTypes []PathParamType
}

func (app *App) wrapHandler(handler any) http.HandlerFunc {
	if h2m, ok := handler.(handler2WithMeta); ok {
		return app.wrapHandlerV2WithMeta(h2m)
	}
	if h2, ok := handler.(HandlerV2); ok {
		return app.wrapHandlerV2(h2)
	}
	if isHandlerV2(handler) {
		return app.wrapHandlerV2(handler.(func(context.Context) (any, error))) //nolint:errcheck
	}

	h := handler.(func(Request, context.Context) *Response) //nolint:errcheck
	return func(w http.ResponseWriter, r *http.Request) {
		request := app.initRequest(r)
		ctx := r.Context()
		requestId := r.Header.Get(common.RequestIDHeader)
		if requestId == "" {
			requestId = r.Header.Get("X-Request-ID")
		}
		w.Header().Set("X-Trace-ID", requestId)

		// --- Sync path: no timeout, no singleflight ---
		// Executes the handler directly on the current goroutine,
		// avoiding channel + goroutine + context.WithTimeout overhead.
		// Panic recovery is still provided via defer panicRecover().
		if app.syncMode {
			var uniqueReqKey string
			defer panicRecover(r, requestId, uniqueReqKey)
			response := h(*request, ctx)
			ReleaseRequest(request)
			response.TraceId = requestId
			response.SetCustomHeader("X-Trace-ID", requestId)
			app.handleResponseError(response, r, requestId, ctx)
			response.Send(w)
			ReleaseResponse(response)
			return
		}

		// --- Async path: timeout + optional singleflight ---
		var response *Response
		var err error
		log := plog.Ctx(ctx)

		timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(app.apiTimeout))
		defer cancel()

		respChan := make(chan *Response)
		go func() {
			// close the channel after finished the process
			defer close(respChan)
			defer ReleaseRequest(request)
			var uniqueReqKey string
			defer panicRecover(r, requestId, uniqueReqKey)

			// Use cached sfActive instead of per-request env var parsing.
			if !app.sfActive {
				respChan <- h(*request, ctx)
				return
			}

			uniqueReqKey = generateUniqueRequestKey(r)

			sfResponse, sfErr, _ := app.sf.Do(uniqueReqKey, func() (interface{}, error) {
				handlerResp := h(*request, ctx)
				return handlerResp, nil
			})
			if sfErr != nil {
				log.Err(sfErr).Msg("[SINGLEFLIGHT] - Error when do singleFlight request")
				respChan <- NewResponse().SetError(sfErr)
				return
			}
			respChan <- sfResponse.(*Response) //nolint:errcheck
		}()

		select {
		case <-timeoutCtx.Done():
			if errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
				w.WriteHeader(http.StatusGatewayTimeout)
				_, err = w.Write([]byte("timeout"))
				if err != nil {
					log.Err(err).Msg("[REQUEST][TIMEOUT] Failed when write to client")
				} else {
					log.Info().Msg("[REQUEST][TIMEOUT] context deadline exceed")
				}

			}
		case response = <-respChan:
			response.TraceId = requestId
			response.SetCustomHeader("X-Trace-ID", requestId)
			log = plog.Ctx(ctx)

			app.handleResponseError(response, r, requestId, ctx)
			response.Send(w)
			ReleaseResponse(response)
		}
	}
}

// wrapHandlerV2 wraps a HandlerV2 for use as an http.HandlerFunc.
// It performs auto-binding (path → query → body → validate) before
// calling the handler, and wraps the (any, error) return into *Response.
func (app *App) wrapHandlerV2(h HandlerV2) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestId := r.Header.Get(common.RequestIDHeader)
		if requestId == "" {
			requestId = r.Header.Get("X-Request-ID")
		}
		w.Header().Set("X-Trace-ID", requestId)

		result, err := h(r.Context())

		var response *Response
		switch {
		case err != nil:
			response = NewResponse().SetError(err)
		case result == nil:
			response = NewResponse()
		default:
			if customResp, ok := result.(*Response); ok {
				response = customResp
			} else {
				response = NewResponse().SetData(result)
			}
		}

		app.handleResponseError(response, r, requestId, r.Context())
		response.Send(w)
		ReleaseResponse(response)
	}
}

// wrapHandlerV2WithMeta wraps a HandlerV2 with full auto-binding (path → query → body → validate).
func (app *App) wrapHandlerV2WithMeta(m handler2WithMeta) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestId := r.Header.Get(common.RequestIDHeader)
		if requestId == "" {
			requestId = r.Header.Get("X-Request-ID")
		}
		w.Header().Set("X-Trace-ID", requestId)
		ctx := r.Context()

		// 1. Extract and validate path params
		if len(m.pathParamTypes) > 0 {
			rctx := chi.RouteContext(ctx)
			params := map[string]string{}
			pi := 0
			if rctx != nil {
				for i, key := range rctx.URLParams.Keys {
					if strings.HasPrefix(key, "*") {
						continue
					}
					val := rctx.URLParams.Values[i]
					params[key] = val
					if pi < len(m.pathParamTypes) {
						if err := validatePathParam(val, m.pathParamTypes[pi]); err != nil {
							resp := NewResponse().SetError(err)
							app.handleResponseError(resp, r, requestId, ctx)
							resp.Send(w)
							ReleaseResponse(resp)
							return
						}
						pi++
					}
				}
			}
			ctx = phastosctx.SetPathParams(ctx, params)
		}

		// 2. Bind query params (for all methods if QueryType or RequestType is set)
		queryTarget := m.queryType
		if queryTarget == nil && r.Method == http.MethodGet {
			queryTarget = m.requestType
		}
		if queryTarget != nil {
			reqType := reflect.TypeOf(queryTarget)
			if reqType.Kind() == reflect.Ptr {
				reqType = reqType.Elem()
			}
			queryVal := reflect.New(reqType).Interface()
			if err := decoder.Decode(queryVal, r.URL.Query()); err != nil {
				resp := NewResponse().SetError(BadRequest(err.Error(), "ERROR_PARSING_QUERY_PARAMS"))
				app.handleResponseError(resp, r, requestId, ctx)
				resp.Send(w)
				ReleaseResponse(resp)
				return
			}
			if err := app.requestValidator(queryVal); err != nil {
				resp := NewResponse().SetError(err)
				app.handleResponseError(resp, r, requestId, ctx)
				resp.Send(w)
				ReleaseResponse(resp)
				return
			}
			ctx = phastosctx.SetQueryParams(ctx, queryVal)
		}

		// 3. Bind body (for POST/PUT/PATCH)
		if m.requestType != nil && (r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch) {
			bodyType := reflect.TypeOf(m.requestType)
			if bodyType.Kind() == reflect.Ptr {
				bodyType = bodyType.Elem()
			}
			bodyVal := reflect.New(bodyType).Interface()
			if err := json.NewDecoder(r.Body).Decode(bodyVal); err != nil {
				resp := NewResponse().SetError(BadRequest(err.Error(), "ERROR_PARSING_BODY"))
				app.handleResponseError(resp, r, requestId, ctx)
				resp.Send(w)
				ReleaseResponse(resp)
				return
			}
			if err := app.requestValidator(bodyVal); err != nil {
				resp := NewResponse().SetError(err)
				app.handleResponseError(resp, r, requestId, ctx)
				resp.Send(w)
				ReleaseResponse(resp)
				return
			}
			ctx = phastosctx.SetRequestBody(ctx, bodyVal)
		}

		// 4. Call handler with enriched context
		result, err := m.h(ctx)

		var response *Response
		switch {
		case err != nil:
			response = NewResponse().SetError(err)
		case result == nil:
			response = NewResponse()
		default:
			if customResp, ok := result.(*Response); ok {
				response = customResp
			} else {
				response = NewResponse().SetData(result)
			}
		}

		app.handleResponseError(response, r, requestId, r.Context())
		response.Send(w)
		ReleaseResponse(response)
	}
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

// handleResponseError processes error responses: sets HTTPError, sends notifications,
// and logs asynchronously. Shared by both sync and async handler paths.
func (app *App) handleResponseError(response *Response, r *http.Request, requestId string, ctx context.Context) {
	if response.Err == nil {
		return
	}
	var respErr *HttpError
	var ok bool
	if ok = errors.As(errors.Cause(response.Err), &respErr); !ok {
		respErr = NewErr(WithErrorMessage(response.Err.Error()), WithTraceId(requestId))
		response.SetHTTPError(respErr)
	}

	// Snapshot values needed by the async goroutine before launching it.
	// This prevents a data race: the goroutine would otherwise read
	// response.InternalError and r.URL while the caller may proceed to
	// ReleaseResponse (which resets those fields) or the runtime may
	// recycle the request object.
	snapshotInternalError := response.InternalError
	snapshotURL := r.URL.String()

	log := plog.Ctx(ctx)
	var asyncTrx *newrelic.Transaction
	if app.newRelic != nil {
		asyncTrx = monitoring.BeginTrxFromContext(ctx).NewGoroutine()
	}
	var asyncSpan trace.Span
	if app.otelTp != nil {
		_, asyncSpan = otel.Tracer("phastos.api").Start(ctx, "PhastosAPIApp-WrapHandler-AsyncSentNotifAndLogError",
			trace.WithAttributes(
				attribute.String("request_id", requestId),
				attribute.String("request_path", snapshotURL),
			),
		)
	}
	go func(asyncTxn *newrelic.Transaction, oSpan trace.Span) {
		asyncCtx := ctx
		if asyncTxn != nil {
			asyncCtx = monitoring.NewContext(ctx, asyncTxn)
			defer asyncTxn.StartSegment("PhastosAPIApp-WrapHandler-AsyncSentNotifAndLogError").End()
		}
		if oSpan != nil {
			defer oSpan.End()
			asyncCtx = trace.ContextWithSpan(asyncCtx, oSpan)
		}
		// sent error to notification + logs asynchronously using snapshots
		response.SentNotif(asyncCtx, snapshotInternalError, r, requestId)
		logEvent := log.Err(snapshotInternalError)
		if snapshotInternalError.Status < 500 {
			// re-assign logEvent from 'error' to 'warn'
			logEvent = log.Warn()
		}

		if snapshotInternalError.Data != nil {
			logEvent.Any("error_data", snapshotInternalError.Data)
		}
		logEvent.
			Str("trace_id", requestId).
			Str("request_path", snapshotURL).
			Msg("Failed processing request")
	}(asyncTrx, asyncSpan)
}

func generateUniqueRequestKey(req *http.Request) string {
	method := req.Method
	path := req.URL.Path
	clientIP := req.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = req.RemoteAddr
	}

	// Sort query parameters to ensure consistent key generation
	query := req.URL.Query()
	var queryParams []string
	for k, v := range query {
		for _, vv := range v {
			queryParams = append(queryParams, fmt.Sprintf("%s=%s", k, vv))
		}
	}
	if queryParams == nil {
		queryParams = []string{""}
		sort.Strings(queryParams)
	}

	return fmt.Sprintf("%s|%s|%s|%s", clientIP, method, path, strings.Join(queryParams, "&"))
}

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

func RegisterMiddleware[T any](app *App, key string, middlewareHandler T) {
	app.middlewares[key] = middlewareHandler
}

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

// registerRoutes recursively registers routes, handling nested groups.
// prefix accumulates path from parent groups; parentMiddlewares accumulates middleware chain.
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

	// Auto-inject middleware metadata into RouteDoc.
	// Controller-specific keys inject security + headers.
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

	// Inject global middleware metadata (headers only) into every route.
	// These are middlewares applied to ALL API routes (e.g., AllowPlatform).
	for _, meta := range app.globalMiddlewareMetas {
		if route.Doc == nil {
			route.Doc = &RouteDoc{}
		}
		route.Doc.Headers = append(route.Doc.Headers, meta.Headers...)
	}

	// Inject security scheme from ALL registered middlewares into every route
	// (e.g., JWTAuth that is applied globally via JoinMiddleware).
	for _, info := range app.middlewareDocs {
		if info.SecurityScheme != nil && route.Doc != nil && route.Doc.Security == nil {
			route.Doc.Security = info.SecurityScheme
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

		// Wrap HandlerV2 with auto-binding metadata from route annotations
		handler := route.Handler
		if isHandlerV2(handler) && route.Doc != nil {
			h2 := handler.(func(context.Context) (any, error)) //nolint:errcheck
			handler = handler2WithMeta{
				h:              h2,
				requestType:    route.Doc.RequestType,
				queryType:      route.Doc.QueryType,
				pathParamTypes: route.PathParamTypes,
			}
		}

		app.registerHandler(route.Method, routePath, handler, middlewares...)

		if route.Doc == nil {
			route.Doc = &RouteDoc{}
		}
		app.routeRegistry = append(app.routeRegistry, routeRegistryEntry{
			Method:         route.Method,
			Path:           routePath,
			Doc:            route.Doc,
			PathParamTypes: route.PathParamTypes,
		})
	}
}

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
	// Ensure default routes (/ping) are registered even if no leaf routes were processed.
	app.flushPendingMiddlewares()
}

func (app *App) AddControllers(ctrls Controllers) {
	controllers := ctrls.Register()
	for _, ctrl := range controllers {
		app.AddController(ctrl)
	}
}

// flushPendingMiddlewares applies any middlewares added via AddGlobalMiddleware
// to the chi router, then registers default routes. This is called lazily before
// the first user-defined route registration to avoid the chi restriction that
// middlewares must be defined before routes.
func (app *App) flushPendingMiddlewares() {
	if !app.pendingMiddlewares {
		return
	}
	app.pendingMiddlewares = false
	app.initRoutes()
}

func (app *App) registerHandler(method, path string, handler any, middlewares ...func(http.Handler) http.Handler) {
	app.flushPendingMiddlewares()

	// Strip type annotations for chi routing (chi expects clean {name} patterns)
	chiPath := stripPathParamTypes(path)

	wrapppedHandler := app.wrapHandler(handler)
	if app.newRelic != nil {
		chiPath, wrapppedHandler = newrelic.WrapHandleFunc(app.newRelic, chiPath, wrapppedHandler)
	}
	handlerFunc := chi.
		Chain(middlewares...).
		HandlerFunc(wrapppedHandler)

	if strings.HasPrefix(chiPath, "/v") {
		parts := strings.SplitN(chiPath[2:], "/", 2)
		if len(parts) > 0 && parts[0] != "" {
			versionPrefix := "/v" + parts[0]
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
			strippedPath := strings.TrimPrefix(chiPath, versionPrefix)
			router.Method(method, strippedPath, handlerFunc)
			app.TotalEndpoints++
			return
		}
	}

	app.Http.Method(method, chiPath, handlerFunc)
	app.TotalEndpoints++
}

func (app *App) WrapToApp(wrapper Wrapper) {
	app.wrapper = append(app.wrapper, wrapper)
}

func (app *App) AddScheduler(pattern string, handler cron.HandlerFunc) {
	app.cron.RegisterScheduler(pattern, handler)
}

func (app *App) WrapScheduler(wrapper cron.Wrapper) {
	app.cron.Wrap(wrapper)
}

func (app *App) Start() error {
	// ensure any pending middlewares and default routes are registered
	app.flushPendingMiddlewares()

	if app.enableOpenAPI {
		spec := app.buildOpenAPISpec()
		olog := plog.Get()

		app.Http.Get("/docs/openapi.json", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(spec); err != nil {
				olog.Err(err).Msg("failed to encode openapi spec")
			}
		})

		app.Http.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if _, err := w.Write([]byte(openapiHTML)); err != nil {
				olog.Err(err).Msg("failed to write swagger ui")
			}
		})
	}

	log := plog.Get()
	app.Handler = InitHandler(app.Http)
	secureMiddleware := secure.New(secure.Options{
		BrowserXssFilter:   true,
		ContentTypeNosniff: true,
	})
	app.Handler = secureMiddleware.Handler(app.Handler)

	corsOptions := cors.Options{
		AllowedMethods: []string{"PATCH", "POST", "DELETE", "GET", "PUT", "OPTIONS"},
		AllowedHeaders: []string{"Origin", "Referer", "token", "content-type", "Content-Type", "Authorization"},
		MaxAge:         60 * 60, //1 hour
	}
	corsOriginEnv := os.Getenv("CORS_ORIGIN")
	if corsOriginEnv != "" {
		corsOptions.AllowedOrigins = strings.Split(corsOriginEnv, ",")
	}

	corsAllowedHeader := os.Getenv("CORS_HEADER")
	if corsOriginEnv != "" {
		corsOptions.AllowedHeaders = append(corsOptions.AllowedHeaders, strings.Split(corsAllowedHeader, ",")...)
	}

	corsMiddleware := cors.New(corsOptions)
	app.Handler = corsMiddleware.Handler(app.Handler)

	for _, wrapper := range app.wrapper {
		app.Handler = wrapper.WrapToHandler(app.Handler)
		app.Ctx = wrapper.WrapToContext(app.Ctx)
	}

	log.Info().Int("total_endpoint(s)", app.TotalEndpoints).Msg("Server Starting")

	if app.cron != nil {
		defer app.cron.Stop()
		go app.cron.Start()
	}

	if app.sseEvent != nil {
		defer app.sseEvent.Stop()
		go app.sseEvent.Run()
	}

	if app.otelTp != nil {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = app.otelTp.Shutdown(shutdownCtx)
		}()
	}

	if app.useFastHttp {

		return serveFastHTTPs(app.Config, false)
	}
	return serveHTTPs(app.Config, false)
}
