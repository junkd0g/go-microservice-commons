// Package jobqueue runs durable editorial generation commands under renewable leases.
package jobqueue

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"time"
)

var ErrBusy = errors.New("job already in progress")

type Job interface{ JobKind() string }
type Store[T Job] interface {
	Enqueue(context.Context, T) (bool, error)
	Take(context.Context, string, time.Duration) (T, bool, error)
	Renew(context.Context, string, string, time.Duration) (bool, error)
	Complete(context.Context, string, string, string) error
}
type Logger interface {
	Error(context.Context, string, ...map[string]interface{})
	Info(context.Context, string, ...map[string]interface{})
}
type Queue[T Job] struct {
	Store Store[T]
	Run   func(context.Context, T) error
	Log   Logger
}

func (q *Queue[T]) Submit(ctx context.Context, job T) error {
	ok, e := q.Store.Enqueue(ctx, job)
	if e != nil {
		return e
	}
	if !ok {
		return ErrBusy
	}
	return nil
}

// Serve belongs to the application lifecycle, not an HTTP request. Pending jobs
// remain in Redis when a worker exits and become claimable after its lease expires.
func (q *Queue[T]) Serve(ctx context.Context) {
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		owner := uuid.NewString()
		job, ok, e := q.Store.Take(ctx, owner, time.Minute)
		if e != nil {
			q.Log.Error(ctx, "background job claim failed", map[string]interface{}{"error": e.Error()})
		} else if ok {
			q.execute(ctx, owner, job)
		}
		timer.Reset(time.Second)
	}
}
func (q *Queue[T]) execute(parent context.Context, owner string, job T) {
	ctx, cancel := context.WithTimeout(parent, 25*time.Minute)
	defer cancel()
	done := make(chan struct{})
	renewed := make(chan error, 1)
	go func() {
		defer close(renewed)
		tick := time.NewTicker(20 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-tick.C:
				ok, e := q.Store.Renew(ctx, job.JobKind(), owner, time.Minute)
				if e != nil || !ok {
					if e == nil {
						e = errors.New("generation lease lost")
					}
					renewed <- e
					cancel()
					return
				}
			}
		}
	}()
	err := q.Run(ctx, job)
	close(done)
	for e := range renewed {
		err = errors.Join(err, e)
	}
	// Cancellation on shutdown leaves the durable job available for another worker.
	if parent.Err() != nil {
		return
	}
	message := ""
	if err != nil {
		message = err.Error()
		q.Log.Error(ctx, "background job failed", map[string]interface{}{"job": job.JobKind(), "error": message})
	}
	finish, stop := context.WithTimeout(parent, 5*time.Second)
	defer stop()
	if e := q.Store.Complete(finish, job.JobKind(), owner, message); e != nil {
		q.Log.Error(parent, "background job completion failed", map[string]interface{}{"job": job.JobKind(), "error": e.Error()})
	}
}
