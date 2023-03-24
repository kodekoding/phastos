package app

import (
	"encoding/json"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/schema"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net/http"
	"os"
	"strconv"
)

var decoder = schema.NewDecoder()

type (
	API struct {
		Http           *chi.Mux
		Port           string
		TotalEndpoints int
	}

	Options func(api *API)
)

func NewAPI(opts ...Options) *API {
	apiApp := API{
		Port:           "8000",
		TotalEndpoints: 0,
	}

	for _, opt := range opts {
		opt(&apiApp)
	}
	return &apiApp
}

func WithAppPort(port string) Options {
	return func(app *API) {
		app.Port = port
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
		context := r.Context()

		if response, err = h(request, context); err != nil {
			if httpError, ok := err.(*HttpError); ok {
				httpError.Write(w)
				return
			}
			unknownError := NewErr(WithMessage(err.Error()))
			unknownError.Write(w)
			return
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
	log.Info().Msg("server started on port " + app.Port + ", serving " + strconv.Itoa(app.TotalEndpoints) + " endpoint(s)")
	return http.ListenAndServe(":"+app.Port, app.Http)
}
