package context

import (
	"context"
	"testing"
)

func TestRequestIdentityDoesNotLeakBetweenContexts(t *testing.T) {
	base := context.Background()
	one := WithRequestID(base, "one")
	two := WithRequestID(base, "two")
	if RequestID(one) != "one" || RequestID(two) != "two" || RequestID(base) != "" {
		t.Fatal("request contexts leaked")
	}
	if ValidRequestID("bad\nvalue") || ValidRequestID("") || !ValidRequestID(NewRequestID()) {
		t.Fatal("unsafe ID")
	}
}
