package httpx

import (
	"context"
	"encoding/json"
	goctx "github.com/junkd0g/go-microservice-commons/context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type recorderLogger struct {
	events []map[string]interface{}
	errors int
}

func (l *recorderLogger) Info(ctx context.Context, msg string, fields ...map[string]interface{}) {
	m := map[string]interface{}{"message": msg}
	for _, f := range append(goctx.GetFieldsFromContext(ctx), fields...) {
		for k, v := range f {
			m[k] = v
		}
	}
	l.events = append(l.events, m)
}
func (l *recorderLogger) Error(ctx context.Context, msg string, fields ...map[string]interface{}) {
	l.errors++
	l.Info(ctx, msg, fields...)
}
func TestMiddlewarePropagatesSafeIdentityAndKeepsStream(t *testing.T) {
	log := &recorderLogger{}
	h := Middleware(ServerOptions{Service: "sample", Logger: log, LegacyRequestIDHeader: "X-Legacy-ID", Route: func(*http.Request) string { return "/items/{id}" }})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if RequestID(r.Context()) != "legacy-123" || r.Header.Get("X-Request-ID") != "legacy-123" {
			t.Fatal("identity lost")
		}
		w.WriteHeader(503)
		w.WriteHeader(200)
		_, _ = w.Write([]byte("no"))
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Fatal(err)
		}
	}))
	r := httptest.NewRequest("GET", "/items/private?token=secret", nil)
	r.Header.Set("X-Legacy-ID", "legacy-123")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 503 || !w.Flushed || w.Header().Get("X-Request-ID") != "legacy-123" || log.errors != 1 {
		t.Fatal("status, streaming or logging changed")
	}
	raw, _ := json.Marshal(log.events)
	if strings.Contains(string(raw), "private") || strings.Contains(string(raw), "secret") {
		t.Fatal("private URL logged")
	}
	if log.events[0]["route"] != "/items/{id}" || log.events[0]["response_bytes"] != 2 {
		t.Fatalf("bad event: %v", log.events)
	}
}
func TestMiddlewareClassificationIsOptIn(t *testing.T) {
	for _, classify := range []bool{false, true} {
		l := &recorderLogger{}
		options := ServerOptions{Logger: l}
		if classify {
			options.ClassifyResponse = func(int, http.Header) (string, bool) { return "waiting", true }
		}
		h := Middleware(options)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
		if (l.errors == 0) != classify {
			t.Fatalf("classification %v errors %d", classify, l.errors)
		}
	}
}
