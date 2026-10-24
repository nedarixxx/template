package telemetry

import (
	"context"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Telemetry struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider

	// Бизнес-метрики
	TripsCreatedCounter   metric.Int64Counter
	TripsCompletedCounter metric.Int64Counter

	// HTTP-метрики
	HTTPRequestDuration metric.Float64Histogram
	HTTPActiveRequests  metric.Int64UpDownCounter
}

// Init инициализирует трейсинг и метрики.
// Если коллектор недоступен, сервис продолжает штатно работать.
func Init(ctx context.Context, serviceName string) (*Telemetry, func(context.Context) error, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, nil, err
	}

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:22317"
	}
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")

	// Настройка OTLP gRPC экспортера трейсов
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	if err != nil {
		return nil, nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// Настройка OTLP экспортера метрик
	metricExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithEndpoint(endpoint),
	)
	if err != nil {
		return nil, nil, err
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(3*time.Second))),
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

	shutdown := func(shutdownCtx context.Context) error {
		_ = tp.Shutdown(shutdownCtx)
		_ = mp.Shutdown(shutdownCtx)
		return nil
	}

	return &Telemetry{
		tracerProvider:        tp,
		meterProvider:         mp,
		TripsCreatedCounter:   tripsCreated,
		TripsCompletedCounter: tripsCompleted,
		HTTPRequestDuration:   httpDuration,
		HTTPActiveRequests:    activeRequests,
	}, shutdown, nil
}