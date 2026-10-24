package create_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/nedarixxx/template/internal/domain/trip"
	"github.com/nedarixxx/template/internal/model"
	"github.com/nedarixxx/template/internal/usecase/trip/create"
)

func TestCreateUseCase_Execute(t *testing.T) {
	ctx := context.Background()
	driverID := uuid.New()
	userID := uuid.New()

	cmd := create.Command{
		UserID:   userID,
		DriverID: driverID,
		StartPoint: model.Coordinates{
			Latitude:  55.751244,
			Longitude: 37.618423,
		},
		EndPoint: model.Coordinates{
			Latitude:  55.755819,
			Longitude: 37.617644,
		},
		Price: 500,
	}

	t.Run("success create trip", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)

		tx.EXPECT().Do(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})

		repo.EXPECT().HasActiveTrip(ctx, driverID).Return(false, nil)
		repo.EXPECT().Save(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, tr model.Trip) error {
			if tr.DriverID != driverID || tr.UserID != userID || tr.Price != 500 {
				t.Fatalf("unexpected trip data: %+v", tr)
			}
			if tr.Status != model.StatusActive {
				t.Fatalf("expected status active, got %s", tr.Status)
			}
			return nil
		})

		uc := create.New(repo, tx)
		res, err := uc.Execute(ctx, cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res.ID == uuid.Nil {
			t.Fatal("expected generated trip ID")
		}
		if res.Status != model.StatusActive {
			t.Fatalf("expected status active, got %s", res.Status)
		}
	})

	t.Run("driver busy returns ErrDriverBusy", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)

		tx.EXPECT().Do(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})

		repo.EXPECT().HasActiveTrip(ctx, driverID).Return(true, nil)

		uc := create.New(repo, tx)
		_, err := uc.Execute(ctx, cmd)
		if !errors.Is(err, domaintrip.ErrDriverBusy) {
			t.Fatalf("expected ErrDriverBusy, got: %v", err)
		}
	})

	t.Run("repo check failure", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)

		dbErr := errors.New("db connection failure")
		tx.EXPECT().Do(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})

		repo.EXPECT().HasActiveTrip(ctx, driverID).Return(false, dbErr)

		uc := create.New(repo, tx)
		_, err := uc.Execute(ctx, cmd)
		if !errors.Is(err, dbErr) {
			t.Fatalf("expected dbErr, got: %v", err)
		}
	})
}