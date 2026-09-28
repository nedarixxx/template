package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound         = errors.New("trip not found")
	ErrDriverBusy       = errors.New("driver already has an active trip")
	ErrAlreadyCompleted = errors.New("trip is already completed")
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

type TripStatusHistory struct {
	ID         int64
	TripID     uuid.UUID
	FromStatus *TripStatus
	ToStatus   TripStatus
	Reason     *string
	ChangedAt  time.Time
}