package finish_test

import (
	"go.opentelemetry.io/otel/metric/noop"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/nedarixxx/template/internal/domain/trip"
	finishHandler "github.com/nedarixxx/template/internal/http/trip/finish"
	"github.com/nedarixxx/template/internal/model"
)

func TestHandler_Handle(t *testing.T) {
	tripID := uuid.New()
	now := time.Now().UTC()

	t.Run("success finish trip returns 200", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUC := NewMockuseCase(ctrl)
		mockUC.EXPECT().Execute(gomock.Any(), tripID).Return(model.Trip{
			ID:         tripID,
			Status:     model.StatusCompleted,
			StartedAt:  now.Add(-10 * time.Minute),
			FinishedAt: &now,
		}, nil)

		h := finishHandler.New(mockUC)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/finish", nil)
		rec := httptest.NewRecorder()

		h.Handle(rec, req, tripID)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("success finish trip with counter option", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUC := NewMockuseCase(ctrl)
		tripID := uuid.New()
		now := time.Now().UTC()
		finishedAt := now.Add(10 * time.Minute)

		mockUC.EXPECT().Execute(gomock.Any(), tripID).Return(model.Trip{
			ID:         tripID,
			UserID:     uuid.New(),
			DriverID:   uuid.New(),
			Price:      600,
			Status:     model.StatusCompleted,
			StartedAt:  now,
			FinishedAt: &finishedAt,
		}, nil)

		meter := noop.NewMeterProvider().Meter("test")
		counter, err := meter.Int64Counter("trips_completed_total")
		if err != nil {
			t.Fatalf("failed to create counter: %v", err)
		}

		h := finishHandler.New(mockUC, finishHandler.WithCounter(counter))
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/finish", nil)
		rec := httptest.NewRecorder()

		h.Handle(rec, req, tripID)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("already completed returns 409", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUC := NewMockuseCase(ctrl)
		mockUC.EXPECT().Execute(gomock.Any(), tripID).Return(model.Trip{}, domaintrip.ErrTripCompleted)

		h := finishHandler.New(mockUC)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/finish", nil)
		rec := httptest.NewRecorder()

		h.Handle(rec, req, tripID)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected status 409, got %d", rec.Code)
		}
	})
}