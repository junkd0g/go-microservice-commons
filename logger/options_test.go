package logger

import (
	"context"
	goctx "github.com/junkd0g/go-microservice-commons/context"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"testing"
)

func TestServiceOptionsIsolateContextsAndDeduplicate(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	l := NewWithCore(core, Options{Service: "test", QuietInfo: func(s string) bool { return s == "probe" }})
	one := goctx.AddFieldsToContext(context.Background(), []map[string]interface{}{{"request_id": "one"}})
	two := goctx.AddFieldsToContext(context.Background(), []map[string]interface{}{{"request_id": "two"}})
	l.Info(one, "probe")
	l.Info(one, "work", map[string]interface{}{"service": "test", "request_id": "earlier"})
	l.Error(two, "probe")
	if logs.Len() != 2 {
		t.Fatalf("unexpected logs: %v", logs.All())
	}
	for i, e := range logs.All() {
		seen := map[string]bool{}
		for _, f := range e.Context {
			if seen[f.Key] {
				t.Fatalf("duplicate %s", f.Key)
			}
			seen[f.Key] = true
		}
		want := []string{"one", "two"}[i]
		if e.ContextMap()["request_id"] != want || e.ContextMap()["service"] != "test" {
			t.Fatalf("context leak: %v", e.ContextMap())
		}
	}
}
