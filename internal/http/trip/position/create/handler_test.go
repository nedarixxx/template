package create_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/mock/gomock"

	"github.com/nedarixxx/template/api"
	domaintrip "github.com/nedarixxx/template/internal/domain/trip"
	createHandler "github.com/nedarixxx/template/internal/http/trip/position/create"
	"github.com/nedarixxx/template/internal/model"
)

func TestHandler_Handle(t *testing.T) {
	tripID := uuid.New()
	recordedAt := time.Now().UTC().Truncate(time.Second)

	validBody, _ := json.Marshal(api.PositionData{
		Coordinates: api.Coordinates{
			Latitude:  55.75,
			Longitude: 37.61,
		},
		RecordedAt: recordedAt,
	})

	t.Run("success create position with counter option returns 201", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUC := NewMockuseCase(ctrl)
		mockUC.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(model.TripPosition{
			ID:     42,
			TripID: tripID,
			Coordinates: model.Coordinates{
				Latitude:  55.75,
				Longitude: 37.61,
			},
			RecordedAt: recordedAt,
			CreatedAt:  recordedAt,
		}, nil)

		meter := noop.NewMeterProvider().Meter("test")
		counter, _ := meter.Int64Counter("tripgo.positions.saved")

		h := createHandler.New(mockUC, createHandler.WithCounter(counter))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/positions", bytes.NewReader(validBody))
		rec := httptest.NewRecorder()

		h.Handle(rec, req, tripID)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", rec.Code)
		}
	})

	t.Run("invalid json returns 400", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUC := NewMockuseCase(ctrl)
		h := createHandler.New(mockUC)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/positions", bytes.NewReader([]byte("{invalid-json")))
		rec := httptest.NewRecorder()

		h.Handle(rec, req, tripID)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("empty body returns 400", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUC := NewMockuseCase(ctrl)
		h := createHandler.New(mockUC)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/positions", bytes.NewReader([]byte("{}")))
		rec := httptest.NewRecorder()

		h.Handle(rec, req, tripID)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("trip not found returns 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUC := NewMockuseCase(ctrl)
		mockUC.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(model.TripPosition{}, domaintrip.ErrTripNotFound)

		h := createHandler.New(mockUC)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/positions", bytes.NewReader(validBody))
		rec := httptest.NewRecorder()

		h.Handle(rec, req, tripID)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rec.Code)
		}
	})

	t.Run("trip completed returns 409", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUC := NewMockuseCase(ctrl)
		mockUC.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(model.TripPosition{}, domaintrip.ErrTripCompleted)

		h := createHandler.New(mockUC)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/positions", bytes.NewReader(validBody))
		rec := httptest.NewRecorder()

		h.Handle(rec, req, tripID)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected status 409, got %d", rec.Code)
		}
	})

	t.Run("internal error returns 500 without details", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUC := NewMockuseCase(ctrl)
		mockUC.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(model.TripPosition{}, errors.New("db connection failure"))

		h := createHandler.New(mockUC)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/positions", bytes.NewReader(validBody))
		rec := httptest.NewRecorder()

		h.Handle(rec, req, tripID)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", rec.Code)
		}
	})

	t.Run("nil trip id returns 400", func(t *testing.T) {
		h := createHandler.New(nil)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/positions", bytes.NewReader(validBody))
		rec := httptest.NewRecorder()

		h.Handle(rec, req, uuid.Nil)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("unknown field returns 400", func(t *testing.T) {
		h := createHandler.New(nil)
		body := []byte(`{"coordinates":{"latitude":55.75,"longitude":37.61},"recorded_at":"2026-08-24T12:00:05Z","extra":"value"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/positions", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		h.Handle(rec, req, tripID)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400 for unknown fields, got %d", rec.Code)
		}
	})

	t.Run("trailing token returns 400", func(t *testing.T) {
		h := createHandler.New(nil)
		body := []byte(`{"coordinates":{"latitude":55.75,"longitude":37.61},"recorded_at":"2026-08-24T12:00:05Z"}{}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/positions", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		h.Handle(rec, req, tripID)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400 for trailing token, got %d", rec.Code)
		}
	})

	t.Run("latitude out of range returns 400", func(t *testing.T) {
		h := createHandler.New(nil)
		body := []byte(`{"coordinates":{"latitude":95.0,"longitude":37.61},"recorded_at":"2026-08-24T12:00:05Z"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/positions", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		h.Handle(rec, req, tripID)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400 for invalid latitude, got %d", rec.Code)
		}
	})

	t.Run("longitude out of range returns 400", func(t *testing.T) {
		h := createHandler.New(nil)
		body := []byte(`{"coordinates":{"latitude":55.75,"longitude":190.0},"recorded_at":"2026-08-24T12:00:05Z"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/positions", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		h.Handle(rec, req, tripID)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400 for invalid longitude, got %d", rec.Code)
		}
	})

	t.Run("recorded_at in future by more than 1 min returns 400", func(t *testing.T) {
		h := createHandler.New(nil)
		futureTime := time.Now().Add(5 * time.Minute).Format(time.RFC3339)
		body := []byte(`{"coordinates":{"latitude":55.75,"longitude":37.61},"recorded_at":"` + futureTime + `"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/positions", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		h.Handle(rec, req, tripID)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400 for future recorded_at, got %d", rec.Code)
		}
	})
}
