package httpx

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type tripFunc func(*http.Request) (*http.Response, error)

func (f tripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestTransportNestedWrappingDoesNotDuplicateOrMutate(t *testing.T) {
	l := &recorderLogger{}
	calls := 0
	h := Middleware(ServerOptions{Logger: l})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, _ := http.NewRequestWithContext(r.Context(), "GET", "http://example.test/private?token=secret", nil)
		base := tripFunc(func(out *http.Request) (*http.Response, error) {
			calls++
			if out.Header.Get("X-Request-ID") != "id-123" {
				t.Fatal("identity lost")
			}
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("ok"))}, nil
		})
		resp, err := (Transport{Base: Transport{Base: base}}).RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if req.Header.Get("X-Request-ID") != "" {
			t.Fatal("request mutated")
		}
	}))
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Request-ID", "id-123")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if calls != 1 || len(l.events) != 2 {
		t.Fatalf("duplicate request/event: %d %d", calls, len(l.events))
	}
}
func TestRetriesRespectMethodHintAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		method, hint string
		calls        int
	}{{"POST", "", 1}, {"GET", "30", 1}} {
		n := 0
		tr := retryTransport{base: tripFunc(func(*http.Request) (*http.Response, error) {
			n++
			h := http.Header{}
			h.Set("Retry-After", tc.hint)
			return &http.Response{StatusCode: 503, Header: h, Body: io.NopCloser(strings.NewReader("later"))}, nil
		})}
		r, _ := http.NewRequest(tc.method, "http://example.test", nil)
		resp, err := tr.RoundTrip(r)
		if err != nil || n != tc.calls {
			t.Fatal("unsafe retry")
		}
		_ = resp.Body.Close()
	}
	ctx, cancel := context.WithCancel(context.Background())
	n := 0
	tr := retryTransport{base: tripFunc(func(*http.Request) (*http.Response, error) { n++; cancel(); return nil, context.Canceled })}
	r, _ := http.NewRequestWithContext(ctx, "GET", "http://example.test", nil)
	_, err := tr.RoundTrip(r)
	if !errors.Is(err, context.Canceled) || n != 1 {
		t.Fatal("cancellation retried")
	}
}
