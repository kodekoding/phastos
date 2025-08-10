package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"golang.org/x/sync/singleflight"

	context2 "github.com/kodekoding/phastos/go/context"
	"github.com/kodekoding/phastos/go/helper"
	"github.com/kodekoding/phastos/go/log"
	"github.com/kodekoding/phastos/go/response"
)

type WrapperFunc func(http.ResponseWriter, *http.Request) *response.JSON

type RouteInterface interface {
	chi.Routes
	GetHandler() *chi.Mux
	InitRoute(corsConfig ...cors.Options)
	Handle(pattern string, handler http.Handler)
	Group(prefix string, fn func(r RouteInterface)) RouteInterface
	Get(pattern string, handler WrapperFunc)
	Post(pattern string, handler WrapperFunc)
	Patch(pattern string, handler WrapperFunc)
	Delete(pattern string, handler WrapperFunc)
	Put(pattern string, handler WrapperFunc)
	Use(middlewares ...func(http.Handler) http.Handler)
	With(middlewares ...func(http.Handler) http.Handler) RouteInterface
	Mount(pattern string, handler http.Handler)
	StaticFS(pattern, imgDir string)
}

type ChiRouter struct {
	handle     *chi.Mux
	timeout    int
	sf         singleflight.Group
	isSfActive bool
}

func NewChiRouter(timeout ...int) *ChiRouter {
	// default timeout if timeout param is nil (3 seconds)
	ctxTimeout := 30 * 6
	if timeout != nil && len(timeout) == 1 {
		ctxTimeout = timeout[0]
	}

	cr := &ChiRouter{handle: chi.NewRouter(), timeout: ctxTimeout}

	isSfActiveFromEnv := os.Getenv("SINGLEFLIGHT_ACTIVE")
	if isSfActiveFromEnv != "" {
		value, err := strconv.ParseBool(isSfActiveFromEnv)
		if err == nil {
			cr.isSfActive = value
		}
	}

	return cr
}

func (cr *ChiRouter) GetHandler() *chi.Mux {
	return cr.handle
}

func (cr *ChiRouter) Find(rctx *chi.Context, method, path string) string {
	return cr.Find(rctx, method, path)
}

func (cr *ChiRouter) InitRoute(corsConfig ...cors.Options) {
	cr.Use(
		middleware.Logger,
		middleware.Recoverer,
	)

	if corsConfig != nil && len(corsConfig) > 0 {
		cr.Use(cors.Handler(corsConfig[0]))
	}

	cr.Get("/ping", func(_ http.ResponseWriter, _ *http.Request) *response.JSON {
		return response.NewJSON().Success("PONG")
	})
}

func (cr *ChiRouter) Group(prefix string, fn func(r RouteInterface)) RouteInterface {
	subRouter := NewChiRouter()
	if fn != nil {
		fn(subRouter)
	}
	cr.Mount(prefix, subRouter)
	return subRouter
}

func (cr *ChiRouter) Mount(pattern string, handler http.Handler) {
	cr.handle.Mount(pattern, handler)
}

func (cr *ChiRouter) Handle(pattern string, handler http.Handler) {
	cr.handle.Handle(pattern, handler)
}

func (cr *ChiRouter) Use(middlewares ...func(http.Handler) http.Handler) {
	cr.handle.Use(middlewares...)
}

func (cr *ChiRouter) With(middlewares ...func(http.Handler) http.Handler) RouteInterface {
	cr.handle.With(middlewares...)
	return cr
}

func (cr *ChiRouter) StaticFS(pattern, imgDir string) {
	handler := cr.createStaticHandler(pattern, imgDir)
	pattern = path.Join(pattern, "/:name")
	cr.handle.Get(pattern, handler)
	cr.handle.Head(pattern, handler)
}

func (cr *ChiRouter) createStaticHandler(pattern, imgStaticDir string) http.HandlerFunc {
	fileSystem := http.Dir(imgStaticDir)
	fileServer := http.StripPrefix(pattern, http.FileServer(fileSystem))
	return func(w http.ResponseWriter, r *http.Request) {
		fileName := chi.URLParam(r, "name")
		file, err := fileSystem.Open(fileName)
		if err != nil {
			log.Println("error when Open File", err.Error())
			return
		}
		_ = file.Close()

		log.Println("done")
		fileServer.ServeHTTP(w, r)
		return
	}
}

