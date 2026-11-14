package list

import (
	"github.com/nedarixxx/template/api"
	"github.com/nedarixxx/template/internal/model"
)

func ToResponse(positions []model.TripPosition) api.TripPositionsResponse {
	res := make([]api.TripPosition, 0, len(positions))
	for _, p := range positions {
		res = append(res, api.TripPosition{
			Id:     p.ID,
			TripId: p.TripID,
			Coordinates: api.Coordinates{
				Latitude:  p.Coordinates.Latitude,
				Longitude: p.Coordinates.Longitude,
			},
			RecordedAt: p.RecordedAt,
		})
	}
	return api.TripPositionsResponse{
		Positions: res,
	}
}
