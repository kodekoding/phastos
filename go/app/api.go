package app

import (
	contextpkg "context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/schema"
	"github.com/kodekoding/phastos/go/server"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/unrolled/secure"
)

var decoder = schema.NewDecoder()

type (
	API struct {
		Http *chi.Mux
		*server.Config
		TotalEndpoints int
		apiTimeout     int
	}

	Options func(api *API)
)

func NewAPI(opts ...Options) *API {
	apiApp := API{
		TotalEndpoints: 0,
	}

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
	return func(app *API) {
		app.Port = port
	}
}

func ReadTimeout(readTimeout int) Options {
	return func(app *API) {
		app.ReadTimeout = readTimeout
	}
}

func WriteTimeout(writeTimeout int) Options {
	return func(app *API) {
		app.WriteTimeout = writeTimeout
	}
}

func WithAPITimeout(apiTimeout int) Options {
	return func(app *API) {
		app.apiTimeout = apiTimeout
	}
}

func (app *API) Init() {
	app.Http = chi.NewRouter()
	app.initPlugins()
}

func (app *API) initPlugins() {
	app.Http.Use(middleware.CleanPath)
	app.Http.Use(middleware.Logger)

	app.Http.Use(PanicHandler)
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

func (app *API) requestValidator(i interface{}) error {
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

func (app *API) wrapHandler(h Handler) http.HandlerFunc {
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

		respChan := make(chan *Response)
		go func() {
			respChan <- h(request, ctx)
		}()

		select {
		case <-ctx.Done():
			if ctx.Err() == contextpkg.DeadlineExceeded {
				w.WriteHeader(http.StatusGatewayTimeout)
				_, err = w.Write([]byte("timeout"))
				if err != nil {

					log.Log().Err(errors.New("context deadline exceed: " + err.Error()))
				}
			}
		case responseFunc := <-respChan:
			if responseFunc.Err != nil {
				if httpError, ok := responseFunc.Err.(*HttpError); ok {
					httpError.Write(w)
					return
				}
				unknownError := NewErr(WithMessage(err.Error()))
				unknownError.Write(w)
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		WriteJson(w, response)
	}
}

func (app *API) AddController(ctrl Controller) {
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

func (app *API) Start() error {
	app.Handler = InitHandler(app.Http)
	secureMiddleware := secure.New(secure.Options{
		BrowserXssFilter:   true,
		ContentTypeNosniff: true,
	})
	app.Handler = secureMiddleware.Handler(app.Handler)

	log.Info().Msg(fmt.Sprintf("server started on port %d, serving %d endpoint(s)", app.Port, app.TotalEndpoints))
	return server.ServeHTTP(app.Config)
}
