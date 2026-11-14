package positions_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/goleak"

	"github.com/nedarixxx/template/internal/model"
	"github.com/nedarixxx/template/internal/worker/positions"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type fakePushGateway struct {
	calls       int32
	shouldError int32
	calledTrips sync.Map
	onCall      func()
}

func (f *fakePushGateway) RequestPosition(_ context.Context, _, tripID uuid.UUID) error {
	atomic.AddInt32(&f.calls, 1)
	f.calledTrips.Store(tripID, true)
	if f.onCall != nil {
		f.onCall()
	}

	if atomic.LoadInt32(&f.shouldError) == 1 {
		return errors.New("simulated push error")
	}
	return nil
}

type fakeTicker struct {
	c    chan time.Time
	stop chan struct{}
}

func newFakeTicker() *fakeTicker {
	return &fakeTicker{
		c:    make(chan time.Time, 1),
		stop: make(chan struct{}),
	}
}

func (f *fakeTicker) C() <-chan time.Time {
	return f.c
}

func (f *fakeTicker) Stop() {
	select {
	case <-f.stop:
	default:
		close(f.stop)
	}
}

func (f *fakeTicker) Tick() {
	select {
	case f.c <- time.Now():
	default:
	}
}

func TestPool_ProcessesTasksWithWorkers(t *testing.T) {
	trip1 := uuid.New()
	trip2 := uuid.New()
	trip3 := uuid.New()
	now := time.Now().UTC()

	repo := &mockStaleRepo{
		trips: []model.StaleTrip{
			{ID: trip1, DriverID: uuid.New(), StaleAt: now.Add(-3 * time.Minute)},
			{ID: trip2, DriverID: uuid.New(), StaleAt: now.Add(-2 * time.Minute)},
			{ID: trip3, DriverID: uuid.New(), StaleAt: now.Add(-1 * time.Minute)},
		},
	}
	gateway := &fakePushGateway{}
	ticker := newFakeTicker()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	cfg := positions.PoolConfig{
		Workers:        4,
		QueueSize:      10,
		RequestTimeout: 50 * time.Millisecond,
		CheckInterval:  10 * time.Second,
		StaleAfter:     30 * time.Second,
		BatchSize:      10,
	}

	var wg sync.WaitGroup
	wg.Add(3)
	gateway.onCall = func() {
		wg.Done()
	}

	pool := positions.NewPool(repo, gateway, cfg, positions.PoolMetrics{}, logger, ticker)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)

	ticker.Tick()

	wg.Wait()

	pool.StopScanner()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer shutdownCancel()
	pool.WaitWorkers(shutdownCtx)

	calls := atomic.LoadInt32(&gateway.calls)
	if calls != 3 {
		t.Fatalf("expected 3 push calls, got %d", calls)
	}
}

func TestPool_PushErrorDoesNotCrashWorker(t *testing.T) {
	trip1 := uuid.New()
	trip2 := uuid.New()
	now := time.Now().UTC()

	repo := &mockStaleRepo{
		trips: []model.StaleTrip{
			{ID: trip1, DriverID: uuid.New(), StaleAt: now.Add(-2 * time.Minute)},
			{ID: trip2, DriverID: uuid.New(), StaleAt: now.Add(-1 * time.Minute)},
		},
	}
	gateway := &fakePushGateway{shouldError: 1}
	ticker := newFakeTicker()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	cfg := positions.PoolConfig{
		Workers:        1,
		QueueSize:      10,
		RequestTimeout: 50 * time.Millisecond,
		CheckInterval:  10 * time.Second,
		StaleAfter:     30 * time.Second,
		BatchSize:      10,
	}

	var wg sync.WaitGroup
	wg.Add(2)
	gateway.onCall = func() {
		wg.Done()
	}

	pool := positions.NewPool(repo, gateway, cfg, positions.PoolMetrics{}, logger, ticker)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)
	ticker.Tick()

	wg.Wait()

	pool.StopScanner()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer shutdownCancel()
	pool.WaitWorkers(shutdownCtx)

	calls := atomic.LoadInt32(&gateway.calls)
	if calls != 2 {
		t.Fatalf("expected 2 push attempts despite errors, got %d", calls)
	}
}

func TestPool_CancelPreventsPush(t *testing.T) {
	tripID := uuid.New()
	now := time.Now().UTC()

	repo := &mockStaleRepo{}
	gateway := &fakePushGateway{}
	ticker := newFakeTicker()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	cfg := positions.PoolConfig{
		Workers:        1,
		QueueSize:      10,
		RequestTimeout: 50 * time.Millisecond,
		CheckInterval:  10 * time.Second,
		StaleAfter:     30 * time.Second,
		BatchSize:      10,
	}

	pool := positions.NewPool(repo, gateway, cfg, positions.PoolMetrics{}, logger, ticker)

	pool.Queue().Push(positions.Task{
		TripID:   tripID,
		DriverID: uuid.New(),
		StaleAt:  now,
	})

	pool.Cancel(context.Background(), tripID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool.Start(ctx)

	pool.StopScanner()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer shutdownCancel()
	pool.WaitWorkers(shutdownCtx)

	calls := atomic.LoadInt32(&gateway.calls)
	if calls != 0 {
		t.Fatalf("expected 0 push calls for cancelled trip, got %d", calls)
	}
}
