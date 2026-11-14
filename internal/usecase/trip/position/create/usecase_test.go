package create_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/nedarixxx/template/internal/domain/trip"
	"github.com/nedarixxx/template/internal/model"
	usecasecreate "github.com/nedarixxx/template/internal/usecase/trip/position/create"
)

func TestUseCase_Execute(t *testing.T) {
	tripID := uuid.New()
	recordedAt := time.Now().UTC()
	cmd := usecasecreate.Command{
		TripID: tripID,
		Coordinates: model.Coordinates{
			Latitude:  55.75,
			Longitude: 37.61,
		},
		RecordedAt: recordedAt,
	}

	t.Run("success create position and update last_position_at", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRepo := NewMocktripRepository(ctrl)
		mockTx := NewMocktxManager(ctrl)
		mockCanceler := NewMockqueueCanceler(ctrl)

		mockRepo.EXPECT().GetByID(gomock.Any(), tripID).Return(model.Trip{
			ID:     tripID,
			Status: model.StatusActive,
		}, nil)

		mockTx.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})

		savedPos := model.TripPosition{
			ID:          42,
			TripID:      tripID,
			Coordinates: cmd.Coordinates,
			RecordedAt:  recordedAt,
			CreatedAt:   recordedAt,
		}

		mockRepo.EXPECT().SavePosition(gomock.Any(), gomock.Any()).Return(savedPos, nil)
		mockRepo.EXPECT().UpdateLastPositionAt(gomock.Any(), tripID, recordedAt).Return(nil)
		mockCanceler.EXPECT().Cancel(gomock.Any(), tripID)

		uc := usecasecreate.New(mockRepo, mockTx, mockCanceler)
		res, err := uc.Execute(context.Background(), cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.ID != 42 || res.TripID != tripID {
			t.Fatalf("unexpected result: %+v", res)
		}
	})

	t.Run("trip not found returns ErrTripNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRepo := NewMocktripRepository(ctrl)
		mockTx := NewMocktxManager(ctrl)
		mockCanceler := NewMockqueueCanceler(ctrl)

		mockRepo.EXPECT().GetByID(gomock.Any(), tripID).Return(model.Trip{}, domaintrip.ErrTripNotFound)

		uc := usecasecreate.New(mockRepo, mockTx, mockCanceler)
		_, err := uc.Execute(context.Background(), cmd)
		if !errors.Is(err, domaintrip.ErrTripNotFound) {
			t.Fatalf("expected ErrTripNotFound, got %v", err)
		}
	})

	t.Run("completed trip returns ErrTripCompleted", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRepo := NewMocktripRepository(ctrl)
		mockTx := NewMocktxManager(ctrl)
		mockCanceler := NewMockqueueCanceler(ctrl)

		mockRepo.EXPECT().GetByID(gomock.Any(), tripID).Return(model.Trip{
			ID:     tripID,
			Status: model.StatusCompleted,
		}, nil)

		uc := usecasecreate.New(mockRepo, mockTx, mockCanceler)
		_, err := uc.Execute(context.Background(), cmd)
		if !errors.Is(err, domaintrip.ErrTripCompleted) {
			t.Fatalf("expected ErrTripCompleted, got %v", err)
		}
	})

	t.Run("validation invalid latitude", func(t *testing.T) {
		uc := usecasecreate.New(nil, nil, nil)
		badCmd := cmd
		badCmd.Coordinates.Latitude = 100
		_, err := uc.Execute(context.Background(), badCmd)
		var vErr domaintrip.ValidationError
		if !errors.As(err, &vErr) || vErr.Field != "coordinates.latitude" {
			t.Fatalf("expected validation error on latitude, got %v", err)
		}
	})

	t.Run("validation invalid longitude", func(t *testing.T) {
		uc := usecasecreate.New(nil, nil, nil)
		badCmd := cmd
		badCmd.Coordinates.Longitude = 200
		_, err := uc.Execute(context.Background(), badCmd)
		var vErr domaintrip.ValidationError
		if !errors.As(err, &vErr) || vErr.Field != "coordinates.longitude" {
			t.Fatalf("expected validation error on longitude, got %v", err)
		}
	})

	t.Run("validation recorded_at in future", func(t *testing.T) {
		uc := usecasecreate.New(nil, nil, nil)
		badCmd := cmd
		badCmd.RecordedAt = time.Now().Add(5 * time.Minute)
		_, err := uc.Execute(context.Background(), badCmd)
		var vErr domaintrip.ValidationError
		if !errors.As(err, &vErr) || vErr.Field != "recorded_at" {
			t.Fatalf("expected validation error on recorded_at, got %v", err)
		}
	})

	t.Run("validation empty trip_id", func(t *testing.T) {
		uc := usecasecreate.New(nil, nil, nil)
		badCmd := cmd
		badCmd.TripID = uuid.Nil
		_, err := uc.Execute(context.Background(), badCmd)
		var vErr domaintrip.ValidationError
		if !errors.As(err, &vErr) || vErr.Field != "trip_id" {
			t.Fatalf("expected validation error on trip_id, got %v", err)
		}
	})

	t.Run("tx rollback on save position error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockRepo := NewMocktripRepository(ctrl)
		mockTx := NewMocktxManager(ctrl)
		mockCanceler := NewMockqueueCanceler(ctrl)

		mockRepo.EXPECT().GetByID(gomock.Any(), tripID).Return(model.Trip{
			ID:     tripID,
			Status: model.StatusActive,
		}, nil)

		expectedErr := errors.New("db error")
		mockTx.EXPECT().Do(gomock.Any(), gomock.Any()).Return(expectedErr)

		uc := usecasecreate.New(mockRepo, mockTx, mockCanceler)
		_, err := uc.Execute(context.Background(), cmd)
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected db error, got %v", err)
		}
	})
}
