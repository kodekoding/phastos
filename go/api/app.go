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
	"github.com/kodekoding/phastos/v2/go/cron"
	"github.com/kodekoding/phastos/v2/go/database"
	plog "github.com/kodekoding/phastos/v2/go/log"
	"github.com/kodekoding/phastos/v2/go/monitoring"
	"github.com/kodekoding/phastos/v2/go/server"
)

var decoder = schema.NewDecoder()

type routeRegistryEntry struct {
	Method string
	Path   string
	Doc    *RouteDoc
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
		middlewareDocs     map[string]MiddlewareInfo
		routeRegistry      []routeRegistryEntry
		enableOpenAPI      bool
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

func (app *App) wrapHandler(h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		request := app.initRequest(r)
		ctx := r.Context()
		requestId := r.Header.Get(common.RequestIDHeader)
		if requestId == "" {
			requestId = r.Header.Get("X-Request-ID")
		}

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
			log = plog.Ctx(ctx)

			app.handleResponseError(response, r, requestId, ctx)
			response.Send(w)
			ReleaseResponse(response)
		}
	}
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

		if route.Doc != nil {
			app.routeRegistry = append(app.routeRegistry, routeRegistryEntry{
				Method: route.Method,
				Path:   routePath,
				Doc:    route.Doc,
			})
		}
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
	if len(app.globalMiddlewares) > 0 {
		app.Http.Use(app.globalMiddlewares...)
	}
	// now that all middlewares are registered, it's safe to add default routes
	app.initRoutes()
}

func (app *App) registerHandler(method, path string, handler Handler, middlewares ...func(http.Handler) http.Handler) {
	// flush any pending global middlewares before registering the first route
	app.flushPendingMiddlewares()

	wrapppedHandler := app.wrapHandler(handler)
	if app.newRelic != nil {
		path, wrapppedHandler = newrelic.WrapHandleFunc(app.newRelic, path, wrapppedHandler)
	}
	handlerFunc := chi.
		Chain(middlewares...).
		HandlerFunc(wrapppedHandler)
	app.Http.Method(method, path, handlerFunc)

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
