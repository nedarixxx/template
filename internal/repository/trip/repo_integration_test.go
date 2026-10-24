//go:build integration

package trip_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	domaintrip "github.com/nedarixxx/template/internal/domain/trip"
	"github.com/nedarixxx/template/internal/infra/postgres/transaction"
	"github.com/nedarixxx/template/internal/model"
	repoTrip "github.com/nedarixxx/template/internal/repository/trip"
)

func getTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://tripgo:tripgo@localhost:22032/tripgo?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}
	return pool
}

func TestRepository_CreateAndGet(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	repo := repoTrip.New(pool)
	ctx := context.Background()

	tripID := uuid.New()
	driverID := uuid.New()
	userID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)

	item := model.Trip{
		ID:         tripID,
		UserID:     userID,
		DriverID:   driverID,
		StartPoint: model.Coordinates{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Coordinates{Latitude: 55.76, Longitude: 37.62},
		Price:      700,
		Status:     model.StatusActive,
		StartedAt:  now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM trip_status_history WHERE trip_id = $1", tripID)
		_, _ = pool.Exec(ctx, "DELETE FROM trips WHERE id = $1", tripID)
	}()

	if err := repo.Save(ctx, item); err != nil {
		t.Fatalf("unexpected save error: %v", err)
	}

	got, err := repo.GetByID(ctx, tripID)
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}

	if got.ID != tripID || got.Price != 700 {
		t.Fatalf("mismatch trip data: %+v", got)
	}
}

func TestTransaction_RollbackOnFailure(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	repo := repoTrip.New(pool)
	txManager := transaction.NewManager(pool)
	ctx := context.Background()

	tripID := uuid.New()
	driverID := uuid.New()
	now := time.Now().UTC()

	item := model.Trip{
		ID:         tripID,
		UserID:     uuid.New(),
		DriverID:   driverID,
		StartPoint: model.Coordinates{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Coordinates{Latitude: 55.76, Longitude: 37.62},
		Price:      1000,
		Status:     model.StatusActive,
		StartedAt:  now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM trip_status_history WHERE trip_id = $1", tripID)
		_, _ = pool.Exec(ctx, "DELETE FROM trips WHERE id = $1", tripID)
	}()

	errExpected := errors.New("simulated error to trigger rollback")

	err := txManager.Do(ctx, func(txCtx context.Context) error {
		if err := repo.Save(txCtx, item); err != nil {
			return err
		}

		if err := repo.AddStatusHistory(txCtx, model.TripStatusHistory{
			TripID:    tripID,
			ToStatus:  model.StatusActive,
			ChangedAt: now,
		}); err != nil {
			return err
		}

		return errExpected
	})

	if !errors.Is(err, errExpected) {
		t.Fatalf("expected simulated error, got: %v", err)
	}

	_, err = repo.GetByID(ctx, tripID)
	if !errors.Is(err, domaintrip.ErrTripNotFound) {
		t.Fatalf("expected ErrTripNotFound due to rollback, got: %v", err)
	}
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	repo := repoTrip.New(pool)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New())
	if !errors.Is(err, domaintrip.ErrTripNotFound) {
		t.Fatalf("expected ErrTripNotFound, got: %v", err)
	}
}

func TestRepository_Update(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	repo := repoTrip.New(pool)
	ctx := context.Background()

	tripID := uuid.New()
	driverID := uuid.New()
	userID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)

	item := model.Trip{
		ID:         tripID,
		UserID:     userID,
		DriverID:   driverID,
		StartPoint: model.Coordinates{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Coordinates{Latitude: 55.76, Longitude: 37.62},
		Price:      800,
		Status:     model.StatusActive,
		StartedAt:  now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM trip_status_history WHERE trip_id = $1", tripID)
		_, _ = pool.Exec(ctx, "DELETE FROM trips WHERE id = $1", tripID)
	}()

	if err := repo.Save(ctx, item); err != nil {
		t.Fatalf("unexpected save error: %v", err)
	}

	finishedAt := now.Add(20 * time.Minute)
	item.Status = model.StatusCompleted
	item.FinishedAt = &finishedAt
	item.UpdatedAt = finishedAt

	if err := repo.Update(ctx, item); err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}

	got, err := repo.GetByID(ctx, tripID)
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}

	if got.Status != model.StatusCompleted {
		t.Fatalf("expected status completed, got %s", got.Status)
	}
	if got.FinishedAt == nil || !got.FinishedAt.Equal(finishedAt) {
		t.Fatalf("expected finished_at %v, got %v", finishedAt, got.FinishedAt)
	}
}

func TestRepository_HasActiveTrip(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	repo := repoTrip.New(pool)
	ctx := context.Background()

	driverID := uuid.New()
	tripID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)

	hasActive, err := repo.HasActiveTrip(ctx, driverID)
	if err != nil {
		t.Fatalf("unexpected HasActiveTrip error: %v", err)
	}
	if hasActive {
		t.Fatal("expected false for new driver")
	}

	item := model.Trip{
		ID:         tripID,
		UserID:     uuid.New(),
		DriverID:   driverID,
		StartPoint: model.Coordinates{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Coordinates{Latitude: 55.76, Longitude: 37.62},
		Price:      500,
		Status:     model.StatusActive,
		StartedAt:  now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM trip_status_history WHERE trip_id = $1", tripID)
		_, _ = pool.Exec(ctx, "DELETE FROM trips WHERE id = $1", tripID)
	}()

	if err := repo.Save(ctx, item); err != nil {
		t.Fatalf("unexpected save error: %v", err)
	}

	hasActive, err = repo.HasActiveTrip(ctx, driverID)
	if err != nil {
		t.Fatalf("unexpected HasActiveTrip error: %v", err)
	}
	if !hasActive {
		t.Fatal("expected true for driver with active trip")
	}

	finishedAt := now.Add(15 * time.Minute)
	item.Status = model.StatusCompleted
	item.FinishedAt = &finishedAt
	if err := repo.Update(ctx, item); err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}

	hasActive, err = repo.HasActiveTrip(ctx, driverID)
	if err != nil {
		t.Fatalf("unexpected HasActiveTrip error: %v", err)
	}
	if hasActive {
		t.Fatal("expected false after trip completed")
	}
}