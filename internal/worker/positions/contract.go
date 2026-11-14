package positions

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/nedarixxx/template/internal/model"
)

type (
	staleTripsRepository interface {
		GetStaleTrips(ctx context.Context, staleAfter time.Duration, limit int, lastStaleAt time.Time, lastID uuid.UUID) ([]model.StaleTrip, error)
	}

	pushGateway interface {
		RequestPosition(ctx context.Context, driverID, tripID uuid.UUID) error
	}

	Ticker interface {
		C() <-chan time.Time
		Stop()
	}
)
