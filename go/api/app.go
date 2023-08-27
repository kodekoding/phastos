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
	}

	Options func(api *App)

	Wrapper interface {
		Handler(handler http.Handler) http.Handler
	}
)

func NewApp(opts ...Options) *App {
	apiApp := App{
		TotalEndpoints: 0,
	}

	apiApp.Config = new(server.Config)

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

func (app *App) Init() {
	app.Http = chi.NewRouter()
	app.initPlugins()
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
			WithCode("VALIDATION_ERROR"),
			WithMessage("validation error"),
			WithStatus(http.StatusUnprocessableEntity),
			WithData(errorResponse),
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
					respErr = NewErr(WithMessage(response.Err.Error()))
					response.SetHTTPError(respErr)
				}
				respErr.TraceId = traceId
				go func() {
					log.Error().Msg(fmt.Sprintf("%s - %s (%s)", response.InternalError.Message, response.InternalError.Code, traceId))
				}()
				go response.SentNotif(ctx, response.InternalError, r, traceId)
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

func (app *App) Start() error {
	app.Handler = InitHandler(app.Http)
	secureMiddleware := secure.New(secure.Options{
		BrowserXssFilter:   true,
		ContentTypeNosniff: true,
	})
	app.Handler = secureMiddleware.Handler(app.Handler)

	for _, wrapper := range app.wrapper {
		app.Handler = wrapper.Handler(app.Handler)
	}

	log.Info().Msg(fmt.Sprintf("server started on port %d, serving %d endpoint(s)", app.Port, app.TotalEndpoints))
	return server.ServeHTTP(app.Config)
}
