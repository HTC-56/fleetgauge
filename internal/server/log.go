package server

import (
	"log/slog"
	"net/http"
	"time"
)

// statusRecorder wraps an http.ResponseWriter, capturing the status code
// (defaulting to 200 when a handler writes a body without calling WriteHeader)
// and forwarding every call to the underlying writer. It also implements
// http.Flusher so it can sit between /events and the client without breaking
// streaming.
type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{ResponseWriter: w, status: 200}
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.wrote = true
	return r.ResponseWriter.Write(b)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// LogRequests returns an http.Handler that logs one info record per request
// with method, path, status, and duration_ms attributes.
func LogRequests(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		snap := newStatusRecorder(w)
		next.ServeHTTP(snap, req)
		duration := time.Since(start)

		// WriteHeader captures the explicit status; when no WriteHeader
		// was called the implicit status is 200, which is the default above.
		log.Info("request",
			"method", req.Method,
			"path", req.URL.Path,
			"status", snap.status,
			"duration_ms", float64(duration.Microseconds())/100.0,
		)
	})
}
