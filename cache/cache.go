// Package cache owns shared read-through cache policy. Storage and lease operations
// are injected; request work never outlives its caller.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"
)

// Store implements atomic, owner-checked shared cache operations.
type Store interface {
	Load(context.Context, string, string) ([]byte, error)
	Acquire(context.Context, string, string, string, time.Duration) (bool, error)
	Renew(context.Context, string, string, string, time.Duration) (bool, error)
	Publish(context.Context, string, string, string, []byte, time.Duration, int) (bool, error)
	Release(context.Context, string, string, string) error
}

type Loader[V any] func(context.Context) (V, time.Duration, error)
type Result struct {
	Hit      bool
	Stale    bool
	StoredAt time.Time
}
type entry[V any] struct {
	Value      V
	StoredAt   time.Time
	Expires    time.Time
	StaleUntil time.Time
}
type Cache[V any] struct {
	store        Store
	namespace    string
	staleIfError time.Duration
	maxEntries   int
	now          func() time.Time
}

// New builds policy; an omitted store explicitly disables caching (useful for
// isolated consumers). Applications inject shared persistence at composition time.
func New[V any](stale time.Duration, maxEntries int, stores ...Store) *Cache[V] {
	c := &Cache[V]{namespace: reflect.TypeOf((*V)(nil)).Elem().String(), staleIfError: stale, maxEntries: maxEntries, now: time.Now}
	if len(stores) > 0 {
		c.store = stores[0]
	}
	return c
}
func (c *Cache[V]) WithNamespace(n string) *Cache[V]         { c.namespace = n; return c }
func (c *Cache[V]) WithClock(now func() time.Time) *Cache[V] { c.now = now; return c }
func (c *Cache[V]) Get(ctx context.Context, key string, load Loader[V]) (V, Result, error) {
	return c.get(ctx, key, load, false)
}

// GetStaleWhileRefresh lets followers reuse a stale snapshot while another
// request owns the refresh. The owner refreshes within its request context.
func (c *Cache[V]) GetStaleWhileRefresh(ctx context.Context, key string, load Loader[V]) (V, Result, error) {
	return c.get(ctx, key, load, true)
}
func (c *Cache[V]) get(ctx context.Context, key string, load Loader[V], staleWhileRefresh bool) (V, Result, error) {
	var zero V
	if err := ctx.Err(); err != nil {
		return zero, Result{}, err
	}
	if c.store == nil {
		v, _, err := load(ctx)
		return v, Result{StoredAt: c.now()}, err
	}
	owner := uuid.NewString()
	const lease = 30 * time.Second
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return zero, Result{}, ctx.Err()
		case <-timer.C:
		}
		raw, err := c.store.Load(ctx, c.namespace, key)
		if err != nil {
			return zero, Result{}, fmt.Errorf("cache load: %w", err)
		}
		var old entry[V]
		found := len(raw) > 0
		if found {
			if err = json.Unmarshal(raw, &old); err != nil {
				return zero, Result{}, fmt.Errorf("cache decode: %w", err)
			}
			if c.now().Before(old.Expires) {
				return old.Value, Result{Hit: true, StoredAt: old.StoredAt}, nil
			}
		}
		acquired, err := c.store.Acquire(ctx, c.namespace, key, owner, lease)
		if err != nil {
			return zero, Result{}, err
		}
		if acquired {
			// A previous owner may have published between our read and acquisition.
			latest, e := c.store.Load(ctx, c.namespace, key)
			if e != nil {
				c.release(c.namespace, key, owner)
				return zero, Result{}, e
			}
			if len(latest) > 0 {
				var after entry[V]
				if e = json.Unmarshal(latest, &after); e != nil {
					c.release(c.namespace, key, owner)
					return zero, Result{}, e
				}
				if c.now().Before(after.Expires) {
					c.release(c.namespace, key, owner)
					return after.Value, Result{Hit: true, StoredAt: after.StoredAt}, nil
				}
			}
			return c.fill(ctx, key, owner, lease, old, found, load)
		}
		if staleWhileRefresh && found && c.stale(old) {
			return old.Value, Result{Hit: true, Stale: true, StoredAt: old.StoredAt}, nil
		}
		timer.Reset(25 * time.Millisecond)
	}
}
func (c *Cache[V]) stale(e entry[V]) bool { return c.staleIfError < 0 || c.now().Before(e.StaleUntil) }
func (c *Cache[V]) release(ns, key, owner string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = c.store.Release(ctx, ns, key, owner)
}
func (c *Cache[V]) fill(ctx context.Context, key, owner string, lease time.Duration, old entry[V], found bool, load Loader[V]) (V, Result, error) {
	var zero V
	work, cancel := context.WithCancel(ctx)
	defer cancel()
	defer c.release(c.namespace, key, owner)
	done := make(chan struct{})
	renewed := make(chan error, 1)
	go func() {
		defer close(renewed)
		t := time.NewTicker(lease / 3)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-work.Done():
				return
			case <-t.C:
				ok, err := c.store.Renew(work, c.namespace, key, owner, lease)
				if err != nil || !ok {
					if err == nil {
						err = errors.New("cache lease lost")
					}
					renewed <- err
					cancel()
					return
				}
			}
		}
	}()
	value, ttl, err := load(work)
	close(done)
	for renewalErr := range renewed {
		err = errors.Join(err, renewalErr)
	}
	if ctx.Err() != nil {
		return zero, Result{}, ctx.Err()
	}
	if err != nil {
		if found && c.stale(old) {
			return old.Value, Result{Stale: true, StoredAt: old.StoredAt}, nil
		}
		return zero, Result{}, err
	}
	now := c.now()
	if ttl > 0 {
		keep := ttl + c.staleIfError
		if c.staleIfError < 0 {
			keep = 0
		}
		record := entry[V]{Value: value, StoredAt: now, Expires: now.Add(ttl), StaleUntil: now.Add(keep)}
		raw, e := json.Marshal(record)
		if e != nil {
			return zero, Result{}, e
		}
		ok, e := c.store.Publish(ctx, c.namespace, key, owner, raw, keep, c.maxEntries)
		if e != nil {
			return zero, Result{}, e
		}
		if !ok {
			return zero, Result{}, errors.New("cache lease lost before publish")
		}
	}
	return value, Result{StoredAt: now}, nil
}
