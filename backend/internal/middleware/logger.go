package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture the status code written
// by downstream handlers — http.ResponseWriter doesn't expose it after the
// fact, so we intercept WriteHeader.
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func wrapResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, status: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.status = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

// Logger returns middleware that logs every request as a structured slog record.
//
// Each log line contains:
//   - method, path, status, duration, remote_addr
//   - request_id (if set by the RequestID middleware upstream)
//
// Requests to /health are logged at Debug level to avoid polluting production
// logs with high-frequency liveness probes.
func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := wrapResponseWriter(w)

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)

			attrs := []any{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", wrapped.status),
				slog.String("duration", duration.String()),
				slog.String("remote_addr", r.RemoteAddr),
			}

			// Include request-id if the RequestID middleware set it.
			if id := r.Header.Get("X-Request-Id"); id != "" {
				attrs = append(attrs, slog.String("request_id", id))
			}

			level := slog.LevelInfo
			// Health checks are noise at INFO; demote them.
			if r.URL.Path == "/health" {
				level = slog.LevelDebug
			}
			// Log 5xx at Error so alerts can trigger on them.
			if wrapped.status >= 500 {
				level = slog.LevelError
			}

			logger.Log(r.Context(), level, "request", attrs...)
		})
	}
}

// Recoverer catches any panic that escapes a handler, logs it with a full
// stack trace, and returns a 500 to the client instead of crashing the server.
func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered",
						slog.Any("panic", rec),
						slog.String("stack", string(debug.Stack())),
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
					)
					http.Error(w,
						`{"error":"Internal Server Error"}`,
						http.StatusInternalServerError,
					)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
