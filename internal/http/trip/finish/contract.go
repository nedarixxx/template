package finish

import (
	"context"

	"github.com/google/uuid"
	"github.com/nedarixxx/template/internal/model"
)

//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

type useCase interface {
	Execute(ctx context.Context, id uuid.UUID) (model.Trip, error)
}