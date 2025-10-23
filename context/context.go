// Package context manages the service context. It provides facilities
// to add objects to the context and retrieve them, ensuring thread safety.
// This context is particularly useful for adding logging capabilities
// and custom fields relevant to service operation.
package context

import (
	"context"
	"sync"
)

// contextKey represents the type of the key for storing
// values within the context.
type contextKey string

// String converts a contextKey into its string representation.
func (c contextKey) String() string {
	return string(c)
}

// MutableFields represents a collection of fields
// that can be safely mutated across multiple goroutines.
type MutableFields struct {
	sync.RWMutex
	fields []map[string]interface{}
}

// NewMutableFields initializes a new instance of MutableFields.
func NewMutableFields() *MutableFields {
	return &MutableFields{}
}

// AddField safely adds a field to the MutableFields.
func (mf *MutableFields) AddField(field map[string]interface{}) {
	mf.Lock()
	defer mf.Unlock()
	mf.fields = append(mf.fields, field)
}

// GetFields safely retrieves all fields from the MutableFields.
func (mf *MutableFields) GetFields() []map[string]interface{} {
	mf.RLock()
	defer mf.RUnlock()
	return mf.fields
}

// Logger provides an interface for logging functionalities.
type Logger interface {
	Info(ctx context.Context, msg string, fields ...map[string]interface{})
	Error(ctx context.Context, msg string, fields ...map[string]interface{})
}

// Predefined context keys for storing logger and fields in the context.
var (
	contextKeyLogger       = contextKey("logger")
	ContextKeyLoggerFields = contextKey("loggerFields")
)

// AddLoggerToContex associates a logger with a context.
func AddLoggerToContex(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, contextKeyLogger, logger)
}

// GetLoggerFromContext retrieves the logger associated with a context.
// If the logger does not exist, it returns an ErrLoggerNotFound error.
func GetLoggerFromContext(ctx context.Context) (Logger, error) {
	logger, ok := ctx.Value(contextKeyLogger).(Logger)
	if !ok {
		return nil, ErrLoggerNotFound
	}
	return logger, nil
}

// AddFieldsToContext associates an array of fields with a context.
func AddFieldsToContext(ctx context.Context, fields []map[string]interface{}) context.Context {
	return context.WithValue(ctx, ContextKeyLoggerFields, fields)
}

// GetFieldsFromContext retrieves the array of fields associated with a context.
// If the fields do not exist, it returns an empty slice.
func GetFieldsFromContext(ctx context.Context) []map[string]interface{} {
	fields, ok := ctx.Value(ContextKeyLoggerFields).([]map[string]interface{})
	if !ok {
		return []map[string]interface{}{}
	}
	return fields
}
