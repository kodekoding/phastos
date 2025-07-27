package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/gorilla/schema"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/unrolled/secure"
	"golang.org/x/sync/singleflight"

	"github.com/kodekoding/phastos/v2/go/common"
	"github.com/kodekoding/phastos/v2/go/cron"
	"github.com/kodekoding/phastos/v2/go/database"
	plog "github.com/kodekoding/phastos/v2/go/log"
	"github.com/kodekoding/phastos/v2/go/monitoring"
	"github.com/kodekoding/phastos/v2/go/server"
)

var decoder = schema.NewDecoder()
var TimezoneLocation *time.Location
var appVersion string
var once sync.Once

type (
	Apps interface {
		LoadModules()
		LoadWrapper()
	}

	App struct {
		Http *chi.Mux
		*server.Config
		TotalEndpoints int
		apiTimeout     int
		wrapper        []Wrapper
		cron           *cron.Engine
		db             database.ISQL
		trx            database.Transactions
		newRelic       *newrelic.Application
		version        string
		timezoneRegion string
		sf             singleflight.Group
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
	}

	apiApp.Config = new(server.Config)
	apiApp.Config.Ctx = context.Background()
	apiApp.Port = 8000
	apiApp.ReadTimeout = 3
	apiApp.WriteTimeout = 3
	apiApp.apiTimeout = 3

	// set default timezone region
	apiApp.timezoneRegion = "Asia/Jakarta"

	for _, opt := range opts {
		opt(&apiApp)
	}
	log := plog.Get(
		plog.WithNewRelicApp(apiApp.newRelic),
		plog.WithAppPort(apiApp.Port),
		plog.WithAppVersion(appVersion),
	)
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
		if timezone != nil && len(timezone) > 0 {
			cronTimeZone = timezone[0]
		}
		cronOpts := cron.WithTimeZone(cronTimeZone)
		app.cron = cron.New(cronOpts)
	}
}

func WithNewRelic() Options {
	return func(app *App) {
		newRelicPlatform := monitoring.InitNewRelic()
		app.newRelic = newRelicPlatform.GetApp()
	}
}

func (app *App) Init() {
	app.Http = chi.NewRouter()

	app.initPlugins()

	// load Notifications if env config is exists
	app.loadNotification()
	app.loadResources()
}

func (app *App) DB() database.ISQL {
	return app.db
}

func (app *App) SetVersion(version string) {
	app.version = version
	app.Config.Version = version
	appVersion = version
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
	app.Http.NotFound(RouteNotFoundHandler)
	app.Http.MethodNotAllowed(MethodNotAllowedHandler)

	app.Http.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		msgString := "pong"
		if app.version != "" {
			msgString = fmt.Sprintf("%s from version: %s", msgString, app.version)
		}
		msgString = fmt.Sprintf("%s on datetime: %s", msgString, GetDateTimeNowStringWithTimezone())
		w.WriteHeader(http.StatusOK)
		WriteJson(w, Map{
			"message": msgString,
		})
	})

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
		var response *Response
		var err error

		request := app.initRequest(r)
		ctx := r.Context()
		log := plog.Ctx(ctx)

		requestId := ctx.Value(common.RequestIdContextKey).(string)
		timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(app.apiTimeout))
		defer cancel()

		respChan := make(chan *Response)
		go func() {
			// close the channel after finished the process
			defer close(respChan)
			var uniqueReqKey string
			defer panicRecover(r, requestId, uniqueReqKey)

			singleFlightEnvValue := os.Getenv("SINGLEFLIGHT_ACTIVE")
			isSingleFlightActive := false
			if singleFlightEnvValue != "" {
				if isSingleFlightActive, err = strconv.ParseBool(singleFlightEnvValue); err != nil {
					log.Warn().Msg("[REQUEST][WrapperHandler] Failed to parse single flight active flag")
				}
			}
			if !isSingleFlightActive {
				respChan <- h(*request, ctx)
				return
			}

			uniqueReqKey = generateUniqueRequestKey(r)

			sfResponse, err, _ := app.sf.Do(uniqueReqKey, func() (interface{}, error) {
				handlerResp := h(*request, ctx)
				return handlerResp, nil
			})
			if err != nil {
				log.Err(err).Msg("[SINGLEFLIGHT] - Error when do singleFlight request")
				respChan <- NewResponse().SetError(err)
				return
			}
			respChan <- sfResponse.(*Response)
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
			if response.Err != nil {
				var respErr *HttpError
				var ok bool
				if ok = errors.As(response.Err, &respErr); !ok {
					respErr = NewErr(WithErrorMessage(response.Err.Error()), WithTraceId(requestId))
					response.SetHTTPError(respErr)
				}
				var asyncTrx *newrelic.Transaction
				if app.newRelic != nil {
					asyncTrx = monitoring.BeginTrxFromContext(ctx).NewGoroutine()
				}
				go func(asyncTxn *newrelic.Transaction) {
					asyncCtx := ctx
					if asyncTxn != nil {
						asyncCtx = monitoring.NewContext(ctx, asyncTxn)
						defer asyncTxn.StartSegment("PhastosAPIApp-WrapHandler-AsyncSentNotifAndLogError").End()
					}
					// sent error to notification + logs asynchronously
					response.SentNotif(asyncCtx, response.InternalError, r, requestId)
					logEvent := log.Err(response.InternalError)
					if response.InternalError.Status < 500 {
						// re-assign logEvent from 'error' to 'warn'
						logEvent = log.Warn()
					}

					if response.InternalError.Data != nil {
						logEvent.Any("error_data", response.InternalError.Data)
					}
					logEvent.
						Str("trace_id", requestId).
						Str("request_path", r.URL.String()).
						Msg("Failed processing request")
				}(asyncTrx)
			}
			response.Send(w)
		}
	}
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

func (app *App) AddController(ctrl Controller) {
	config := ctrl.GetConfig()
	for _, route := range config.Routes {
		var middlewares []func(http.Handler) http.Handler

		if config.Middlewares != nil {
			middlewares = append(middlewares, *config.Middlewares...)
		}

		if route.Middlewares != nil {
			middlewares = append(middlewares, *route.Middlewares...)
		}

		routePath := route.GetVersionedPath(config.Path)
		handlerFunc := app.wrapHandler(route.Handler)
		if app.newRelic != nil {
			routePath, handlerFunc = newrelic.WrapHandleFunc(app.newRelic, routePath, handlerFunc)
		}
		handler := chi.
			Chain(middlewares...).
			HandlerFunc(handlerFunc)
		app.Http.Method(route.Method, routePath, handler)
	}
	app.TotalEndpoints += len(config.Routes)
}

func (app *App) AddControllers(ctrls Controllers) {
	controllers := ctrls.Register()
	for _, ctrl := range controllers {
		app.AddController(ctrl)
	}
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
		app.Config.Ctx = wrapper.WrapToContext(app.Config.Ctx)
	}

	log.Info().Int("total_endpoint(s)", app.TotalEndpoints).Msg("Server Starting")

	if app.cron != nil {
		defer app.cron.Stop()
		go app.cron.Start()
	}

	return serveHTTPs(app.Config, false)
}
