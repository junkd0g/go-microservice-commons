package cache

import (
	"context"
	"errors"
	"github.com/alicebob/miniredis/v2"
	"github.com/junkd0g/go-microservice-commons/rediscache"
	"github.com/stretchr/testify/require"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestReplicasShareOneFillAndReuseEmptyResult(t *testing.T) {
	store := newStore(t)
	a, b := New[[]int](time.Minute, 100, store), New[[]int](time.Minute, 100, store)
	var calls atomic.Int32
	start := make(chan struct{})
	release := make(chan struct{})
	load := func(ctx context.Context) ([]int, time.Duration, error) {
		if calls.Add(1) == 1 {
			close(start)
		}
		select {
		case <-release:
			return []int{}, time.Minute, nil
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		}
	}
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := a
			if i%2 == 0 {
				c = b
			}
			v, _, err := c.Get(context.Background(), "key", load)
			if err != nil || len(v) != 0 {
				t.Errorf("get: %v, %v", v, err)
			}
		}()
	}
	<-start
	close(release)
	wg.Wait()
	require.Equal(t, int32(1), calls.Load())
}
func TestFreshnessStaleAndCancellation(t *testing.T) {
	now := time.Now()
	c := New[string](time.Minute, 10, newStore(t)).WithClock(func() time.Time { return now })
	_, _, err := c.Get(context.Background(), "key", func(context.Context) (string, time.Duration, error) { return "old", time.Second, nil })
	require.NoError(t, err)
	now = now.Add(2 * time.Second)
	fail := func(context.Context) (string, time.Duration, error) { return "", 0, errors.New("offline") }
	v, r, err := c.Get(context.Background(), "key", fail)
	require.NoError(t, err)
	require.Equal(t, "old", v)
	require.True(t, r.Stale)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = c.Get(ctx, "key", fail)
	require.ErrorIs(t, err, context.Canceled)
	now = now.Add(time.Minute)
	_, _, err = c.Get(context.Background(), "key", fail)
	require.EqualError(t, err, "offline")
}
func TestCanceledOwnerCannotPoisonNextFill(t *testing.T) {
	store := newStore(t)
	a, b := New[string](time.Minute, 10, store), New[string](time.Minute, 10, store)
	ctx, cancel := context.WithCancel(context.Background())
	start := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, _, e := a.Get(ctx, "k", func(ctx context.Context) (string, time.Duration, error) {
			close(start)
			<-ctx.Done()
			return "", 0, ctx.Err()
		})
		done <- e
	}()
	<-start
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
	v, _, err := b.Get(context.Background(), "k", func(context.Context) (string, time.Duration, error) { return "new", time.Minute, nil })
	require.NoError(t, err)
	require.Equal(t, "new", v)
}
func TestWaitingReaderUsesStaleDuringOwnedRefresh(t *testing.T) {
	store := newStore(t)
	now := time.Now()
	a := New[string](time.Minute, 10, store).WithClock(func() time.Time { return now })
	b := New[string](time.Minute, 10, store).WithClock(func() time.Time { return now })
	_, _, err := a.Get(context.Background(), "k", func(context.Context) (string, time.Duration, error) { return "old", time.Second, nil })
	require.NoError(t, err)
	now = now.Add(2 * time.Second)
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		_, _, e := a.Get(context.Background(), "k", func(context.Context) (string, time.Duration, error) {
			close(started)
			<-release
			return "new", time.Minute, nil
		})
		if e != nil {
			t.Error(e)
		}
	}()
	<-started
	v, r, err := b.GetStaleWhileRefresh(context.Background(), "k", func(context.Context) (string, time.Duration, error) {
		t.Error("unexpected duplicate load")
		return "", 0, nil
	})
	require.NoError(t, err)
	require.Equal(t, "old", v)
	require.True(t, r.Stale)
	close(release)
	<-done
}
func TestUncachedZeroTTL(t *testing.T) {
	c := New[int](time.Minute, 10, newStore(t))
	calls := 0
	load := func(context.Context) (int, time.Duration, error) { calls++; return calls, 0, nil }
	_, _, e := c.Get(context.Background(), "k", load)
	require.NoError(t, e)
	v, _, e := c.Get(context.Background(), "k", load)
	require.NoError(t, e)
	require.Equal(t, 2, v)
}

func newStore(t testing.TB) *rediscache.Store {
	t.Helper()
	r := miniredis.RunT(t)
	s, err := rediscache.New("redis://"+r.Addr(), "test")
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}
