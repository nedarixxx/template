//go:build integration

package trip_test

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
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
		u := url.URL{
			Scheme:   "postgres",
			User:     url.UserPassword("tripgo", "tripgo"),
			Host:     "localhost:22032",
			Path:     "tripgo",
			RawQuery: "sslmode=disable",
		}
		dbURL = u.String()
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

func TestRepository_SavePositionAndLastPositionAt(t *testing.T) {
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
		Price:      600,
		Status:     model.StatusActive,
		StartedAt:  now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM trip_positions WHERE trip_id = $1", tripID)
		_, _ = pool.Exec(ctx, "DELETE FROM trips WHERE id = $1", tripID)
	}()

	if err := repo.Save(ctx, item); err != nil {
		t.Fatalf("unexpected save error: %v", err)
	}

	t1 := now.Add(5 * time.Minute)
	pos1, err := repo.SavePosition(ctx, model.TripPosition{
		TripID:      tripID,
		Coordinates: model.Coordinates{Latitude: 55.751, Longitude: 37.611},
		RecordedAt:  t1,
	})
	if err != nil {
		t.Fatalf("unexpected save position 1 error: %v", err)
	}
	if pos1.ID == 0 {
		t.Fatalf("expected non-zero ID for position")
	}

	if err := repo.UpdateLastPositionAt(ctx, tripID, t1); err != nil {
		t.Fatalf("unexpected update last position error: %v", err)
	}

	tripAfter1, err := repo.GetByID(ctx, tripID)
	if err != nil {
		t.Fatalf("unexpected get trip: %v", err)
	}
	if tripAfter1.LastPositionAt == nil || !tripAfter1.LastPositionAt.Equal(t1) {
		t.Fatalf("expected last position %v, got %v", t1, tripAfter1.LastPositionAt)
	}

	t2 := now.Add(10 * time.Minute)
	_, err = repo.SavePosition(ctx, model.TripPosition{
		TripID:      tripID,
		Coordinates: model.Coordinates{Latitude: 55.752, Longitude: 37.612},
		RecordedAt:  t2,
	})
	if err != nil {
		t.Fatalf("unexpected save position 2 error: %v", err)
	}
	if err := repo.UpdateLastPositionAt(ctx, tripID, t2); err != nil {
		t.Fatalf("unexpected update last position error: %v", err)
	}

	tripAfter2, err := repo.GetByID(ctx, tripID)
	if err != nil {
		t.Fatalf("unexpected get trip: %v", err)
	}
	if tripAfter2.LastPositionAt == nil || !tripAfter2.LastPositionAt.Equal(t2) {
		t.Fatalf("expected last position %v, got %v", t2, tripAfter2.LastPositionAt)
	}

	tOlder := now.Add(2 * time.Minute)
	_, err = repo.SavePosition(ctx, model.TripPosition{
		TripID:      tripID,
		Coordinates: model.Coordinates{Latitude: 55.7505, Longitude: 37.6105},
		RecordedAt:  tOlder,
	})
	if err != nil {
		t.Fatalf("unexpected save older position error: %v", err)
	}
	if err := repo.UpdateLastPositionAt(ctx, tripID, tOlder); err != nil {
		t.Fatalf("unexpected update last position error: %v", err)
	}

	tripAfterOlder, err := repo.GetByID(ctx, tripID)
	if err != nil {
		t.Fatalf("unexpected get trip: %v", err)
	}
	if tripAfterOlder.LastPositionAt == nil || !tripAfterOlder.LastPositionAt.Equal(t2) {
		t.Fatalf("expected last position to stay %v, got %v", t2, tripAfterOlder.LastPositionAt)
	}

	positions, err := repo.GetPositions(ctx, tripID)
	if err != nil {
		t.Fatalf("unexpected get positions error: %v", err)
	}
	if len(positions) != 3 {
		t.Fatalf("expected 3 positions, got %d", len(positions))
	}
	if !positions[0].RecordedAt.Equal(tOlder) || !positions[1].RecordedAt.Equal(t1) || !positions[2].RecordedAt.Equal(t2) {
		t.Fatalf("positions not sorted by recorded_at asc: %+v", positions)
	}
}

func TestRepository_SavePosition_RollbackOnFailure(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	repo := repoTrip.New(pool)
	txManager := transaction.NewManager(pool)
	ctx := context.Background()

	tripID := uuid.New()
	driverID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)

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
		_, _ = pool.Exec(ctx, "DELETE FROM trip_positions WHERE trip_id = $1", tripID)
		_, _ = pool.Exec(ctx, "DELETE FROM trips WHERE id = $1", tripID)
	}()

	if err := repo.Save(ctx, item); err != nil {
		t.Fatalf("unexpected save error: %v", err)
	}

	errSimulated := errors.New("rollback simulated")
	tPos := now.Add(5 * time.Minute)

	err := txManager.Do(ctx, func(txCtx context.Context) error {
		if _, err := repo.SavePosition(txCtx, model.TripPosition{
			TripID:      tripID,
			Coordinates: model.Coordinates{Latitude: 55.751, Longitude: 37.611},
			RecordedAt:  tPos,
		}); err != nil {
			return err
		}
		if err := repo.UpdateLastPositionAt(txCtx, tripID, tPos); err != nil {
			return err
		}
		return errSimulated
	})

	if !errors.Is(err, errSimulated) {
		t.Fatalf("expected simulated error, got %v", err)
	}

	positions, err := repo.GetPositions(ctx, tripID)
	if err != nil {
		t.Fatalf("unexpected get positions: %v", err)
	}
	if len(positions) != 0 {
		t.Fatalf("expected 0 positions after rollback, got %d", len(positions))
	}

	tripAfter, err := repo.GetByID(ctx, tripID)
	if err != nil {
		t.Fatalf("unexpected get trip: %v", err)
	}
	if tripAfter.LastPositionAt != nil {
		t.Fatalf("expected LastPositionAt to remain nil, got %v", tripAfter.LastPositionAt)
	}
}

