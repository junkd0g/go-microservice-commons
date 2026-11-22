// Package redisqueue owns Redis commands for durable generation work.
package redisqueue

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/junkd0g/go-microservice-commons/jobqueue"
	"github.com/redis/go-redis/v9"
	"time"
)

type Store[T jobqueue.Job] struct {
	client      *redis.Client
	due, prefix string
}

func New[T jobqueue.Job](url, namespace string) (*Store[T], error) {
	o, e := redis.ParseURL(url)
	if e != nil {
		return nil, errors.New("invalid generation Redis URL")
	}
	o.ContextTimeoutEnabled = true
	o.DialTimeout = 3 * time.Second
	o.ReadTimeout = 2 * time.Second
	o.WriteTimeout = 2 * time.Second
	return &Store[T]{client: redis.NewClient(o), due: "{" + namespace + "}:due", prefix: "{" + namespace + "}:job:"}, nil
}
func (s *Store[T]) Close() error                   { return s.client.Close() }
func (s *Store[T]) Ping(ctx context.Context) error { return s.client.Ping(ctx).Err() }

var enqueue = redis.NewScript(`local status=redis.call('HGET',KEYS[2],'status');if status=='queued' or status=='running' then return 0 end;redis.call('HSET',KEYS[2],'status','queued','payload',ARGV[1],'owner','');redis.call('PERSIST',KEYS[2]);redis.call('ZADD',KEYS[1],0,KEYS[2]);return 1`)
var take = redis.NewScript(`local t=redis.call('TIME');local now=t[1]*1000+math.floor(t[2]/1000);local found=redis.call('ZRANGEBYSCORE',KEYS[1],'-inf',now,'LIMIT',0,1);if #found==0 then return {} end;local key=found[1];redis.call('HSET',key,'owner',ARGV[1],'status','running');redis.call('ZADD',KEYS[1],now+tonumber(ARGV[2]),key);return {redis.call('HGET',key,'payload')}`)
var renew = redis.NewScript(`if redis.call('HGET',KEYS[2],'owner')~=ARGV[1] or redis.call('HGET',KEYS[2],'status')~='running' then return 0 end;local t=redis.call('TIME');local now=t[1]*1000+math.floor(t[2]/1000);local expires=redis.call('ZSCORE',KEYS[1],KEYS[2]);if not expires or tonumber(expires)<=now then return 0 end;redis.call('ZADD',KEYS[1],now+tonumber(ARGV[2]),KEYS[2]);return 1`)
var complete = redis.NewScript(`if redis.call('HGET',KEYS[2],'owner')~=ARGV[1] then return 0 end;local t=redis.call('TIME');local now=t[1]*1000+math.floor(t[2]/1000);local expires=redis.call('ZSCORE',KEYS[1],KEYS[2]);if not expires or tonumber(expires)<=now then return 0 end;redis.call('ZREM',KEYS[1],KEYS[2]);redis.call('HSET',KEYS[2],'status',ARGV[2],'error',ARGV[3]);redis.call('EXPIRE',KEYS[2],604800);return 1`)

func (s *Store[T]) Enqueue(ctx context.Context, j T) (bool, error) {
	b, e := json.Marshal(j)
	if e != nil {
		return false, e
	}
	n, e := enqueue.Run(ctx, s.client, []string{s.due, s.prefix + j.JobKind()}, b).Int()
	return n == 1, e
}
func (s *Store[T]) Take(ctx context.Context, owner string, ttl time.Duration) (T, bool, error) {
	var j T
	a, e := take.Run(ctx, s.client, []string{s.due}, owner, ttl.Milliseconds()).StringSlice()
	if e != nil || len(a) == 0 {
		return j, false, e
	}
	e = json.Unmarshal([]byte(a[0]), &j)
	return j, e == nil, e
}
func (s *Store[T]) Renew(ctx context.Context, kind, owner string, ttl time.Duration) (bool, error) {
	n, e := renew.Run(ctx, s.client, []string{s.due, s.prefix + kind}, owner, ttl.Milliseconds()).Int()
	return n == 1, e
}
func (s *Store[T]) Complete(ctx context.Context, kind, owner, message string) error {
	status := "completed"
	if message != "" {
		status = "failed"
	}
	n, e := complete.Run(ctx, s.client, []string{s.due, s.prefix + kind}, owner, status, message).Int()
	if e == nil && n != 1 {
		return errors.New("generation completion lease lost")
	}
	return e
}