// FileServer is serving static files
func FileServer(r chi.Router, public string, static string) {

	if strings.ContainsAny(public, "{}*") {
		panic("FileServer does not permit URL parameters.")
	}

	root, _ := filepath.Abs(static)
	if _, err := os.Stat(root); os.IsNotExist(err) {
		panic("Static Documents Directory Not Found")
	}

	fs := http.StripPrefix(public, http.FileServer(http.Dir(root)))

	log.Println(public, "->", root)
	if public != "/" && public[len(public)-1] != '/' {
		r.Get(public, http.RedirectHandler(public+"/", 301).ServeHTTP)
		public += "/"
	}

	r.Get(public+"*", func(w http.ResponseWriter, r *http.Request) {
		file := strings.Replace(r.RequestURI, public, "/", 1)
		if _, err := os.Stat(root + file); os.IsNotExist(err) {
			http.ServeFile(w, r, path.Join(root, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	})
}

func (cr *ChiRouter) Get(pattern string, handler WrapperFunc) {
	cr.handle.Get(pattern, cr.wrapper(pattern, handler))
}
func (cr *ChiRouter) Post(pattern string, handler WrapperFunc) {
	cr.handle.Post(pattern, cr.wrapper(pattern, handler))
}
func (cr *ChiRouter) Patch(pattern string, handler WrapperFunc) {
	cr.handle.Patch(pattern, cr.wrapper(pattern, handler))
}
func (cr *ChiRouter) Delete(pattern string, handler WrapperFunc) {
	cr.handle.Delete(pattern, cr.wrapper(pattern, handler))
}
func (cr *ChiRouter) Put(pattern string, handler WrapperFunc) {
	cr.handle.Put(pattern, cr.wrapper(pattern, handler))
}

func (cr *ChiRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cr.handle.ServeHTTP(w, r)
}

func (cr *ChiRouter) Routes() []chi.Route {
	return cr.handle.Routes()
}

func (cr *ChiRouter) Middlewares() chi.Middlewares {
	return cr.handle.Middlewares()
}

// Match(rctx *Context, method, path string) bool
func (cr *ChiRouter) Match(rctx *chi.Context, method, path string) bool {
	return cr.handle.Match(rctx, method, path)
}

func (cr *ChiRouter) wrapper(pattern string, handler WrapperFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		t := time.Now()
		ctx, cancel := context.WithTimeout(request.Context(), time.Second*time.Duration(cr.timeout))
		defer cancel()

		respChan := make(chan *response.JSON)
		go func() {
			defer panicRecover(request, pattern)
			//handler
			if !cr.isSfActive {
				resp := handler(writer, request)
				respChan <- resp
				return
			}

			uniqueReqKey := generateUniqueRequestKey(request)

			sfResponse, err, _ := cr.sf.Do(uniqueReqKey, func() (interface{}, error) {
				handlerResp := handler(writer, request)
				return handlerResp, nil
			})
			if err != nil {
				log.Errorln("[SINGLEFLIGHT] - Error when do singleFlight request")
				respChan <- response.NewJSON().SetError(http.StatusInternalServerError, err)
				return
			}
			respChan <- sfResponse.(*response.JSON)
		}()

		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				writer.WriteHeader(http.StatusGatewayTimeout)
				_, err := writer.Write([]byte("timeout"))
				if err != nil {
					log.Error("context deadline exceed: ", err.Error())
				}
			}
		case responseFunc := <-respChan:
			if responseFunc != nil {
				_ = responseFunc.ErrorChecking(request)
				responseFunc.Latency = time.Since(t).Seconds() * 1000
				responseFunc.Send(writer)
			} else {
				log.Infoln(request.URL.Path + ": handler send nil response")
			}
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

func panicRecover(r *http.Request, path string) {
	if err := recover(); err != nil {
		stackTrace := string(debug.Stack())
		b, _ := io.ReadAll(r.Body)

		fields := map[string]interface{}{
			"Request.Body":  string(b),
			"Host":          r.Host,
			"RequestMethod": r.Method,
			"RequestURI":    r.RequestURI,
			"Error":         err,
			"Path":          path,
		}
		// slack.PostToSlack(fmt.Errorf("panic in api handler, [Path] %s, [err] %v", path, err), stackTrace, "", fields)

		panicData := fields
		panicData["StackTrace"] = stackTrace
		panicData["IP"] = r.RemoteAddr

		// route path
		//routePath := r.URL.Path
		//panicData["ServiceName"] = constants.ServiceName[routePath]

		panicData["StackError"] = err
		ctx := r.Context()
		traceId := helper.GenerateUUIDV4()
		byteData, _ := json.Marshal(panicData)
		notifMsg := fmt.Sprintf(`your API is panic:
			%s
			traceID: %s`, string(byteData), traceId)
		notif := context2.GetNotif(r.Context())
		allNotifPlatform := notif.GetAllPlatform()
		go func() {
			for _, service := range allNotifPlatform {
				if service.IsActive() {
					if err := service.Send(ctx, notifMsg, nil); err != nil {
						log.Errorf("error when send %s notifications: %s", service.Type(), err.Error())
					}
				}

			}
		}()
		log.Error("got panic at handler: ", fields)
	}
}
