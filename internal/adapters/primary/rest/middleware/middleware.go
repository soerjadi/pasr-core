package middleware

import (
	"net/http"
	"runtime/debug"
	"time"

	log "github.com/oemahdev/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Tracing starts a server span per request so downstream spans and log lines share its trace_id.
// The span is renamed to the matched route pattern after routing, keeping span names low-cardinality.
// It must wrap the mux and sit outside Logger and Recover.
func Tracing(next http.Handler) http.Handler {
	t := otel.Tracer("nusantara-core.rest")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := t.Start(r.Context(), r.Method+" "+r.URL.Path)
		defer span.End()

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		req := r.WithContext(ctx)
		next.ServeHTTP(rec, req)

		if req.Pattern != "" {
			span.SetName(req.Pattern)
		}
		if rec.status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, http.StatusText(rec.status))
		}
	})
}

// Recover turns a handler panic into a 500 instead of killing the connection.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				log.ErrorCtx(r.Context(), "panic recovered", log.Field("value", v), log.Field("stack", string(debug.Stack())))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"code":"INTERNAL","message":"internal server error"}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.InfoCtx(r.Context(), "HTTP Request",
			log.Field("method", r.Method),
			log.Field("path", r.URL.Path),
			log.Field("status", rec.status),
			log.Field("duration", time.Since(start).String()),
		)
	})
}

func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if origin := r.Header.Get("Origin"); origin != "" && (allowed["*"] || allowed[origin]) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
