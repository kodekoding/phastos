package api

import (
	"context"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	plog "github.com/kodekoding/phastos/v2/go/log"
)

type Map map[string]interface{}

type StringMap map[string]string

type Request struct {
	GetParams  func(key string, defaultValue ...string) string
	GetFile    func(key string) (multipart.File, *multipart.FileHeader, error)
	GetQuery   func(interface{}) error
	GetBody    func(interface{}) error
	GetHeaders func(interface{}) error
}

type Handler func(Request, context.Context) *Response

type Route struct {
	Method      string
	Path        string
	Handler     Handler
	Version     int
	Middlewares *[]func(http.Handler) http.Handler
}

type RouteOption func(*Route)

func NewRoute(method string, handler Handler, opts ...RouteOption) Route {
	route := Route{
		Method:  method,
		Path:    "",
		Handler: handler,
		Version: 1,
	}
	for _, opt := range opts {
		opt(&route)
	}
	return route
}

func WithPath(path string) RouteOption {
	return func(r *Route) {
		r.Path = path
	}
}

func WithVersion(version int) RouteOption {
	return func(r *Route) {
		r.Version = version
	}
}

func WithMiddleware(handlers ...func(http.Handler) http.Handler) RouteOption {
	return func(r *Route) {
		r.Middlewares = &handlers
	}
}

func (r *Route) GetVersionedPath(controllerPath string) string {
	versionPrefix := "/v" + strconv.Itoa(r.Version)
	return versionPrefix + controllerPath + r.Path
}

type ControllerConfig struct {
	Path        string
	Routes      []Route
	Middlewares *[]func(http.Handler) http.Handler
}

type Controller interface {
	GetConfig() ControllerConfig
}

type Controllers interface {
	Register() []Controller
}

func (app *App) initRequest(r *http.Request) *Request {
	log := plog.Ctx(r.Context())
	return &Request{
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
				if err := getBodyFromJSON(r, i); err != nil {
					return BadRequest(err.Error(), ErrParsedBodyCode)
				}
			case ContentURLEncoded:
				if err := parseFormRequest(r); err != nil {
					return BadRequest(err.Error(), ErrParsedBodyCode)
				}
				if err := doHandleDecodeSchema(r, i); err != nil {
					return BadRequest(err.Error(), ErrDecodeBodyCode)
				}
			case ContentFormData:
				if err := parseMultiPartFormRequest(r, 32<<20); err != nil {
					return BadRequest(err.Error(), ErrParsedBodyCode)
				}
				if err := doHandleDecodeSchema(r, i); err != nil {
					return BadRequest(err.Error(), ErrDecodeBodyCode)
				}
			default:
				log.Warn().Msg("Content-Type Header didn't sent, please defined it, will treat as JSON body payload")
				if err := getBodyFromJSON(r, i); err != nil {
					return BadRequest(err.Error(), ErrParsedBodyCode)
				}
			}
			return app.requestValidator(i)
		},
	}
}

type ControllerImpl struct {
}

func (ctrl *ControllerImpl) JoinMiddleware(handlers ...func(http.Handler) http.Handler) *[]func(http.Handler) http.Handler {
	return &handlers
}
