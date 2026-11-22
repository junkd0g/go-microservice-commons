package rediscache

import (
	"github.com/stretchr/testify/require"
	"net/url"
	"testing"
)

func TestConnectionURLPreservesReservedCredentials(t *testing.T) {
	raw := ConnectionURL("", "cache.test:25061", "user", "a@b:c/#?%")
	u, e := url.Parse(raw)
	require.NoError(t, e)
	p, ok := u.User.Password()
	require.True(t, ok)
	require.Equal(t, "a@b:c/#?%", p)
	require.Equal(t, "rediss", u.Scheme)
	require.Equal(t, "redis://test:6379", ConnectionURL("redis://test:6379", "ignored", "", ""))
}
