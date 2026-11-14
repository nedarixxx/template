//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package create

import (
	"context"

	"github.com/nedarixxx/template/internal/model"
	usecasecreate "github.com/nedarixxx/template/internal/usecase/trip/position/create"
)

type useCase interface {
	Execute(ctx context.Context, cmd usecasecreate.Command) (model.TripPosition, error)
}
