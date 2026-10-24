package finish_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/nedarixxx/template/internal/domain/trip"
	"github.com/nedarixxx/template/internal/model"
	"github.com/nedarixxx/template/internal/usecase/trip/finish"
)

func TestFinishUseCase_Execute(t *testing.T) {
	ctx := context.Background()
	tripID := uuid.New()
	now := time.Now().UTC()

	t.Run("success finish active trip", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)

		tx.EXPECT().Do(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})

		existing := model.Trip{
			ID:        tripID,
			Status:    model.StatusActive,
			StartedAt: now.Add(-15 * time.Minute),
		}

		repo.EXPECT().GetByID(ctx, tripID).Return(existing, nil)
		repo.EXPECT().Update(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, tr model.Trip) error {
			if tr.Status != model.StatusCompleted {
				t.Fatalf("expected status completed, got %s", tr.Status)
			}
			if tr.FinishedAt == nil {
				t.Fatal("expected non-nil FinishedAt")
			}
			return nil
		})
		repo.EXPECT().AddStatusHistory(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, h model.TripStatusHistory) error {
			if h.TripID != tripID || h.ToStatus != model.StatusCompleted {
				t.Fatalf("unexpected status history: %+v", h)
			}
			return nil
		})

		uc := finish.New(repo, tx)
		res, err := uc.Execute(ctx, tripID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if res.Status != model.StatusCompleted {
			t.Fatalf("expected completed status, got %s", res.Status)
		}
	})

	t.Run("fail finish already completed trip returns ErrTripCompleted", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)

		tx.EXPECT().Do(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})

		finishedAt := now.Add(-5 * time.Minute)
		existing := model.Trip{
			ID:         tripID,
			Status:     model.StatusCompleted,
			StartedAt:  now.Add(-20 * time.Minute),
			FinishedAt: &finishedAt,
		}

		repo.EXPECT().GetByID(ctx, tripID).Return(existing, nil)

		uc := finish.New(repo, tx)
		_, err := uc.Execute(ctx, tripID)
		if !errors.Is(err, domaintrip.ErrTripCompleted) {
			t.Fatalf("expected ErrTripCompleted, got: %v", err)
		}
	})

	t.Run("fail repo get returns error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)

		tx.EXPECT().Do(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})

		expectedErr := errors.New("db select error")
		repo.EXPECT().GetByID(ctx, tripID).Return(model.Trip{}, expectedErr)

		uc := finish.New(repo, tx)
		_, err := uc.Execute(ctx, tripID)
		if err == nil || !errors.Is(err, expectedErr) {
			t.Fatalf("expected error %v, got: %v", expectedErr, err)
		}
	})

	t.Run("fail repo update returns error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)

		tx.EXPECT().Do(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})

		existing := model.Trip{
			ID:        tripID,
			Status:    model.StatusActive,
			StartedAt: now.Add(-15 * time.Minute),
		}

		repo.EXPECT().GetByID(ctx, tripID).Return(existing, nil)
		repo.EXPECT().Update(ctx, gomock.Any()).Return(errors.New("db update error"))

		uc := finish.New(repo, tx)
		_, err := uc.Execute(ctx, tripID)
		if err == nil {
			t.Fatal("expected update error, got nil")
		}
	})

	t.Run("fail add status history returns error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMocktripRepository(ctrl)
		tx := NewMocktxManager(ctrl)

		tx.EXPECT().Do(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})

		existing := model.Trip{
			ID:        tripID,
			Status:    model.StatusActive,
			StartedAt: now.Add(-15 * time.Minute),
		}

		repo.EXPECT().GetByID(ctx, tripID).Return(existing, nil)
		repo.EXPECT().Update(ctx, gomock.Any()).Return(nil)
		repo.EXPECT().AddStatusHistory(ctx, gomock.Any()).Return(errors.New("db history error"))

		uc := finish.New(repo, tx)
		_, err := uc.Execute(ctx, tripID)
		if err == nil {
			t.Fatal("expected status history error, got nil")
		}
	})
}