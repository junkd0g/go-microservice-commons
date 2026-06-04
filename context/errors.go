package context

import "errors"

var (
	// ErrLoggerNotFound is the error returned when the logger is not found in the context
	ErrLoggerNotFound = errors.New("logger not found in context")
)
