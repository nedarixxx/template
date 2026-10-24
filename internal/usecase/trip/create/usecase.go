package create

import (
	"context"
	"time"

	"github.com/google/uuid"

	domaintrip "github.com/nedarixxx/template/internal/domain/trip"
	"github.com/nedarixxx/template/internal/model"
)

type UseCase struct {
	repo      tripRepository
	txManager txManager
}

func New(repo tripRepository, tx txManager) *UseCase {
	return &UseCase{
		repo:      repo,
		txManager: tx,
	}
}

type Command struct {
	UserID     uuid.UUID
	DriverID   uuid.UUID
	StartPoint model.Coordinates
	EndPoint   model.Coordinates
	Price      int64
}

func (u *UseCase) Execute(ctx context.Context, cmd Command) (model.Trip, error) {
	now := time.Now().UTC()

	newTrip := model.Trip{
		ID:         uuid.New(),
		UserID:     cmd.UserID,
		DriverID:   cmd.DriverID,
		StartPoint: cmd.StartPoint,
		EndPoint:   cmd.EndPoint,
		Price:      cmd.Price,
		Status:     model.StatusActive,
		StartedAt:  now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	err := u.txManager.Do(ctx, func(ctx context.Context) error {
		busy, err := u.repo.HasActiveTrip(ctx, cmd.DriverID)
		if err != nil {
			return err
		}
		if busy {
			return domaintrip.ErrDriverBusy
		}

		return u.repo.Save(ctx, newTrip)
	})
	if err != nil {
		return model.Trip{}, err
	}

	return newTrip, nil
}