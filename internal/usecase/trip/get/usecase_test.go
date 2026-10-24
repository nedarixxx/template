package get_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/nedarixxx/template/internal/domain/trip"
	"github.com/nedarixxx/template/internal/model"
	"github.com/nedarixxx/template/internal/usecase/trip/get"
)

func TestGetUseCase_Execute(t *testing.T) {
	ctx := context.Background()
	tripID := uuid.New()

	t.Run("success get existing trip", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMocktripRepository(ctrl)

		expectedTrip := model.Trip{
			ID:        tripID,
			Price:     350,
			Status:    model.StatusActive,
			StartedAt: time.Now().UTC(),
		}

		repo.EXPECT().GetByID(ctx, tripID).Return(expectedTrip, nil)

		uc := get.New(repo)
		trip, err := uc.Execute(ctx, tripID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if trip.ID != tripID || trip.Price != 350 {
			t.Fatalf("unexpected trip data: %+v", trip)
		}
	})

	t.Run("trip not found returns ErrTripNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMocktripRepository(ctrl)
		repo.EXPECT().GetByID(ctx, tripID).Return(model.Trip{}, domaintrip.ErrTripNotFound)

		uc := get.New(repo)
		_, err := uc.Execute(ctx, tripID)
		if !errors.Is(err, domaintrip.ErrTripNotFound) {
			t.Fatalf("expected ErrTripNotFound, got: %v", err)
		}
	})
}