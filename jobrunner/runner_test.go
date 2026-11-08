package jobrunner

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type record struct {
	date     time.Time
	complete bool
}
type memory struct {
	mu      sync.Mutex
	owner   string
	records map[string]record
	lost    bool
}

func (m *memory) Acquire(_ context.Context, _, owner string, _ time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.owner != "" {
		return false, nil
	}
	m.owner = owner
	return true, nil
}
func (m *memory) Renew(_ context.Context, _, owner string, _ time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return !m.lost && m.owner == owner, nil
}
func (m *memory) Release(_ context.Context, _, owner string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.owner == owner {
		m.owner = ""
	}
	return nil
}
func (m *memory) Begin(_ context.Context, key string, date time.Time) (time.Time, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.records == nil {
		m.records = map[string]record{}
	}
	r, ok := m.records[key]
	if !ok {
		r.date = date
		m.records[key] = r
	}
	return r.date, r.complete, nil
}
func (m *memory) Finish(_ context.Context, key, msg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.records[key]
	r.complete = msg == ""
	m.records[key] = r
	return nil
}
func execute(ctx context.Context, m *memory, id string, day time.Time, f func(context.Context, time.Time) error) error {
	return Execute(ctx, m, "test", id, day, Spec{Timeout: time.Second, Run: f}, 50*time.Millisecond, 5*time.Millisecond)
}
func TestCompletionDeduplicatesAndFailureRetriesOriginalDay(t *testing.T) {
	m := &memory{}
	day := time.Date(2026, 9, 30, 23, 59, 0, 0, time.UTC)
	calls := 0
	work := func(_ context.Context, d time.Time) error {
		calls++
		if !d.Equal(day) {
			t.Errorf("date moved on retry: %s", d)
		}
		if calls == 1 {
			return errors.New("retry")
		}
		return nil
	}
	if execute(context.Background(), m, "one", day, work) == nil {
		t.Fatal("failure hidden")
	}
	if err := execute(context.Background(), m, "one", day.Add(24*time.Hour), work); err != nil {
		t.Fatal(err)
	}
	if err := execute(context.Background(), m, "one", day.Add(48*time.Hour), work); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("work ran %d times", calls)
	}
}
func TestConcurrentReplicaCannotExecute(t *testing.T) {
	m := &memory{}
	started := make(chan struct{})
	finish := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		result <- execute(context.Background(), m, "one", time.Now(), func(context.Context, time.Time) error { close(started); <-finish; return nil })
	}()
	<-started
	err := execute(context.Background(), m, "two", time.Now(), func(context.Context, time.Time) error { t.Error("duplicate executed"); return nil })
	if !errors.Is(err, ErrBusy) {
		t.Errorf("want busy, got %v", err)
	}
	close(finish)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}
func TestLeaseLossCancelsWorkAndDoesNotRecordSuccess(t *testing.T) {
	m := &memory{}
	err := execute(context.Background(), m, "one", time.Now(), func(ctx context.Context, _ time.Time) error {
		m.mu.Lock()
		m.lost = true
		m.mu.Unlock()
		<-ctx.Done()
		return ctx.Err()
	})
	if !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("want lease lost, got %v", err)
	}
	if m.records["test:one"].complete {
		t.Fatal("lost lease recorded success")
	}
}
func TestPanicAndCancellationAreFailures(t *testing.T) {
	for _, panicWork := range []bool{false, true} {
		m := &memory{}
		ctx, cancel := context.WithCancel(context.Background())
		err := execute(ctx, m, "one", time.Now(), func(context.Context, time.Time) error {
			if panicWork {
				panic("boom")
			}
			cancel()
			return nil
		})
		cancel()
		if err == nil {
			t.Fatal("failure hidden")
		}
		if m.records["test:one"].complete {
			t.Fatal("failure completed")
		}
	}
}
