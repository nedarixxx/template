package create

import (
	"context"

	"github.com/google/uuid"

	"github.com/nedarixxx/template/internal/model"
)


type (
	tripRepository interface {
		Save(ctx context.Context, trip model.Trip) error
		HasActiveTrip(ctx context.Context, driverID uuid.UUID) (bool, error)
	}

	txManager interface {
		Do(ctx context.Context, fn func(ctx context.Context) error) error
	}
)