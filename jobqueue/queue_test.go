package jobqueue_test

import (
	"context"
	"github.com/alicebob/miniredis/v2"
	"github.com/junkd0g/go-microservice-commons/jobqueue"
	"github.com/junkd0g/go-microservice-commons/redisqueue"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type job struct{ Kind string }

func (j job) JobKind() string { return j.Kind }

type quiet struct{}

func (quiet) Info(context.Context, string, ...map[string]interface{})  {}
func (quiet) Error(context.Context, string, ...map[string]interface{}) {}
func TestAcceptedWorkOutlivesSubmitterButFollowsWorkerLifecycle(t *testing.T) {
	r := miniredis.RunT(t)
	s, e := redisqueue.New[job]("redis://"+r.Addr(), "jobs-test")
	require.NoError(t, e)
	defer s.Close()
	started, stopped := make(chan struct{}), make(chan struct{})
	q := jobqueue.Queue[job]{Store: s, Log: quiet{}, Run: func(ctx context.Context, j job) error { close(started); <-ctx.Done(); return ctx.Err() }}
	request, done := context.WithCancel(context.Background())
	require.NoError(t, q.Submit(request, job{"work"}))
	done()
	worker, cancel := context.WithCancel(context.Background())
	go func() { q.Serve(worker); close(stopped) }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("accepted job was lost")
	}
	cancel()
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not follow cancellation")
	}
	r.SetTime(time.Now().Add(2 * time.Minute))
	_, ok, e := s.Take(context.Background(), "replacement", time.Minute)
	require.NoError(t, e)
	require.True(t, ok, "interrupted work must remain recoverable")
}
