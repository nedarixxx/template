package get_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/nedarixxx/template/internal/domain/trip"
	getHandler "github.com/nedarixxx/template/internal/http/trip/get"
	"github.com/nedarixxx/template/internal/model"
)

func TestHandler_Handle(t *testing.T) {
	tripID := uuid.New()

	t.Run("success get trip returns 200", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUC := NewMockuseCase(ctrl)
		mockUC.EXPECT().Execute(gomock.Any(), tripID).Return(model.Trip{
			ID:        tripID,
			Price:     350,
			Status:    model.StatusActive,
			StartedAt: time.Now().UTC(),
		}, nil)

		h := getHandler.New(mockUC)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+tripID.String(), nil)
		rec := httptest.NewRecorder()

		h.Handle(rec, req, tripID)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("not found returns 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUC := NewMockuseCase(ctrl)
		mockUC.EXPECT().Execute(gomock.Any(), tripID).Return(model.Trip{}, domaintrip.ErrTripNotFound)

		h := getHandler.New(mockUC)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+tripID.String(), nil)
		rec := httptest.NewRecorder()

		h.Handle(rec, req, tripID)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rec.Code)
		}
	})
}