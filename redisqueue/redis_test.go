package redisqueue

import (
	"context"
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type testJob struct{ Kind, Value string }

func (j testJob) JobKind() string { return j.Kind }
func TestDurableRecoveryAndOwnerFencing(t *testing.T) {
	r := miniredis.RunT(t)
	s, e := New[testJob]("redis://"+r.Addr(), "jobs-test")
	require.NoError(t, e)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	j := testJob{"digest", "2026-03-01"}
	ok, e := s.Enqueue(ctx, j)
	require.NoError(t, e)
	require.True(t, ok)
	ok, e = s.Enqueue(ctx, j)
	require.NoError(t, e)
	require.False(t, ok)
	got, ok, e := s.Take(ctx, "old", time.Second)
	require.NoError(t, e)
	require.True(t, ok)
	require.Equal(t, j, got)
	_, ok, e = s.Take(ctx, "other", time.Minute)
	require.NoError(t, e)
	require.False(t, ok)
	r.SetTime(time.Now().Add(2 * time.Second))
	got, ok, e = s.Take(ctx, "new", time.Minute)
	require.NoError(t, e)
	require.True(t, ok)
	require.Equal(t, j, got)
	ok, e = s.Renew(ctx, j.Kind, "old", time.Minute)
	require.NoError(t, e)
	require.False(t, ok)
	require.Error(t, s.Complete(ctx, j.Kind, "old", ""))
	require.NoError(t, s.Complete(ctx, j.Kind, "new", ""))
	_, ok, e = s.Take(ctx, "another", time.Minute)
	require.NoError(t, e)
	require.False(t, ok)
	ok, e = s.Enqueue(ctx, j)
	require.NoError(t, e)
	require.True(t, ok)
}
