package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type ctxKey string

const (
	loggerKey ctxKey = "logger"
	reqIDKey  ctxKey = "request_id"
)

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

func Logger(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(reqIDKey).(string); ok && v != "" {
		return v
	}
	return "unknown"
}

func RequestIDMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := uuid.New().String()
			w.Header().Set("X-Request-ID", id)
			ctx := context.WithValue(r.Context(), reqIDKey, id)
			ctx = WithLogger(ctx, logger.With(slog.String("request_id", id)))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type responseWriterWrapper struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriterWrapper) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func LoggingMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &responseWriterWrapper{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(ww, r)
			dur := time.Since(start)
			Logger(r.Context()).Info("request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", ww.status),
				slog.Int64("duration_ms", dur.Milliseconds()),
			)
		})
	}
}
