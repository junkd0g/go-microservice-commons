// Package rediscache implements cache storage and atomic leases in Redis/Valkey.
package rediscache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store struct {
	client *redis.Client
	prefix string
}

func New(rawURL, prefix string) (*Store, error) {
	o, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, errors.New("invalid shared cache URL")
	}
	o.ContextTimeoutEnabled = true
	o.DialTimeout = 3 * time.Second
	o.ReadTimeout = 2 * time.Second
	o.WriteTimeout = 2 * time.Second
	return &Store{redis.NewClient(o), prefix}, nil
}
func (s *Store) Close() error                   { return s.client.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.client.Ping(ctx).Err() }
func (s *Store) keys(ns, key string) (string, string, string) {
	h := sha256.Sum256([]byte(ns))
	bucket := s.prefix + ":{" + hex.EncodeToString(h[:]) + "}"
	h = sha256.Sum256([]byte(key))
	k := bucket + ":" + hex.EncodeToString(h[:])
	return k, k + ":lease", bucket + ":entries"
}
func (s *Store) Load(ctx context.Context, ns, key string) ([]byte, error) {
	k, _, _ := s.keys(ns, key)
	b, e := s.client.Get(ctx, k).Bytes()
	if errors.Is(e, redis.Nil) {
		return nil, nil
	}
	return b, e
}
func (s *Store) Acquire(ctx context.Context, ns, key, owner string, ttl time.Duration) (bool, error) {
	_, k, _ := s.keys(ns, key)
	return s.client.SetNX(ctx, k, owner, ttl).Result()
}

var renew = redis.NewScript(`if redis.call('GET',KEYS[1])==ARGV[1] then return redis.call('PEXPIRE',KEYS[1],ARGV[2]) end return 0`)
var release = redis.NewScript(`if redis.call('GET',KEYS[1])==ARGV[1] then return redis.call('DEL',KEYS[1]) end return 0`)
var publish = redis.NewScript(`
if redis.call('GET',KEYS[2]) ~= ARGV[1] then return 0 end
if tonumber(ARGV[3])>0 then redis.call('SET',KEYS[1],ARGV[2],'PX',ARGV[3]) else redis.call('SET',KEYS[1],ARGV[2]) end
local t=redis.call('TIME');local now=t[1]*1000+math.floor(t[2]/1000)
redis.call('ZADD',KEYS[3],now,KEYS[1])
local over=redis.call('ZCARD',KEYS[3])-tonumber(ARGV[4])
if tonumber(ARGV[4])>0 and over>0 then
 local keys=redis.call('ZRANGE',KEYS[3],0,math.min(over,100)-1)
 for _,k in ipairs(keys) do redis.call('DEL',k);redis.call('ZREM',KEYS[3],k) end
end
return 1`)

func (s *Store) Renew(ctx context.Context, ns, key, owner string, ttl time.Duration) (bool, error) {
	_, k, _ := s.keys(ns, key)
	n, e := renew.Run(ctx, s.client, []string{k}, owner, ttl.Milliseconds()).Int()
	return n == 1, e
}
func (s *Store) Release(ctx context.Context, ns, key, owner string) error {
	_, k, _ := s.keys(ns, key)
	return release.Run(ctx, s.client, []string{k}, owner).Err()
}
func (s *Store) Publish(ctx context.Context, ns, key, owner string, value []byte, keep time.Duration, max int) (bool, error) {
	k, l, z := s.keys(ns, key)
	n, e := publish.Run(ctx, s.client, []string{k, l, z}, owner, value, keep.Milliseconds(), max).Int()
	return n == 1, e
}
