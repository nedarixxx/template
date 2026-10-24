package finish

import (
	"context"

	"github.com/google/uuid"

	"github.com/nedarixxx/template/internal/model"
)

type (
	tripRepository interface {
		GetByID(ctx context.Context, id uuid.UUID) (model.Trip, error)
		Update(ctx context.Context, trip model.Trip) error
		AddStatusHistory(ctx context.Context, history model.TripStatusHistory) error
	}

	txManager interface {
		Do(ctx context.Context, fn func(ctx context.Context) error) error
	}
)