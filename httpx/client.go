package httpx

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	goctx "github.com/junkd0g/go-microservice-commons/context"
)

var fallbackTransport = http.DefaultTransport

type transportKey struct{}

// Transport forwards request correlation and records upstream results.
type Transport struct {
	Base             http.RoundTripper
	RequestHeaders   func(*http.Request) http.Header
	ClassifyResponse func(*http.Response) (string, bool)
	ResponseError    func(*http.Response) error
}

// RoundTrip preserves caller headers and the underlying transport behavior.
func (t Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = fallbackTransport
	}
	if r.Context().Value(transportKey{}) != nil {
		return base.RoundTrip(r)
	}
	copy := r.Clone(context.WithValue(r.Context(), transportKey{}, true))
	id := RequestID(r.Context())
	if id == "" {
		id = goctx.NewRequestID()
	}
	copy.Header.Set("X-Request-ID", id)
	if t.RequestHeaders != nil {
		for key, values := range t.RequestHeaders(r) {
			copy.Header[key] = append([]string(nil), values...)
		}
	}
	if parent, ok := r.Context().Value(parentKey{}).(string); ok && parent != "" {
		copy.Header.Set("traceparent", parent)
	} else if !validTraceParent(copy.Header.Get("traceparent")) {
		copy.Header.Del("traceparent")
	}

	start := time.Now()
	resp, err := base.RoundTrip(copy)
	if logger, e := goctx.GetLoggerFromContext(r.Context()); e == nil {
		fields := map[string]interface{}{"request_id": id, "upstream": r.URL.Hostname(), "method": r.Method, "duration_ms": int(time.Since(start).Milliseconds())}
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		fields["status_code"] = status
		if err != nil {
			kind := "transport_error"
			var timeout net.Error
			switch {
			case errors.Is(err, context.Canceled):
				kind = "canceled"
			case errors.Is(err, context.DeadlineExceeded):
				kind = "timeout"
			case errors.As(err, &timeout) && timeout.Timeout():
				kind = "timeout"
			}
			fields["error_type"] = kind
		}
		kind, informational := "", false
		if t.ClassifyResponse != nil && resp != nil {
			kind, informational = t.ClassifyResponse(resp)
		}
		if kind != "" {
			fields["error_type"] = kind
		}
		if (status >= 500 && !informational) || (err != nil && !errors.Is(err, context.Canceled)) {
			logger.Error(r.Context(), "Upstream request completed", fields)
		} else {
			logger.Info(r.Context(), "Upstream request completed", fields)
		}
	}
	if err == nil && resp != nil && t.ResponseError != nil {
		if responseErr := t.ResponseError(resp); responseErr != nil {
			if resp.Body != nil {
				_ = resp.Body.Close()
			}
			return nil, responseErr
		}
	}
	return resp, err
}
