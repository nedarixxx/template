package list_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/nedarixxx/template/internal/domain/trip"
	"github.com/nedarixxx/template/internal/model"
	usecaselist "github.com/nedarixxx/template/internal/usecase/trip/position/list"
)

func TestUseCase_Execute(t *testing.T) {
	tripID := uuid.New()

	t.Run("success list positions", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRepo := NewMocktripRepository(ctrl)

		mockRepo.EXPECT().GetByID(gomock.Any(), tripID).Return(model.Trip{
			ID: tripID,
		}, nil)

		expected := []model.TripPosition{
			{
				ID:          1,
				TripID:      tripID,
				Coordinates: model.Coordinates{Latitude: 55.75, Longitude: 37.61},
				RecordedAt:  time.Now().UTC(),
			},
		}

		mockRepo.EXPECT().GetPositions(gomock.Any(), tripID).Return(expected, nil)

		uc := usecaselist.New(mockRepo)
		res, err := uc.Execute(context.Background(), tripID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 1 || res[0].ID != 1 {
			t.Fatalf("unexpected positions: %+v", res)
		}
	})

	t.Run("trip not found returns ErrTripNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRepo := NewMocktripRepository(ctrl)

		mockRepo.EXPECT().GetByID(gomock.Any(), tripID).Return(model.Trip{}, domaintrip.ErrTripNotFound)

		uc := usecaselist.New(mockRepo)
		_, err := uc.Execute(context.Background(), tripID)
		if !errors.Is(err, domaintrip.ErrTripNotFound) {
			t.Fatalf("expected ErrTripNotFound, got %v", err)
		}
	})

	t.Run("empty tripID returns validation error", func(t *testing.T) {
		uc := usecaselist.New(nil)
		_, err := uc.Execute(context.Background(), uuid.Nil)
		var vErr domaintrip.ValidationError
		if !errors.As(err, &vErr) || vErr.Field != "trip_id" {
			t.Fatalf("expected validation error on trip_id, got %v", err)
		}
	})

	t.Run("empty positions list returns non-nil slice", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRepo := NewMocktripRepository(ctrl)

		mockRepo.EXPECT().GetByID(gomock.Any(), tripID).Return(model.Trip{
			ID: tripID,
		}, nil)
		mockRepo.EXPECT().GetPositions(gomock.Any(), tripID).Return(nil, nil)

		uc := usecaselist.New(mockRepo)
		res, err := uc.Execute(context.Background(), tripID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == nil || len(res) != 0 {
			t.Fatalf("expected empty non-nil slice, got %+v", res)
		}
	})
}
