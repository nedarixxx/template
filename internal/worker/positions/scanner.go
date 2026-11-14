package positions

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type ScannerMetrics struct {
	StaleChecks             metric.Int64Counter
	StaleTripsRowsRead      metric.Int64Counter
	PositionRequests        metric.Int64Counter
	PositionRequestsQueueSz metric.Int64UpDownCounter
}

type ScannerConfig struct {
	StaleAfter time.Duration
	BatchSize  int
}

type Scanner struct {
	repo    staleTripsRepository
	queue   *PriorityQueue
	cfg     ScannerConfig
	metrics ScannerMetrics
	logger  *slog.Logger
	tracer  trace.Tracer
	running int32
}

func NewScanner(
	repo staleTripsRepository,
	queue *PriorityQueue,
	cfg ScannerConfig,
	metrics ScannerMetrics,
	logger *slog.Logger,
) *Scanner {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	return &Scanner{
		repo:    repo,
		queue:   queue,
		cfg:     cfg,
		metrics: metrics,
		logger:  logger,
		tracer:  otel.GetTracerProvider().Tracer("positions-scanner"),
	}
}

func (s *Scanner) Run(ctx context.Context, ticker Ticker, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C():
			s.ScanOnce(ctx)
		}
	}
}

func (s *Scanner) ScanOnce(ctx context.Context) {
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&s.running, 0)

	ctx, span := s.tracer.Start(ctx, "positions.scanner.tick",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	spanCtx := span.SpanContext()

	var (
		lastStaleAt time.Time
		lastID      uuid.UUID
	)

	for {
		if ctx.Err() != nil {
			return
		}

		trips, err := s.repo.GetStaleTrips(ctx, s.cfg.StaleAfter, s.cfg.BatchSize, lastStaleAt, lastID)
		if err != nil {
			s.logger.Error("failed to get stale trips", slog.Any("error", err))
			return
		}

		if len(trips) == 0 {
			return
		}

		if s.metrics.StaleTripsRowsRead != nil {
			s.metrics.StaleTripsRowsRead.Add(ctx, int64(len(trips)))
		}

		queueFull := false
		for _, trip := range trips {
			task := Task{
				TripID:      trip.ID,
				DriverID:    trip.DriverID,
				StaleAt:     trip.StaleAt,
				SpanContext: spanCtx,
			}

			res, evicted := s.queue.Push(task)
			switch res {
			case ResultQueued:
				if s.metrics.StaleChecks != nil {
					s.metrics.StaleChecks.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "queued")))
				}
				if s.metrics.PositionRequestsQueueSz != nil {
					s.metrics.PositionRequestsQueueSz.Add(ctx, 1)
				}
			case ResultDuplicate:
				if s.metrics.StaleChecks != nil {
					s.metrics.StaleChecks.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "duplicate")))
				}
			case ResultDropped:
				queueFull = true
				if s.metrics.StaleChecks != nil {
					s.metrics.StaleChecks.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "dropped")))
				}
				if s.metrics.PositionRequests != nil {
					s.metrics.PositionRequests.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "dropped")))
				}
				s.logger.Warn("position request dropped: queue full", slog.String("trip_id", task.TripID.String()))
			case ResultEvicted:
				if s.metrics.StaleChecks != nil {
					s.metrics.StaleChecks.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "dropped")))
				}
				if s.metrics.PositionRequests != nil {
					s.metrics.PositionRequests.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "dropped")))
				}
				if evicted != nil {
					s.logger.Warn("position request dropped: queue full", slog.String("trip_id", evicted.TripID.String()))
				}
				if s.metrics.StaleChecks != nil {
					s.metrics.StaleChecks.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "queued")))
				}
			}

			if queueFull {
				break
			}
		}

		if queueFull || len(trips) < s.cfg.BatchSize {
			return
		}

		lastTrip := trips[len(trips)-1]
		lastStaleAt = lastTrip.StaleAt
		lastID = lastTrip.ID
	}
}
