package positions

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type PoolConfig struct {
	Workers        int
	QueueSize      int
	RequestTimeout time.Duration
	CheckInterval  time.Duration
	StaleAfter     time.Duration
	BatchSize      int
}

type PoolMetrics struct {
	StaleChecks              metric.Int64Counter
	StaleTripsRowsRead       metric.Int64Counter
	PositionRequests         metric.Int64Counter
	PositionRequestsDuration metric.Float64Histogram
	QueueSize                metric.Int64UpDownCounter
	WorkersActive            metric.Int64UpDownCounter
}

type Pool struct {
	cfg       PoolConfig
	metrics   PoolMetrics
	gateway   pushGateway
	queue     *PriorityQueue
	scanner   *Scanner
	logger    *slog.Logger
	tracer    trace.Tracer
	ticker    Ticker
	scannerWg sync.WaitGroup
	workersWg sync.WaitGroup

	cancelScanner context.CancelFunc
	scannerCtx    context.Context
	workersCancel context.CancelFunc
	workersCtx    context.Context
}

func NewPool(
	repo staleTripsRepository,
	gateway pushGateway,
	cfg PoolConfig,
	metrics PoolMetrics,
	logger *slog.Logger,
	customTicker Ticker,
) *Pool {
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 100
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 1 * time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = 30 * time.Second
	}

	queue := NewPriorityQueue(cfg.QueueSize)

	scannerMetrics := ScannerMetrics{
		StaleChecks:             metrics.StaleChecks,
		StaleTripsRowsRead:      metrics.StaleTripsRowsRead,
		PositionRequests:        metrics.PositionRequests,
		PositionRequestsQueueSz: metrics.QueueSize,
	}

	scannerCfg := ScannerConfig{
		StaleAfter: cfg.StaleAfter,
		BatchSize:  cfg.BatchSize,
	}

	scanner := NewScanner(repo, queue, scannerCfg, scannerMetrics, logger)

	var ticker Ticker
	if customTicker != nil {
		ticker = customTicker
	} else {
		ticker = NewRealTicker(cfg.CheckInterval)
	}

	return &Pool{
		cfg:     cfg,
		metrics: metrics,
		gateway: gateway,
		queue:   queue,
		scanner: scanner,
		logger:  logger,
		tracer:  otel.GetTracerProvider().Tracer("positions-worker-pool"),
		ticker:  ticker,
	}
}

func (p *Pool) Start(ctx context.Context) {
	p.workersCtx, p.workersCancel = context.WithCancel(ctx)
	p.scannerCtx, p.cancelScanner = context.WithCancel(ctx)

	if p.metrics.QueueSize != nil {
		p.metrics.QueueSize.Add(ctx, 0)
	}
	if p.metrics.WorkersActive != nil {
		p.metrics.WorkersActive.Add(ctx, 0)
	}

	for i := 0; i < p.cfg.Workers; i++ {
		p.workersWg.Add(1)
		go p.workerLoop(p.workersCtx, &p.workersWg)
	}

	p.scannerWg.Add(1)
	go p.scanner.Run(p.scannerCtx, p.ticker, &p.scannerWg)
}

func (p *Pool) StopScanner() {
	if p.ticker != nil {
		p.ticker.Stop()
	}
	if p.cancelScanner != nil {
		p.cancelScanner()
	}
	p.scannerWg.Wait()
}

func (p *Pool) WaitWorkers(ctx context.Context) {
	p.queue.CloseInput()

	done := make(chan struct{})
	go func() {
		p.workersWg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		if p.workersCancel != nil {
			p.workersCancel()
		}
		<-done
	}
}

func (p *Pool) Cancel(ctx context.Context, tripID uuid.UUID) {
	if p.queue.Cancel(tripID) && p.metrics.QueueSize != nil {
		p.metrics.QueueSize.Add(ctx, -1)
	}
}

func (p *Pool) Queue() *PriorityQueue {
	return p.queue
}

func (p *Pool) Scanner() *Scanner {
	return p.scanner
}

func (p *Pool) workerLoop(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		task, ok := p.queue.Pop(ctx)
		if !ok {
			return
		}

		if p.metrics.QueueSize != nil {
			p.metrics.QueueSize.Add(ctx, -1)
		}

		p.processTask(ctx, task)
	}
}

func (p *Pool) processTask(parentCtx context.Context, task Task) {
	if !p.queue.IsActive(task.TripID) {
		return
	}
	defer p.queue.MarkDone(task.TripID)

	if p.metrics.WorkersActive != nil {
		p.metrics.WorkersActive.Add(parentCtx, 1)
		defer p.metrics.WorkersActive.Add(parentCtx, -1)
	}

	var spanOpts []trace.SpanStartOption
	spanOpts = append(spanOpts, trace.WithSpanKind(trace.SpanKindInternal))
	if task.SpanContext.IsValid() {
		spanOpts = append(spanOpts, trace.WithLinks(trace.Link{SpanContext: task.SpanContext}))
	}

	workerCtx, span := p.tracer.Start(parentCtx, "positions.worker.process", spanOpts...)
	defer span.End()

	reqCtx, reqCancel := context.WithTimeout(workerCtx, p.cfg.RequestTimeout)
	defer reqCancel()

	start := time.Now()
	err := p.gateway.RequestPosition(reqCtx, task.DriverID, task.TripID)
	durationSec := time.Since(start).Seconds()

	if err != nil {
		if p.metrics.PositionRequests != nil {
			p.metrics.PositionRequests.Add(workerCtx, 1, metric.WithAttributes(attribute.String("result", "error")))
		}
		if p.metrics.PositionRequestsDuration != nil {
			p.metrics.PositionRequestsDuration.Record(workerCtx, durationSec, metric.WithAttributes(attribute.String("result", "error")))
		}
		p.logger.Error("failed to request position from push service", slog.String("trip_id", task.TripID.String()), slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		if p.metrics.PositionRequests != nil {
			p.metrics.PositionRequests.Add(workerCtx, 1, metric.WithAttributes(attribute.String("result", "success")))
		}
		if p.metrics.PositionRequestsDuration != nil {
			p.metrics.PositionRequestsDuration.Record(workerCtx, durationSec, metric.WithAttributes(attribute.String("result", "success")))
		}
	}
}
