package rediscache

import "net/url"

// ConnectionURL uses an explicit URL for local/test connections or safely
// encodes the credentials of a managed TLS connection. Empty configuration
// remains invalid and is rejected by New.
func ConnectionURL(rawURL, address, username, password string) string {
	if rawURL != "" {
		return rawURL
	}
	if address == "" {
		return ""
	}
	if username == "" {
		username = "default"
	}
	return (&url.URL{Scheme: "rediss", Host: address, User: url.UserPassword(username, password)}).String()
}
