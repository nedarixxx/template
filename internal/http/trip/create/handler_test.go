package create_test

import (
	"go.opentelemetry.io/otel/metric/noop"
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/nedarixxx/template/api"
	domaintrip "github.com/nedarixxx/template/internal/domain/trip"
	createHandler "github.com/nedarixxx/template/internal/http/trip/create"
	"github.com/nedarixxx/template/internal/model"
)

func TestHandler_Handle(t *testing.T) {
	driverID := uuid.New()
	userID := uuid.New()

	validBody, _ := json.Marshal(api.TripData{
		UserId:   userID,
		DriverId: driverID,
		StartPoint: api.Coordinates{
			Latitude:  55.75,
			Longitude: 37.61,
		},
		EndPoint: api.Coordinates{
			Latitude:  55.76,
			Longitude: 37.62,
		},
		Price: 500,
	})

	t.Run("success create trip with counter option", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUC := NewMockuseCase(ctrl)
		tripID := uuid.New()
		mockUC.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(model.Trip{
			ID:        tripID,
			UserID:    userID,
			DriverID:  driverID,
			Price:     500,
			Status:    model.StatusActive,
			StartedAt: time.Now().UTC(),
		}, nil)

		meter := noop.NewMeterProvider().Meter("test")
		counter, err := meter.Int64Counter("trips_created_total")
		if err != nil {
			t.Fatalf("failed to create counter: %v", err)
		}

		h := createHandler.New(mockUC, createHandler.WithCounter(counter))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader(validBody))
		rec := httptest.NewRecorder()

		h.Handle(rec, req, api.CreateTripParams{})

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", rec.Code)
		}
	})

	t.Run("success create trip returns 201", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUC := NewMockuseCase(ctrl)
		tripID := uuid.New()
		mockUC.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(model.Trip{
			ID:        tripID,
			UserID:    userID,
			DriverID:  driverID,
			Price:     500,
			Status:    model.StatusActive,
			StartedAt: time.Now().UTC(),
		}, nil)

		h := createHandler.New(mockUC)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader(validBody))
		rec := httptest.NewRecorder()

		h.Handle(rec, req, api.CreateTripParams{})

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", rec.Code)
		}
	})

	t.Run("invalid request body returns 400 with invalid_request", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUC := NewMockuseCase(ctrl)
		h := createHandler.New(mockUC)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader([]byte("not-json")))
		rec := httptest.NewRecorder()

		h.Handle(rec, req, api.CreateTripParams{})

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("driver busy returns 409", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUC := NewMockuseCase(ctrl)
		mockUC.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(model.Trip{}, domaintrip.ErrDriverBusy)

		h := createHandler.New(mockUC)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader(validBody))
		rec := httptest.NewRecorder()

		h.Handle(rec, req, api.CreateTripParams{})

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected status 409, got %d", rec.Code)
		}
	})

	t.Run("repo error returns 500 without details", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUC := NewMockuseCase(ctrl)
		mockUC.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(model.Trip{}, errors.New("db error"))

		h := createHandler.New(mockUC)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader(validBody))
		rec := httptest.NewRecorder()

		h.Handle(rec, req, api.CreateTripParams{})

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", rec.Code)
		}
	})
}