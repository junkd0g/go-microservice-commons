// Package jobrunner executes finite, replica-safe jobs without starting an HTTP server.
package jobrunner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"crypto/rand"
	"encoding/hex"
)

// ErrBusy indicates another worker holds the job lease.
var ErrBusy = errors.New("job is leased by another worker")

// ErrLeaseLost indicates this worker no longer owns the lease.
var ErrLeaseLost = errors.New("job lease lost")

// Spec is owned by the service, not by a central scheduler.
type Spec struct {
	Group   string
	Timeout time.Duration
	Run     func(context.Context, time.Time) error
}

// Store persists leases and retry completion records.
type Store interface {
	Acquire(context.Context, string, string, time.Duration) (bool, error)
	Renew(context.Context, string, string, time.Duration) (bool, error)
	Release(context.Context, string, string) error
	Begin(context.Context, string, time.Time) (time.Time, bool, error)
	Finish(context.Context, string, string) error
}

// RunEnv reads the process job identity and executes against the supplied store.
// The caller creates/configures the store, so this package has no database dependency.
func RunEnv(ctx context.Context, store Store, specs map[string]Spec) error {
	name, id := os.Getenv("JOB_NAME"), os.Getenv("JOB_RUN_ID")
	spec, ok := specs[name]
	if !ok {
		return fmt.Errorf("unknown JOB_NAME %q", name)
	}
	if id == "" {
		return errors.New("JOB_RUN_ID is required (stable across retries)")
	}
	now := time.Now().UTC()
	if date := os.Getenv("JOB_DATE"); date != "" {
		var err error
		now, err = time.Parse("2006-01-02", date)
		if err != nil {
			return fmt.Errorf("JOB_DATE: %w", err)
		}
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	if raw := os.Getenv("JOB_DELAY_SECONDS"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds < 0 || seconds > 59 {
			return errors.New("JOB_DELAY_SECONDS must be between 0 and 59")
		}
		timer := time.NewTimer(time.Duration(seconds) * time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	if setup, ok := store.(interface{ Ensure(context.Context) error }); ok {
		if err := setup.Ensure(ctx); err != nil {
			return err
		}
	}
	slog.Info("job_start", "name", name, "run_id", id)
	err := Execute(ctx, store, name, id, now, spec, 90*time.Second, 20*time.Second)
	slog.Info("job_finish", "name", name, "run_id", id, "error", fmt.Sprint(err))
	return err
}

// Execute keeps a renewable lease across replicas, waits for real completion,
// and persists the first attempt's business date so retries across midnight agree.
// Work must honor cancellation and be idempotent at its business-write boundary:
// a crash after a side effect but before Finish can cause that work to repeat.
func Execute(parent context.Context, store Store, name, id string, now time.Time, spec Spec, lease, heartbeat time.Duration) (err error) {
	if spec.Run == nil || spec.Timeout <= 0 || heartbeat <= 0 || lease <= heartbeat {
		return errors.New("invalid job specification")
	}
	ctx, cancel := context.WithTimeout(parent, spec.Timeout)
	defer cancel()
	group := spec.Group
	if group == "" {
		group = name
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Errorf("job lease owner: %w", err)
	}
	owner := hex.EncodeToString(random[:])
	acquired, err := store.Acquire(ctx, group, owner, lease)
	if err != nil {
		return err
	}
	if !acquired {
		return ErrBusy
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		err = errors.Join(err, store.Release(cleanup, group, owner))
	}()
	runKey := name + ":" + id
	date, complete, err := store.Begin(ctx, runKey, now)
	if err != nil || complete {
		return err
	}
	stopped := make(chan struct{})
	renewed := make(chan error, 1)
	go renewLease(ctx, cancel, store, group, owner, lease, heartbeat, stopped, renewed)
	err = invoke(ctx, date, spec.Run)
	close(stopped)
	err = errors.Join(err, <-renewed, ctx.Err())
	// Verify ownership even on failure. A stale worker must not overwrite the
	// completion of a replacement which acquired the expired lease.
	check, checked := context.WithTimeout(context.Background(), 5*time.Second)
	ok, ownershipErr := store.Renew(check, group, owner, lease)
	checked()
	if ownershipErr != nil || !ok {
		return errors.Join(err, ErrLeaseLost, ownershipErr)
	}
	cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	message := ""
	if err != nil {
		message = err.Error()
	}
	return errors.Join(err, store.Finish(cleanup, runKey, message))
}

func invoke(ctx context.Context, date time.Time, work func(context.Context, time.Time) error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("job panicked: %v", p)
		}
	}()
	return work(ctx, date)
}

func renewLease(ctx context.Context, cancel context.CancelFunc, store Store, group, owner string, lease, heartbeat time.Duration, stopped <-chan struct{}, renewed chan<- error) {
	ticker := time.NewTicker(heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-stopped:
			renewed <- nil
			return
		case <-ctx.Done():
			renewed <- ctx.Err()
			return
		case <-ticker.C:
			check, done := context.WithTimeout(ctx, heartbeat)
			ok, e := store.Renew(check, group, owner, lease)
			done()
			if e != nil || !ok {
				cancel()
				renewed <- errors.Join(ErrLeaseLost, e)
				return
			}
		}
	}
}
