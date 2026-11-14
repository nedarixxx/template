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
	"go.opentelemetry.io/otel/metric/noop"

	"github.com/nedarixxx/template/internal/model"
	"github.com/nedarixxx/template/internal/worker/positions"
)

type mockStaleRepo struct {
	trips      []model.StaleTrip
	queryCount int32
}

func (m *mockStaleRepo) GetStaleTrips(
	_ context.Context,
	_ time.Duration,
	limit int,
	lastStaleAt time.Time,
	lastID uuid.UUID,
) ([]model.StaleTrip, error) {
	atomic.AddInt32(&m.queryCount, 1)

	var startIndex int
	if !lastStaleAt.IsZero() && lastID != uuid.Nil {
		for i, t := range m.trips {
			if t.StaleAt.After(lastStaleAt) || (t.StaleAt.Equal(lastStaleAt) && t.ID.String() > lastID.String()) {
				startIndex = i
				break
			}
		}
	}

	if startIndex >= len(m.trips) {
		return nil, nil
	}

	endIndex := startIndex + limit
	if endIndex > len(m.trips) {
		endIndex = len(m.trips)
	}

	return m.trips[startIndex:endIndex], nil
}

func TestScanner_Batching(t *testing.T) {
	now := time.Now().UTC()
	const totalTrips = 250
	trips := make([]model.StaleTrip, 0, totalTrips)
	for i := 0; i < totalTrips; i++ {
		trips = append(trips, model.StaleTrip{
			ID:       uuid.New(),
			DriverID: uuid.New(),
			StaleAt:  now.Add(time.Duration(i) * time.Second),
		})
	}

	repo := &mockStaleRepo{trips: trips}
	queue := positions.NewPriorityQueue(500)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	meter := noop.NewMeterProvider().Meter("test")
	staleChecks, _ := meter.Int64Counter("tripgo.stale_checks")
	rowsRead, _ := meter.Int64Counter("tripgo.stale_trips.rows_read")
	posRequests, _ := meter.Int64Counter("tripgo.position_requests")
	queueSize, _ := meter.Int64UpDownCounter("tripgo.position_requests.queue.size")

	metrics := positions.ScannerMetrics{
		StaleChecks:             staleChecks,
		StaleTripsRowsRead:      rowsRead,
		PositionRequests:        posRequests,
		PositionRequestsQueueSz: queueSize,
	}

	cfg := positions.ScannerConfig{
		StaleAfter: 60 * time.Second,
		BatchSize:  100,
	}

	scanner := positions.NewScanner(repo, queue, cfg, metrics, logger)
	scanner.ScanOnce(context.Background())

	queries := atomic.LoadInt32(&repo.queryCount)
	if queries != 3 {
		t.Fatalf("expected 3 batch queries for 250 items with batch size 100, got %d", queries)
	}

	if queue.Size() != totalTrips {
		t.Fatalf("expected %d trips in queue, got %d", totalTrips, queue.Size())
	}
}

func TestScanner_QueueFullStopsBatching(t *testing.T) {
	now := time.Now().UTC()
	const totalTrips = 50
	trips := make([]model.StaleTrip, 0, totalTrips)
	for i := 0; i < totalTrips; i++ {
		trips = append(trips, model.StaleTrip{
			ID:       uuid.New(),
			DriverID: uuid.New(),
			StaleAt:  now.Add(time.Duration(i) * time.Second),
		})
	}

	repo := &mockStaleRepo{trips: trips}
	queue := positions.NewPriorityQueue(5)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	cfg := positions.ScannerConfig{
		StaleAfter: 60 * time.Second,
		BatchSize:  10,
	}

	scanner := positions.NewScanner(repo, queue, cfg, positions.ScannerMetrics{}, logger)
	scanner.ScanOnce(context.Background())

	if queue.Size() != 5 {
		t.Fatalf("expected queue size 5, got %d", queue.Size())
	}
}

type blockingRepo struct {
	started chan struct{}
	release chan struct{}
}

func (b *blockingRepo) GetStaleTrips(
	_ context.Context,
	_ time.Duration,
	_ int,
	_ time.Time,
	_ uuid.UUID,
) ([]model.StaleTrip, error) {
	select {
	case b.started <- struct{}{}:
	default:
	}
	<-b.release
	return nil, nil
}

func TestScanner_NonOverlappingTicks(t *testing.T) {
	repo := &blockingRepo{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	queue := positions.NewPriorityQueue(10)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	scanner := positions.NewScanner(repo, queue, positions.ScannerConfig{BatchSize: 10}, positions.ScannerMetrics{}, logger)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner.ScanOnce(context.Background())
	}()

	<-repo.started

	scanner.ScanOnce(context.Background())

	close(repo.release)
	wg.Wait()
	if queue.Size() != 0 {
		t.Fatalf("expected 0 items in queue, got %d", queue.Size())
	}
}

type errorRepo struct{}

func (e *errorRepo) GetStaleTrips(_ context.Context, _ time.Duration, _ int, _ time.Time, _ uuid.UUID) ([]model.StaleTrip, error) {
	return nil, errors.New("simulated database failure")
}

func TestScanner_RepoErrorDoesNotPanic(t *testing.T) {
	queue := positions.NewPriorityQueue(10)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	scanner := positions.NewScanner(&errorRepo{}, queue, positions.ScannerConfig{BatchSize: 10}, positions.ScannerMetrics{}, logger)

	scanner.ScanOnce(context.Background())

	if queue.Size() != 0 {
		t.Fatalf("expected 0 items in queue on repo error, got %d", queue.Size())
	}
}

func TestScanner_ContextCancelled(t *testing.T) {
	queue := positions.NewPriorityQueue(10)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	scanner := positions.NewScanner(&mockStaleRepo{}, queue, positions.ScannerConfig{BatchSize: 10}, positions.ScannerMetrics{}, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	scanner.ScanOnce(ctx)

	if queue.Size() != 0 {
		t.Fatalf("expected 0 items in queue on cancelled context, got %d", queue.Size())
	}
}
