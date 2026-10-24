package trip

import (
	"time"

	"github.com/google/uuid"

	"github.com/nedarixxx/template/internal/model"
)

type dbTrip struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	DriverID       uuid.UUID
	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64
	Price          int64
	Status         string
	StartedAt      time.Time
	FinishedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func toModel(d dbTrip) model.Trip {
	return model.Trip{
		ID:       d.ID,
		UserID:   d.UserID,
		DriverID: d.DriverID,
		StartPoint: model.Coordinates{
			Latitude:  d.StartLatitude,
			Longitude: d.StartLongitude,
		},
		EndPoint: model.Coordinates{
			Latitude:  d.EndLatitude,
			Longitude: d.EndLongitude,
		},
		Price:      d.Price,
		Status:     model.TripStatus(d.Status),
		StartedAt:  d.StartedAt,
		FinishedAt: d.FinishedAt,
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
	}
}