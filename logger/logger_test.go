package logger_test

import (
	"context"
	"testing"

	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	goctx "github.com/junkd0g/go-microservice-commons/context"
	"github.com/junkd0g/go-microservice-commons/logger"
)

func TestInfoLog(t *testing.T) {
	log, err := logger.NewLogger()
	if err != nil {
		t.Fatalf("Error creating logger: %v", err)
	}

	// Create a memory logger to capture log entries.
	core, recorded := observer.New(zapcore.InfoLevel)
	log.SetCore(core)

	// Log a message.
	log.Info(context.Background(), "Info Message", map[string]interface{}{"key": "value", "number": 1})

	// Check if the log entry was recorded.
	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(entries))
	}

	if entries[0].Message != "Info Message" {
		t.Errorf("Unexpected message: %s", entries[0].Message)
	}

	if len(entries[0].Context) != 2 {
		t.Fatalf("Expected 2 fields, got %d", len(entries[0].Context))
	}
}

func TestErrorLog(t *testing.T) {
	log, err := logger.NewLogger()
	if err != nil {
		t.Fatalf("Error creating logger: %v", err)
	}

	// Create a memory logger to capture log entries.
	core, recorded := observer.New(zapcore.ErrorLevel)
	log.SetCore(core)

	// Log an error.
	log.Error(context.Background(), "Error Message", map[string]interface{}{"error_key": "error_value", "error_number": 2})

	// Check if the log entry was recorded.
	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(entries))
	}

	if entries[0].Message != "Error Message" {
		t.Errorf("Unexpected message: %s", entries[0].Message)
	}

	if len(entries[0].Context) != 2 {
		t.Fatalf("Expected 2 fields, got %d", len(entries[0].Context))
	}
}

func TestInfoLog_WithVariousFieldTypes(t *testing.T) {
	log, err := logger.NewLogger()
	if err != nil {
		t.Fatalf("Error creating logger: %v", err)
	}

	core, recorded := observer.New(zapcore.InfoLevel)
	log.SetCore(core)

	log.Info(context.Background(), "mixed types", map[string]interface{}{
		"str":   "hello",
		"num":   42,
		"flag":  true,
		"ratio": 3.14,
	})

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(entries))
	}

	if len(entries[0].Context) != 4 {
		t.Fatalf("Expected 4 fields, got %d", len(entries[0].Context))
	}

	fieldKeys := make(map[string]bool)
	for _, f := range entries[0].Context {
		fieldKeys[f.Key] = true
	}
	for _, key := range []string{"str", "num", "flag", "ratio"} {
		if !fieldKeys[key] {
			t.Errorf("Expected field %q to be present", key)
		}
	}
}

func TestInfoLog_WithMutableFieldsFromContext(t *testing.T) {
	log, err := logger.NewLogger()
	if err != nil {
		t.Fatalf("Error creating logger: %v", err)
	}

	core, recorded := observer.New(zapcore.InfoLevel)
	log.SetCore(core)

	mf := goctx.NewMutableFields()
	mf.AddField(map[string]interface{}{"request_id": "abc-123"})

	ctx := context.WithValue(context.Background(), goctx.ContextKeyLoggerFields, mf)

	log.Info(ctx, "with context fields", map[string]interface{}{"extra": "val"})

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(entries))
	}

	// Should have 2 fields: "extra" from the direct arg + "request_id" from context.
	if len(entries[0].Context) != 2 {
		t.Fatalf("Expected 2 fields, got %d", len(entries[0].Context))
	}

	fieldMap := make(map[string]string)
	for _, f := range entries[0].Context {
		fieldMap[f.Key] = f.String
	}
	if fieldMap["request_id"] != "abc-123" {
		t.Errorf("Expected request_id=abc-123, got %s", fieldMap["request_id"])
	}
	if fieldMap["extra"] != "val" {
		t.Errorf("Expected extra=val, got %s", fieldMap["extra"])
	}
}

func TestErrorLog_WithMutableFieldsFromContext(t *testing.T) {
	log, err := logger.NewLogger()
	if err != nil {
		t.Fatalf("Error creating logger: %v", err)
	}

	core, recorded := observer.New(zapcore.ErrorLevel)
	log.SetCore(core)

	mf := goctx.NewMutableFields()
	mf.AddField(map[string]interface{}{"trace_id": "xyz-789"})

	ctx := context.WithValue(context.Background(), goctx.ContextKeyLoggerFields, mf)

	log.Error(ctx, "error with context fields")

	entries := recorded.All()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(entries))
	}

	if len(entries[0].Context) != 1 {
		t.Fatalf("Expected 1 field, got %d", len(entries[0].Context))
	}

	if entries[0].Context[0].Key != "trace_id" {
		t.Errorf("Expected trace_id field, got %s", entries[0].Context[0].Key)
	}
}
