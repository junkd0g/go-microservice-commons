package httpx

import (
	goctx "github.com/junkd0g/go-microservice-commons/context"
	"net/http"
)

// RequestIdentity sets one identity before routing, including unmatched routes.
// The optional legacy header is kept in sync with X-Request-ID.
func RequestIdentity(legacyHeader string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !goctx.ValidRequestID(id) && legacyHeader != "" {
			id = r.Header.Get(legacyHeader)
		}
		if !goctx.ValidRequestID(id) {
			id = goctx.NewRequestID()
		}
		r = r.Clone(goctx.WithRequestID(r.Context(), id))
		r.Header.Set("X-Request-ID", id)
		w.Header().Set("X-Request-ID", id)
		if legacyHeader != "" {
			w.Header().Set(legacyHeader, id)
		}
		next.ServeHTTP(w, r)
	})
}
