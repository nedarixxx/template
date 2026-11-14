package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	tracer     trace.Tracer
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: strings.TrimRight(baseURL, "/"),
		tracer:  otel.GetTracerProvider().Tracer("push-gateway"),
	}
}

type pushRequestBody struct {
	RequestID   string            `json:"request_id"`
	RecipientID uuid.UUID         `json:"recipient_id"`
	Kind        string            `json:"kind"`
	Data        map[string]string `json:"data"`
}

func (c *Client) RequestPosition(ctx context.Context, driverID, tripID uuid.UUID) error {
	ctx, span := c.tracer.Start(ctx, "HTTP POST /api/v1/push",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.HTTPRequestMethodKey.String(http.MethodPost),
			semconv.URLFull(c.baseURL+"/api/v1/push"),
		),
	)
	defer span.End()

	reqBody := pushRequestBody{
		RequestID:   uuid.New().String(),
		RecipientID: driverID,
		Kind:        "REQUEST_POSITION",
		Data: map[string]string{
			"trip_id": tripID.String(),
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("marshal push request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/push", bytes.NewReader(payload))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create push http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(httpReq.Header))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("push service transport error: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	_, _ = io.Copy(io.Discard, resp.Body)

	span.SetAttributes(semconv.HTTPResponseStatusCode(resp.StatusCode))

	if resp.StatusCode == http.StatusAccepted {
		return nil
	}

	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		err := fmt.Errorf("push service client error: %d", resp.StatusCode)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	if resp.StatusCode >= 500 {
		err := fmt.Errorf("push service server error: %d", resp.StatusCode)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	err = fmt.Errorf("push service unexpected status: %d", resp.StatusCode)
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	return err
}
