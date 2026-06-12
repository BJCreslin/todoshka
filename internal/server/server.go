package server

import (
	"net/http"
	"time"
)

func Wrap(h http.Handler) http.Handler { return logMiddleware(h) }

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		println(r.Method, r.URL.Path, time.Since(start).String())
	})
}
