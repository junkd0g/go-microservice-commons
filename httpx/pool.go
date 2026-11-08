package httpx

import (
	"math/rand"
	"net/http"
	"time"
)

const (
	// maxIdleConnsPerHost is sized above the widest per-request fan-out
	// (top-leagues standings: 7 leagues) so pooled connections are reused.
	maxIdleConnsPerHost = 16
	// maxRetries bounds retry attempts for idempotent GETs.
	maxRetries = 2
	// initialBackoff is the first retry delay; it doubles per attempt with
	// full jitter added.
	initialBackoff = 200 * time.Millisecond
)

// NewClient returns an HTTP client with a connection pool sized for the adapter
// fan-outs and bounded, jittered retries for idempotent GET requests. The
// client-level timeout bounds the whole exchange including retries.
func NewClient(timeout time.Duration) *http.Client {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{}
	}
	tuned := base.Clone()
	tuned.MaxIdleConnsPerHost = maxIdleConnsPerHost

	return &http.Client{
		Timeout:   timeout,
		Transport: &retryTransport{base: tuned},
	}
}

// retryTransport retries GET requests — and only GETs, which are idempotent
// and carry no body — on transport-level errors and 502/503/504 responses.
type retryTransport struct {
	base http.RoundTripper
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet {
		return t.base.RoundTrip(req)
	}

	backoff := initialBackoff
	for attempt := 0; ; attempt++ {
		resp, err := t.base.RoundTrip(req)
		if (resp != nil && resp.Header.Get("Retry-After") != "") || !isRetryable(resp, err) || attempt >= maxRetries || req.Context().Err() != nil {
			return resp, err
		}
		if resp != nil {
			_ = resp.Body.Close()
		}

		// Full jitter keeps concurrent retries from re-bursting in lockstep.
		delay := backoff + time.Duration(rand.Int63n(int64(backoff))) //nolint:gosec // jitter, not security-sensitive
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(delay):
		}
		backoff *= 2
	}
}

// isRetryable reports whether the exchange failed in a way a retry can fix:
// a transport error (dial, reset, timeout) or an upstream gateway error.
// 4xx responses are the caller's problem and are never retried.
func isRetryable(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	if resp == nil {
		return false
	}
	switch resp.StatusCode {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}
