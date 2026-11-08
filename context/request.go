package context

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"regexp"
)

type requestIDKey struct{}

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

// ValidRequestID accepts bounded identifiers safe to put in headers and logs.
func ValidRequestID(id string) bool { return requestIDPattern.MatchString(id) }

// NewRequestID returns a cryptographically random request identifier.
func NewRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

// WithRequestID attaches the caller-provided request identity to a new context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestID reads the identity, with compatibility for existing logging contexts.
func RequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	for _, fields := range GetFieldsFromContext(ctx) {
		if id, ok := fields["request_id"].(string); ok {
			return id
		}
	}
	return ""
}
