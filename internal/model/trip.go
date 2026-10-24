package model

import (
	"time"

	"github.com/google/uuid"

	domaintrip "github.com/nedarixxx/template/internal/domain/trip"
)

type TripStatus string

const (
	StatusActive    TripStatus = "active"
	StatusCompleted TripStatus = "completed"
)

type Coordinates struct {
	Latitude  float64
	Longitude float64
}

type Trip struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	DriverID   uuid.UUID
	StartPoint Coordinates
	EndPoint   Coordinates
	Price      int64
	Status     TripStatus
	StartedAt  time.Time
	FinishedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (t Trip) ToDomain() *domaintrip.Trip {
	return &domaintrip.Trip{
		ID:     t.ID,
		UserID: t.UserID,
		DriverID: t.DriverID,
		StartPoint: domaintrip.Coordinates{
			Latitude:  t.StartPoint.Latitude,
			Longitude: t.StartPoint.Longitude,
		},
		EndPoint: domaintrip.Coordinates{
			Latitude:  t.EndPoint.Latitude,
			Longitude: t.EndPoint.Longitude,
		},
		Price:      t.Price,
		Status:     domaintrip.Status(t.Status),
		StartedAt:  t.StartedAt,
		FinishedAt: t.FinishedAt,
		CreatedAt:  t.CreatedAt,
		UpdatedAt:  t.UpdatedAt,
	}
}

func FromDomain(d *domaintrip.Trip) Trip {
	return Trip{
		ID:       d.ID,
		UserID:   d.UserID,
		DriverID: d.DriverID,
		StartPoint: Coordinates{
			Latitude:  d.StartPoint.Latitude,
			Longitude: d.StartPoint.Longitude,
		},
		EndPoint: Coordinates{
			Latitude:  d.EndPoint.Latitude,
			Longitude: d.EndPoint.Longitude,
		},
		Price:      d.Price,
		Status:     TripStatus(d.Status),
		StartedAt:  d.StartedAt,
		FinishedAt: d.FinishedAt,
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
	}
}

type TripStatusHistory struct {
	ID         int64
	TripID     uuid.UUID
	FromStatus *TripStatus
	ToStatus   TripStatus
	Reason     *string
	ChangedAt  time.Time
}