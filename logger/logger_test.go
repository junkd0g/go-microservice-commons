package logger_test

import (
	"context"
	"testing"

	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

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
