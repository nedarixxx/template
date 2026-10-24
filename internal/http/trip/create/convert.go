package create

import (
	"github.com/nedarixxx/template/api"
	"github.com/nedarixxx/template/internal/model"
	usecasecreate "github.com/nedarixxx/template/internal/usecase/trip/create"
)

func ToCommand(req api.TripData) usecasecreate.Command {
	return usecasecreate.Command{
		UserID:   req.UserId,
		DriverID: req.DriverId,
		StartPoint: model.Coordinates{
			Latitude:  req.StartPoint.Latitude,
			Longitude: req.StartPoint.Longitude,
		},
		EndPoint: model.Coordinates{
			Latitude:  req.EndPoint.Latitude,
			Longitude: req.EndPoint.Longitude,
		},
		Price: req.Price,
	}
}

func ToResponse(t model.Trip) api.Trip {
	return api.Trip{
		Id:       t.ID,
		UserId:   t.UserID,
		DriverId: t.DriverID,
		StartPoint: api.Coordinates{
			Latitude:  t.StartPoint.Latitude,
			Longitude: t.StartPoint.Longitude,
		},
		EndPoint: api.Coordinates{
			Latitude:  t.EndPoint.Latitude,
			Longitude: t.EndPoint.Longitude,
		},
		Price:      t.Price,
		Status:     api.TripStatus(t.Status),
		StartedAt:  t.StartedAt,
		FinishedAt: t.FinishedAt,
	}
}