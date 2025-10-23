package context

import "errors"

var (
	// ErrLoggerNotFound is the error returned when the logger is not found in the context
	ErrLoggerNotFound = errors.New("logger not found in context")

	// ErrLoggerFieldsNotFound is the error returned when the logger fields are not found in the context
	ErrLoggerFieldsNotFound = errors.New("logger fields not found in context")
)
