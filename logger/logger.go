// Package logger provides a customized logging utility
// built on top of Uber's zap library. It also integrates with the
// service's internal context to automatically extract and log mutable fields.
package logger

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	goctx "github.com/junkd0g/go-microservice-commons/context"
)

// LogField represents a custom map type for log fields.
type LogField map[string]interface{}

// Logger encapsulates an instance of zap's logger with custom functionalities.
type Logger struct {
	logger *zap.Logger
}

// NewLogger initializes and returns a new instance of Logger with predefined configurations.
func NewLogger() (*Logger, error) {
	config := zap.NewProductionConfig()

	// Set the desired logging level and control stack trace settings
	config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	config.DisableStacktrace = true

	// Initialize the logger with the given configuration
	logger, err := config.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	return &Logger{
		logger: logger,
	}, nil
}

// SetCore updates the logger's core, useful for testing and custom configurations.
func (l *Logger) SetCore(core zapcore.Core) {
	l.logger = zap.New(core)
}

// Info logs an informational message and extracts additional fields from the context, if present.
func (l *Logger) Info(ctx context.Context, msg string, fields ...map[string]interface{}) {
	// Extract additional fields from the context, if available
	if mutableFields, ok := ctx.Value(goctx.ContextKeyLoggerFields).(*goctx.MutableFields); ok {
		extraFields := mutableFields.GetFields()
		fields = append(fields, extraFields...)
	}

	// Convert custom fields to zap fields and log the message
	zapFields := convertToZapFields(fields...)
	l.logger.Info(msg, zapFields...)
}

// Error logs an error message and extracts additional fields from the context, if present.
func (l *Logger) Error(ctx context.Context, msg string, fields ...map[string]interface{}) {
	// Extract additional fields from the context, if available
	if mutableFields, ok := ctx.Value(goctx.ContextKeyLoggerFields).(*goctx.MutableFields); ok {
		extraFields := mutableFields.GetFields()
		fields = append(fields, extraFields...)
	}

	// Convert custom fields to zap fields and log the error message
	zapFields := convertToZapFields(fields...)
	l.logger.Error(msg, zapFields...)
}

// convertToZapFields transforms custom log fields into zap-compatible fields.
// It currently supports fields of type string and int.
func convertToZapFields(fields ...map[string]interface{}) []zap.Field {
	var zapFields []zap.Field

	for _, field := range fields {
		for k, v := range field {
			switch value := v.(type) {
			case string:
				zapFields = append(zapFields, zap.String(k, value))
			case int:
				zapFields = append(zapFields, zap.Int(k, value))
			}
		}
	}

	return zapFields
}
