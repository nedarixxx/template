package list

import (
	"context"

	"github.com/google/uuid"

	domaintrip "github.com/nedarixxx/template/internal/domain/trip"
	"github.com/nedarixxx/template/internal/model"
)

type UseCase struct {
	repo tripRepository
}

func New(repo tripRepository) *UseCase {
	return &UseCase{repo: repo}
}

func (u *UseCase) Execute(ctx context.Context, tripID uuid.UUID) ([]model.TripPosition, error) {
	if tripID == uuid.Nil {
		return nil, domaintrip.ValidationError{Field: "trip_id", Message: "required"}
	}

	if _, err := u.repo.GetByID(ctx, tripID); err != nil {
		return nil, err
	}

	positions, err := u.repo.GetPositions(ctx, tripID)
	if err != nil {
		return nil, err
	}

	if positions == nil {
		positions = []model.TripPosition{}
	}

	return positions, nil
}
