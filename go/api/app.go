package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/gorilla/schema"
	logWriter "github.com/newrelic/go-agent/v3/integrations/logcontext-v2/zerologWriter"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/unrolled/secure"

	"github.com/kodekoding/phastos/v2/go/common"
	"github.com/kodekoding/phastos/v2/go/cron"
	"github.com/kodekoding/phastos/v2/go/database"
	"github.com/kodekoding/phastos/v2/go/helper"
	"github.com/kodekoding/phastos/v2/go/monitoring"
	"github.com/kodekoding/phastos/v2/go/server"
)

var decoder = schema.NewDecoder()

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

	for _, opt := range opts {
		opt(&apiApp)
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

func WithCronJob(timezone ...string) Options {
	return func(app *App) {
		cronOpts := cron.WithTimeZone("Asia/Jakarta")
		if timezone != nil && len(timezone) > 0 {
			cronOpts = cron.WithTimeZone(timezone[0])
		}
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

func (app *App) Trx() database.Transactions {
	return app.trx
}

func (app *App) initPlugins() {
	app.Http.Use(
		middleware.Logger,
		middleware.Recoverer,
		PanicHandler,
	)
	app.Http.NotFound(RouteNotFoundHandler)
	app.Http.MethodNotAllowed(MethodNotAllowedHandler)

	app.Http.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		WriteJson(w, Map{
			"message": "pong",
		})
	})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	var writer io.Writer
	writer = zerolog.ConsoleWriter{Out: os.Stderr}

	if app.newRelic != nil {
		writer = logWriter.New(os.Stdout, app.newRelic)
	}
	log.Logger = log.Output(writer).With().Str("app", os.Getenv("APP_NAME")).Logger()
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
		requestId := helper.GenerateUUIDV4()
		ctx := context.WithValue(r.Context(), common.TraceIdKeyContextStr, requestId)
		*r = *r.WithContext(ctx)

		var response *Response
		var err error

		request := Request{
			GetParams: func(key string, defaultValue ...string) string {
				var paramValue string
				if paramValue = chi.URLParam(r, key); paramValue == "" {
					for _, v := range defaultValue {
						paramValue = v
					}
				}
				return paramValue
			},
			GetFile: func(key string) (multipart.File, *multipart.FileHeader, error) {
				return r.FormFile(key)
			},
			GetQuery: func(i interface{}) error {
				if err := decoder.Decode(i, r.URL.Query()); err != nil {
					return BadRequest(err.Error(), "ERROR_PARSING_QUERY_PARAMS")
				}
				return app.requestValidator(i)
			},
			// GetHeaders
			GetHeaders: func(i interface{}) error {
				if err := decoder.Decode(i, r.Header); err != nil {
					return BadRequest(err.Error(), "ERROR_PARSING_HEADER")
				}
				return app.requestValidator(i)
			},
			GetBody: func(i interface{}) error {
				contentType := filterFlags(r.Header.Get("Content-Type"))
				switch contentType {
				case ContentJSON:
					if err = json.NewDecoder(r.Body).Decode(i); err != nil {
						return BadRequest(err.Error(), ErrParsedBodyCode)
					}
				case ContentURLEncoded:
					if err = parseFormRequest(r); err != nil {
						return BadRequest(err.Error(), ErrParsedBodyCode)
					}
					if err = doHandleDecodeSchema(r, i); err != nil {
						return BadRequest(err.Error(), ErrDecodeBodyCode)
					}
				case ContentFormData:
					if err = parseMultiPartFormRequest(r, 32<<20); err != nil {
						return BadRequest(err.Error(), ErrParsedBodyCode)
					}
					if err = doHandleDecodeSchema(r, i); err != nil {
						return BadRequest(err.Error(), ErrDecodeBodyCode)
					}
				default:
					log.Warn().Msg("Content-Type Header didn't sent, please defined it, will treat as JSON body payload")
					if err = json.NewDecoder(r.Body).Decode(i); err != nil {
						return BadRequest(err.Error(), ErrParsedBodyCode)
					}
				}
				return app.requestValidator(i)
			},
		}

		timeoutCtx, cancel := context.WithTimeout(r.Context(), time.Second*time.Duration(app.apiTimeout))
		defer cancel()

		respChan := make(chan *Response)
		go func() {
			defer panicRecover(r, requestId)
			respChan <- h(request, r.Context())
		}()

		select {
		case <-timeoutCtx.Done():
			if timeoutCtx.Err() == context.DeadlineExceeded {
				w.WriteHeader(http.StatusGatewayTimeout)
				_, err = w.Write([]byte("timeout"))
				if err != nil {

					log.Error().Msg("context deadline exceed: " + err.Error())
				}
			}
		case response = <-respChan:
			if response.Err != nil {
				var respErr *HttpError
				var ok bool
				if respErr, ok = response.Err.(*HttpError); !ok {
					respErr = NewErr(WithErrorMessage(response.Err.Error()))
					response.SetHTTPError(respErr)
				}
				respErr.TraceId = requestId
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
					logEvent := log.Error()
					if response.InternalError.Status < 500 {
						// re-assign logEvent from 'error' to 'warn'
						logEvent = log.Warn()
					}
					errData := map[string]interface{}{
						"error":        response.InternalError,
						"request_path": r.URL.String(),
						"trace_id":     requestId,
					}
					logEvent.Fields(errData).Msg("Failed processing request")
				}(asyncTrx)
			}
			response.Send(w)
		}
	}
}

func (app *App) AddController(ctrl Controller) {
	config := ctrl.GetConfig()
	for _, route := range config.Routes {
		middlewares := []func(http.Handler) http.Handler{}

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

	log.Info().Msg(fmt.Sprintf("server started on port %d, serving %d endpoint(s)", app.Port, app.TotalEndpoints))

	if app.cron != nil {
		defer app.cron.Stop()
		go app.cron.Start()
	}

	return server.ServeHTTP(app.Config)
}
