package finish

import (
	"context"
	"time"

	"github.com/google/uuid"

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

func (u *UseCase) Execute(ctx context.Context, id uuid.UUID) (model.Trip, error) {
	var updatedTrip model.Trip

	err := u.txManager.Do(ctx, func(ctx context.Context) error {
		current, err := u.repo.GetByID(ctx, id)
		if err != nil {
			return err
		}

		domainEntity := current.ToDomain()
		now := time.Now().UTC()
		if err := domainEntity.Finish(now); err != nil {
			return err
		}

		updatedTrip = model.FromDomain(domainEntity)

		if err := u.repo.Update(ctx, updatedTrip); err != nil {
			return err
		}

		fromStatus := current.Status
		return u.repo.AddStatusHistory(ctx, model.TripStatusHistory{
			TripID:     id,
			FromStatus: &fromStatus,
			ToStatus:   updatedTrip.Status,
			ChangedAt:  now,
		})
	})
	if err != nil {
		return model.Trip{}, err
	}

	return updatedTrip, nil
}