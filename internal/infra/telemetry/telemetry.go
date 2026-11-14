package telemetry

import (
	"context"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Telemetry struct {
	TripsCreatedCounter   metric.Int64Counter
	TripsCompletedCounter metric.Int64Counter

	HTTPRequestDuration metric.Float64Histogram
	HTTPActiveRequests  metric.Int64UpDownCounter

	PositionsSavedCounter         metric.Int64Counter
	StaleChecksCounter            metric.Int64Counter
	StaleTripsRowsReadCounter     metric.Int64Counter
	PositionRequestsQueueSize     metric.Int64UpDownCounter
	PositionRequestsWorkersActive metric.Int64UpDownCounter
	PositionRequestsCounter       metric.Int64Counter
	PositionRequestsDuration      metric.Float64Histogram
}

func Init(ctx context.Context, serviceName string) (*Telemetry, func(context.Context) error, error) {
	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, nil, err
	}

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:22318"
	}

	protocol := strings.ToLower(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"))
	isInsecure := !strings.HasPrefix(endpoint, "https://")
	cleanEndpoint := strings.TrimPrefix(endpoint, "http://")
	cleanEndpoint = strings.TrimPrefix(cleanEndpoint, "https://")

	var (
		traceExporter  sdktrace.SpanExporter
		metricExporter sdkmetric.Exporter
	)

	if protocol == "grpc" {
		grpcOpts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(cleanEndpoint),
		}
		if isInsecure {
			grpcOpts = append(grpcOpts,
				otlptracegrpc.WithInsecure(),
				otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
			)
		}
		traceExporter, err = otlptracegrpc.New(ctx, grpcOpts...)
		if err != nil {
			return nil, nil, err
		}

		mGrpcOpts := []otlpmetricgrpc.Option{
			otlpmetricgrpc.WithEndpoint(cleanEndpoint),
		}
		if isInsecure {
			mGrpcOpts = append(mGrpcOpts, otlpmetricgrpc.WithInsecure())
		}
		metricExporter, err = otlpmetricgrpc.New(ctx, mGrpcOpts...)
		if err != nil {
			return nil, nil, err
		}
	} else {
		httpOpts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(cleanEndpoint),
		}
		if isInsecure {
			httpOpts = append(httpOpts, otlptracehttp.WithInsecure())
		}
		traceExporter, err = otlptracehttp.New(ctx, httpOpts...)
		if err != nil {
			return nil, nil, err
		}

		mHttpOpts := []otlpmetrichttp.Option{
			otlpmetrichttp.WithEndpoint(cleanEndpoint),
		}
		if isInsecure {
			mHttpOpts = append(mHttpOpts, otlpmetrichttp.WithInsecure())
		}
		metricExporter, err = otlpmetrichttp.New(ctx, mHttpOpts...)
		if err != nil {
			return nil, nil, err
		}
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(1*time.Second))),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	meter := mp.Meter(serviceName)

	tripsCreated, _ := meter.Int64Counter("tripgo.trips.created",
		metric.WithDescription("Total number of trips created"),
		metric.WithUnit("{trips}"),
	)

	tripsCompleted, _ := meter.Int64Counter("tripgo.trips.completed",
		metric.WithDescription("Total number of trips completed"),
		metric.WithUnit("{trips}"),
	)

	httpDuration, _ := meter.Float64Histogram("http.server.request.duration",
		metric.WithDescription("Duration of HTTP requests in seconds"),
		metric.WithUnit("s"),
	)

	activeRequests, _ := meter.Int64UpDownCounter("http.server.active_requests",
		metric.WithDescription("Number of active HTTP requests"),
		metric.WithUnit("{requests}"),
	)

	positionsSaved, _ := meter.Int64Counter("tripgo.positions.saved",
		metric.WithDescription("Total number of positions saved"),
		metric.WithUnit("{positions}"),
	)

	staleChecks, _ := meter.Int64Counter("tripgo.stale_checks",
		metric.WithDescription("Result of stale trip checks"),
		metric.WithUnit("{checks}"),
	)

	rowsRead, _ := meter.Int64Counter("tripgo.stale_trips.rows_read",
		metric.WithDescription("Total number of stale trip rows read"),
		metric.WithUnit("{rows}"),
	)

	queueSize, _ := meter.Int64UpDownCounter("tripgo.position_requests.queue.size",
		metric.WithDescription("Current number of tasks in the queue"),
		metric.WithUnit("{tasks}"),
	)

	workersActive, _ := meter.Int64UpDownCounter("tripgo.position_requests.workers.active",
		metric.WithDescription("Current number of active workers"),
		metric.WithUnit("{workers}"),
	)

	posRequests, _ := meter.Int64Counter("tripgo.position_requests",
		metric.WithDescription("Result of position requests"),
		metric.WithUnit("{requests}"),
	)

	posReqDuration, _ := meter.Float64Histogram("tripgo.position_requests.duration",
		metric.WithDescription("Duration of position requests in seconds"),
		metric.WithUnit("s"),
	)

	shutdown := func(shutdownCtx context.Context) error {
		_ = tp.Shutdown(shutdownCtx)
		_ = mp.Shutdown(shutdownCtx)
		return nil
	}

	return &Telemetry{
		TripsCreatedCounter:           tripsCreated,
		TripsCompletedCounter:         tripsCompleted,
		HTTPRequestDuration:           httpDuration,
		HTTPActiveRequests:            activeRequests,
		PositionsSavedCounter:         positionsSaved,
		StaleChecksCounter:            staleChecks,
		StaleTripsRowsReadCounter:     rowsRead,
		PositionRequestsQueueSize:     queueSize,
		PositionRequestsWorkersActive: workersActive,
		PositionRequestsCounter:       posRequests,
		PositionRequestsDuration:      posReqDuration,
	}, shutdown, nil
}
