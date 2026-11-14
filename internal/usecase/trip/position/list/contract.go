//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package list

import (
	"context"

	"github.com/google/uuid"

	"github.com/nedarixxx/template/internal/model"
)

type tripRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (model.Trip, error)
	GetPositions(ctx context.Context, tripID uuid.UUID) ([]model.TripPosition, error)
}
