package app

import (
	"net/http"
	"runtime/debug"

	"github.com/go-chi/chi/v5/middleware"
)

func RouteNotFoundHandler(w http.ResponseWriter, r *http.Request) {
	err := NotFound("route not found", "ROUTE_NOT_FOUND")
	err.Write(w)
}

func MethodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	err := MethodNotAllowed("method not allowed", "METHOD_NOT_ALLOWED")
	err.Write(w)
}

func PanicHandler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil && rvr != http.ErrAbortHandler {

				logEntry := middleware.GetLogEntry(r)
				if logEntry != nil {
					logEntry.Panic(rvr, debug.Stack())
				} else {
					middleware.PrintPrettyStack(rvr)
				}

				w.WriteHeader(http.StatusInternalServerError)
				err := InternalServerError("server error", "SERVER_ERROR")
				err.Write(w)
			}
		}()

		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}
