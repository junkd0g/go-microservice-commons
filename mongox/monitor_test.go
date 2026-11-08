package mongox

import (
	"context"
	"encoding/json"
	goctx "github.com/junkd0g/go-microservice-commons/context"
	"go.mongodb.org/mongo-driver/event"
	"strings"
	"testing"
	"time"
)

type testLogger struct{ entries []map[string]interface{} }

func (l *testLogger) Info(_ context.Context, _ string, f ...map[string]interface{}) {
	l.entries = append(l.entries, f...)
}
func (l *testLogger) Error(ctx context.Context, s string, f ...map[string]interface{}) {
	l.Info(ctx, s, f...)
}
func TestMonitorRedactsDriverDetailsAndHonorsFilter(t *testing.T) {
	log := &testLogger{}
	ctx := goctx.WithRequestID(context.Background(), "r-1")
	m := Monitor(MonitorOptions{Logger: log, IgnoreFailure: func(_ context.Context, e *event.CommandFailedEvent) bool { return e.CommandName == "expected" }})
	finished := event.CommandFinishedEvent{CommandName: "find", DatabaseName: "test", Duration: time.Second, ConnectionID: "private:password"}
	m.Succeeded(ctx, &event.CommandSucceededEvent{CommandFinishedEvent: finished})
	m.Failed(ctx, &event.CommandFailedEvent{CommandFinishedEvent: finished, Failure: "private payload"})
	finished.CommandName = "expected"
	m.Failed(ctx, &event.CommandFailedEvent{CommandFinishedEvent: finished})
	if len(log.entries) != 2 || log.entries[0]["request_id"] != "r-1" {
		t.Fatalf("bad monitor events: %v", log.entries)
	}
	b, _ := json.Marshal(log.entries)
	if strings.Contains(string(b), "private") || strings.Contains(string(b), "password") {
		t.Fatal("sensitive driver details logged")
	}
}
