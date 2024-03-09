package api

import (
	contextpkg "context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/schema"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/unrolled/secure"
	"golang.org/x/net/context"

	"github.com/kodekoding/phastos/v2/go/cache"
	"github.com/kodekoding/phastos/v2/go/common"
	"github.com/kodekoding/phastos/v2/go/cron"
	"github.com/kodekoding/phastos/v2/go/database"
	"github.com/kodekoding/phastos/v2/go/helper"
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
		cache          cache.Caches
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
	apiApp.Config.Ctx = contextpkg.Background()
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

func (app *App) Cache() cache.Caches {
	return app.cache
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

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
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
			GetHeaders: func(i interface{}) error {
				if err := decoder.Decode(i, r.Header); err != nil {
					return BadRequest(err.Error(), "ERROR_PARSING_HEADER")
				}
				return app.requestValidator(i)
			},
			GetBody: func(i interface{}) error {
				if err := json.NewDecoder(r.Body).Decode(i); err != nil {
					return BadRequest(err.Error(), "ERROR_PARSING_BODY")
				}
				return app.requestValidator(i)
			},
		}
		ctx, cancel := contextpkg.WithTimeout(r.Context(), time.Second*time.Duration(app.apiTimeout))
		defer cancel()
		traceId := helper.GenerateUUIDV4()

		ctx = contextpkg.WithValue(ctx, common.TraceIdKeyContextStr, traceId)
		*r = *r.WithContext(ctx)

		respChan := make(chan *Response)
		go func() {
			defer panicRecover(r, traceId)
			respChan <- h(request, ctx)
		}()

		select {
		case <-ctx.Done():
			if ctx.Err() == contextpkg.DeadlineExceeded {
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
				respErr.TraceId = traceId
				go func() {
					// sent error to notification + logs asynchronously
					response.SentNotif(ctx, response.InternalError, r, traceId)
					log.Error().Msg(fmt.Sprintf("%s - %s (%s)", response.InternalError.Message, response.InternalError.Code, traceId))
				}()
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

		handler := chi.
			Chain(middlewares...).
			HandlerFunc(app.wrapHandler(route.Handler))

		app.Http.Method(route.Method, route.GetVersionedPath(config.Path), handler)
	}
	app.TotalEndpoints += len(config.Routes)
}

func (app *App) WrapToContext(wrapper Wrapper) {
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
