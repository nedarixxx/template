package create

import (
	"github.com/nedarixxx/template/api"
	"github.com/nedarixxx/template/internal/model"
)

func ToResponse(pos model.TripPosition) api.TripPosition {
	return api.TripPosition{
		Id:     pos.ID,
		TripId: pos.TripID,
		Coordinates: api.Coordinates{
			Latitude:  pos.Coordinates.Latitude,
			Longitude: pos.Coordinates.Longitude,
		},
		RecordedAt: pos.RecordedAt,
	}
}
