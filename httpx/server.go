// Package httpx provides request instrumentation and reusable HTTP transports.
package httpx

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"time"

	goctx "github.com/junkd0g/go-microservice-commons/context"
)

var validParent = regexp.MustCompile(`^00-[0-9a-f]{32}-[0-9a-f]{16}-[0-9a-f]{2}$`)

type parentKey struct{}

// ValidID accepts bounded identifiers safe for headers and logs.
func ValidID(id string) bool { return goctx.ValidRequestID(id) }

// RequestID returns the shared request identity.
func RequestID(ctx context.Context) string { return goctx.RequestID(ctx) }

// ServerOptions configures request instrumentation; application labels and response
// classifications are supplied by the caller rather than hardcoded in the library.
type ServerOptions struct {
	Service               string
	Logger                goctx.Logger
	Route                 func(*http.Request) string
	Fields                func(*http.Request) map[string]interface{}
	QuietRoute            func(string) bool
	ClassifyResponse      func(int, http.Header) (string, bool)
	LegacyRequestIDHeader string
}

// Middleware correlates contextual logs and records HTTP completion.
func Middleware(options ServerOptions) func(http.Handler) http.Handler {
	service, logger := options.Service, options.Logger
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if !ValidID(id) && options.LegacyRequestIDHeader != "" {
				id = r.Header.Get(options.LegacyRequestIDHeader)
			}
			if !ValidID(id) {
				id = goctx.NewRequestID()
			}
			fields := map[string]interface{}{}
			if options.Fields != nil {
				for k, v := range options.Fields(r) {
					fields[k] = v
				}
			}
			fields["request_id"], fields["service"] = id, service
			parent := r.Header.Get("traceparent")
			if !validTraceParent(parent) {
				parent = ""
			}
			if parent != "" {
				fields["trace_id"] = parent[3:35]
			}
			existing := append(goctx.GetFieldsFromContext(r.Context()), fields)
			ctx := goctx.AddFieldsToContext(goctx.WithRequestID(r.Context(), id), existing)
			if logger != nil {
				ctx = goctx.AddLoggerToContext(ctx, logger)
			}
			ctx = context.WithValue(ctx, parentKey{}, parent)
			w.Header().Set("X-Request-ID", id)
			r = r.Clone(ctx)
			r.Header.Set("X-Request-ID", id)
			if options.LegacyRequestIDHeader != "" {
				w.Header().Set(options.LegacyRequestIDHeader, id)
			}
			route := "unmatched"
			if options.Route != nil {
				route = options.Route(r)
			}
			recorder := &responseWriter{ResponseWriter: w, status: 200}
			start := time.Now()
			completed := false
			defer func() {
				if logger == nil {
					return
				}
				if options.QuietRoute != nil && options.QuietRoute(route) && recorder.status < 400 && completed {
					return
				}
				fields := map[string]interface{}{"method": r.Method, "route": route, "status_code": recorder.status, "duration_ms": int(time.Since(start).Milliseconds()), "response_bytes": recorder.bytes}
				if !completed {
					fields["error_type"] = "panic"
					if !recorder.wrote {
						fields["status_code"] = 0
					}
				}
				kind, informational := "", false
				if options.ClassifyResponse != nil {
					kind, informational = options.ClassifyResponse(recorder.status, recorder.Header())
				}
				if kind != "" {
					fields["error_type"] = kind
				}
				if (recorder.status >= 500 && !informational) || !completed {
					logger.Error(ctx, "HTTP request completed", fields)
				} else {
					logger.Info(ctx, "HTTP request completed", fields)
				}
			}()
			next.ServeHTTP(recorder, r.WithContext(ctx))
			completed = true
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
	wrote  bool
}

func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *responseWriter) WriteHeader(status int) {
	if status >= 100 && status < 200 && status != 101 {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if w.wrote {
		return
	}
	w.status = status
	w.wrote = true
	w.ResponseWriter.WriteHeader(status)
}
func (w *responseWriter) Write(p []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(200)
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}
func (w *responseWriter) Flush() {
	if !w.wrote {
		w.WriteHeader(200)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func validTraceParent(parent string) bool {
	return validParent.MatchString(parent) && !strings.HasPrefix(parent[3:], strings.Repeat("0", 32)) && parent[36:52] != strings.Repeat("0", 16)
}
