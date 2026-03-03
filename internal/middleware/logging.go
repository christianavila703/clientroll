package middleware

import (
	"log"
	"net/http"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// Logger provides structured HTTP request logging.
type Logger struct{}

// NewLogger creates a new Logger middleware.
func NewLogger() *Logger {
	return &Logger{}
}

// Handler returns an http.Handler that logs requests.
func (l *Logger) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		ww := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(ww, r)

		duration := time.Since(start)
		log.Printf(
			"http.request request_id=%s method=%s path=%s status=%d duration=%s remote=%s user_agent=%q",
			chimw.GetReqID(r.Context()),
			r.Method,
			r.URL.Path,
			ww.statusCode,
			duration.String(),
			r.RemoteAddr,
			r.UserAgent(),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
