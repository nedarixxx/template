package create

import (
	"context"

	"github.com/nedarixxx/template/internal/model"
	usecasecreate "github.com/nedarixxx/template/internal/usecase/trip/create"
)


type useCase interface {
	Execute(ctx context.Context, cmd usecasecreate.Command) (model.Trip, error)
}