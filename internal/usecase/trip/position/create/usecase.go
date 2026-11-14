package create

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"

	domaintrip "github.com/nedarixxx/template/internal/domain/trip"
	"github.com/nedarixxx/template/internal/model"
)

type Command struct {
	TripID      uuid.UUID
	Coordinates model.Coordinates
	RecordedAt  time.Time
}

type UseCase struct {
	repo      tripRepository
	txManager txManager
	canceler  queueCanceler
}

func New(repo tripRepository, tx txManager, canceler queueCanceler) *UseCase {
	return &UseCase{
		repo:      repo,
		txManager: tx,
		canceler:  canceler,
	}
}

func (u *UseCase) Execute(ctx context.Context, cmd Command) (model.TripPosition, error) {
	if cmd.TripID == uuid.Nil {
		return model.TripPosition{}, domaintrip.ValidationError{Field: "trip_id", Message: "required"}
	}
	if math.IsNaN(cmd.Coordinates.Latitude) || math.IsInf(cmd.Coordinates.Latitude, 0) || cmd.Coordinates.Latitude < -90 || cmd.Coordinates.Latitude > 90 {
		return model.TripPosition{}, domaintrip.ValidationError{Field: "coordinates.latitude", Message: "must be between -90 and 90"}
	}
	if math.IsNaN(cmd.Coordinates.Longitude) || math.IsInf(cmd.Coordinates.Longitude, 0) || cmd.Coordinates.Longitude < -180 || cmd.Coordinates.Longitude > 180 {
		return model.TripPosition{}, domaintrip.ValidationError{Field: "coordinates.longitude", Message: "must be between -180 and 180"}
	}
	if cmd.RecordedAt.IsZero() {
		return model.TripPosition{}, domaintrip.ValidationError{Field: "recorded_at", Message: "required"}
	}
	if cmd.RecordedAt.After(time.Now().Add(1 * time.Minute)) {
		return model.TripPosition{}, domaintrip.ValidationError{Field: "recorded_at", Message: "cannot be in the future by more than 1 minute"}
	}

	trip, err := u.repo.GetByID(ctx, cmd.TripID)
	if err != nil {
		return model.TripPosition{}, err
	}

	domainTrip := trip.ToDomain()
	if err := domainTrip.AddPosition(cmd.RecordedAt); err != nil {
		return model.TripPosition{}, err
	}

	var saved model.TripPosition

	err = u.txManager.Do(ctx, func(txCtx context.Context) error {
		pos := model.TripPosition{
			TripID:      cmd.TripID,
			Coordinates: cmd.Coordinates,
			RecordedAt:  cmd.RecordedAt,
		}

		created, err := u.repo.SavePosition(txCtx, pos)
		if err != nil {
			return err
		}

		if err := u.repo.UpdateLastPositionAt(txCtx, cmd.TripID, cmd.RecordedAt); err != nil {
			return err
		}

		saved = created
		return nil
	})
	if err != nil {
		return model.TripPosition{}, err
	}

	if u.canceler != nil {
		u.canceler.Cancel(ctx, cmd.TripID)
	}

	return saved, nil
}
