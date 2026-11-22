package rediscache

import (
	"context"
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestExpiredOwnerCannotRenewPublishOrReleaseSuccessor(t *testing.T) {
	r := miniredis.RunT(t)
	s, e := New("redis://"+r.Addr(), "test")
	require.NoError(t, e)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	ok, e := s.Acquire(ctx, "n", "k", "old", time.Second)
	require.NoError(t, e)
	require.True(t, ok)
	r.FastForward(2 * time.Second)
	ok, e = s.Acquire(ctx, "n", "k", "new", time.Minute)
	require.NoError(t, e)
	require.True(t, ok)
	ok, e = s.Renew(ctx, "n", "k", "old", time.Minute)
	require.NoError(t, e)
	require.False(t, ok)
	ok, e = s.Publish(ctx, "n", "k", "old", []byte("stale"), time.Minute, 10)
	require.NoError(t, e)
	require.False(t, ok)
	require.NoError(t, s.Release(ctx, "n", "k", "old"))
	ok, e = s.Publish(ctx, "n", "k", "new", []byte("fresh"), time.Minute, 10)
	require.NoError(t, e)
	require.True(t, ok)
	v, e := s.Load(ctx, "n", "k")
	require.NoError(t, e)
	require.Equal(t, "fresh", string(v))
}
func TestCapacityAndTTL(t *testing.T) {
	r := miniredis.RunT(t)
	s, e := New("redis://"+r.Addr(), "test")
	require.NoError(t, e)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	for _, k := range []string{"a", "b", "c"} {
		ok, e := s.Acquire(ctx, "n", k, "owner", time.Minute)
		require.NoError(t, e)
		require.True(t, ok)
		ok, e = s.Publish(ctx, "n", k, "owner", []byte(k), time.Second, 2)
		require.NoError(t, e)
		require.True(t, ok)
	}
	values := 0
	for _, k := range []string{"a", "b", "c"} {
		v, e := s.Load(ctx, "n", k)
		require.NoError(t, e)
		if len(v) > 0 {
			values++
		}
	}
	require.Equal(t, 2, values)
	r.FastForward(2 * time.Second)
	v, e := s.Load(ctx, "n", "c")
	require.NoError(t, e)
	require.Empty(t, v)
}