func TestRepository_GetStaleTrips(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	repo := repoTrip.New(pool)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	staleTripID := uuid.New()
	freshTripID := uuid.New()
	completedTripID := uuid.New()

	tripStale := model.Trip{
		ID:         staleTripID,
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: model.Coordinates{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Coordinates{Latitude: 55.76, Longitude: 37.62},
		Price:      500,
		Status:     model.StatusActive,
		StartedAt:  now.Add(-2 * time.Hour),
		CreatedAt:  now.Add(-2 * time.Hour),
		UpdatedAt:  now.Add(-2 * time.Hour),
	}

	tripFresh := model.Trip{
		ID:         freshTripID,
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: model.Coordinates{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Coordinates{Latitude: 55.76, Longitude: 37.62},
		Price:      500,
		Status:     model.StatusActive,
		StartedAt:  now.Add(-2 * time.Hour),
		CreatedAt:  now.Add(-2 * time.Hour),
		UpdatedAt:  now.Add(-2 * time.Hour),
	}

	tripCompleted := model.Trip{
		ID:         completedTripID,
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: model.Coordinates{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Coordinates{Latitude: 55.76, Longitude: 37.62},
		Price:      500,
		Status:     model.StatusActive,
		StartedAt:  now.Add(-2 * time.Hour),
		CreatedAt:  now.Add(-2 * time.Hour),
		UpdatedAt:  now.Add(-2 * time.Hour),
	}

	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM trip_positions WHERE trip_id IN ($1, $2, $3)", staleTripID, freshTripID, completedTripID)
		_, _ = pool.Exec(ctx, "DELETE FROM trips WHERE id IN ($1, $2, $3)", staleTripID, freshTripID, completedTripID)
	}()

	if err := repo.Save(ctx, tripStale); err != nil {
		t.Fatalf("failed to save tripStale: %v", err)
	}
	if err := repo.Save(ctx, tripFresh); err != nil {
		t.Fatalf("failed to save tripFresh: %v", err)
	}
	if err := repo.Save(ctx, tripCompleted); err != nil {
		t.Fatalf("failed to save tripCompleted: %v", err)
	}

	finishedAt := now.Add(-1 * time.Hour)
	tripCompleted.Status = model.StatusCompleted
	tripCompleted.FinishedAt = &finishedAt
	tripCompleted.UpdatedAt = finishedAt
	if err := repo.Update(ctx, tripCompleted); err != nil {
		t.Fatalf("failed to update tripCompleted: %v", err)
	}

	freshPositionTime := now.Add(-10 * time.Second)
	if _, err := repo.SavePosition(ctx, model.TripPosition{
		TripID:      freshTripID,
		Coordinates: model.Coordinates{Latitude: 55.75, Longitude: 37.61},
		RecordedAt:  freshPositionTime,
	}); err != nil {
		t.Fatalf("failed to save position for fresh trip: %v", err)
	}
	if err := repo.UpdateLastPositionAt(ctx, freshTripID, freshPositionTime); err != nil {
		t.Fatalf("failed to update last position for fresh trip: %v", err)
	}

	staleTrips, err := repo.GetStaleTrips(ctx, 60*time.Second, 100, time.Time{}, uuid.Nil)
	if err != nil {
		t.Fatalf("failed to get stale trips: %v", err)
	}

	var foundStale, foundFresh, foundCompleted bool
	for _, st := range staleTrips {
		if st.ID == staleTripID {
			foundStale = true
		}
		if st.ID == freshTripID {
			foundFresh = true
		}
		if st.ID == completedTripID {
			foundCompleted = true
		}
	}

	if !foundStale {
		t.Fatalf("expected stale trip %v to be in stale trips list", staleTripID)
	}
	if foundFresh {
		t.Fatalf("fresh trip %v should not be in stale trips list", freshTripID)
	}
	if foundCompleted {
		t.Fatalf("completed trip %v should not be in stale trips list", completedTripID)
	}
}

func TestRepository_GetStaleTrips_ExplainIndexScan(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()

	_, _ = pool.Exec(ctx, "SET enable_seqscan = off")
	defer func() {
		_, _ = pool.Exec(ctx, "SET enable_seqscan = on")
	}()

	query := `EXPLAIN SELECT id, driver_id, COALESCE(last_position_at, started_at) AS stale_at
FROM trips
WHERE status = 'active'
  AND COALESCE(last_position_at, started_at) <= now() - make_interval(secs => 60)
ORDER BY COALESCE(last_position_at, started_at) ASC, id ASC
LIMIT 10;`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		t.Fatalf("failed to explain query: %v", err)
	}
	defer rows.Close()

	var planText string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan explain line: %v", err)
		}
		planText += line + "\n"
	}

	if !strings.Contains(planText, "trips_stale_active_idx") {
		t.Fatalf("expected explain plan to use trips_stale_active_idx, plan was:\n%s", planText)
	}
}
