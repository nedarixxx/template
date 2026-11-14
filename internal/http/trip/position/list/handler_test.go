package list_test

import (
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
	listHandler "github.com/nedarixxx/template/internal/http/trip/position/list"
	"github.com/nedarixxx/template/internal/model"
)

func TestHandler_Handle(t *testing.T) {
	tripID := uuid.New()

	t.Run("success returns 200 with positions list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUC := NewMockuseCase(ctrl)
		positions := []model.TripPosition{
			{
				ID:          1,
				TripID:      tripID,
				Coordinates: model.Coordinates{Latitude: 55.75, Longitude: 37.61},
				RecordedAt:  time.Now().UTC(),
			},
		}

		mockUC.EXPECT().Execute(gomock.Any(), tripID).Return(positions, nil)

		h := listHandler.New(mockUC)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+tripID.String()+"/positions", nil)
		rec := httptest.NewRecorder()

		h.Handle(rec, req, tripID)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var res api.TripPositionsResponse
		if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(res.Positions) != 1 || res.Positions[0].Id != 1 {
			t.Fatalf("unexpected positions in response: %+v", res)
		}
	})

	t.Run("trip not found returns 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUC := NewMockuseCase(ctrl)
		mockUC.EXPECT().Execute(gomock.Any(), tripID).Return(nil, domaintrip.ErrTripNotFound)

		h := listHandler.New(mockUC)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+tripID.String()+"/positions", nil)
		rec := httptest.NewRecorder()

		h.Handle(rec, req, tripID)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rec.Code)
		}
	})

	t.Run("nil trip id returns 400", func(t *testing.T) {
		h := listHandler.New(nil)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/positions", nil)
		rec := httptest.NewRecorder()

		h.Handle(rec, req, uuid.Nil)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("internal error returns 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockUC := NewMockuseCase(ctrl)
		mockUC.EXPECT().Execute(gomock.Any(), tripID).Return(nil, errors.New("database unavailable"))

		h := listHandler.New(mockUC)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+tripID.String()+"/positions", nil)
		rec := httptest.NewRecorder()

		h.Handle(rec, req, tripID)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", rec.Code)
		}
	})
}
