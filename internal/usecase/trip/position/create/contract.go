//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package create

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/nedarixxx/template/internal/model"
)

type (
	tripRepository interface {
		GetByID(ctx context.Context, id uuid.UUID) (model.Trip, error)
		SavePosition(ctx context.Context, pos model.TripPosition) (model.TripPosition, error)
		UpdateLastPositionAt(ctx context.Context, tripID uuid.UUID, recordedAt time.Time) error
	}

	txManager interface {
		Do(ctx context.Context, fn func(ctx context.Context) error) error
	}

	queueCanceler interface {
		Cancel(ctx context.Context, tripID uuid.UUID)
	}
)
