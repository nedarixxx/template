package trip

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"

	domaintrip "github.com/nedarixxx/template/internal/domain/trip"
	"github.com/nedarixxx/template/internal/model"
)

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (model.Trip, error) {
	exec := r.getExecutor(ctx)

	query, args, err := psql.Select(
		"id", "user_id", "driver_id",
		"start_latitude", "start_longitude",
		"end_latitude", "end_longitude",
		"price", "status", "started_at", "finished_at", "last_position_at", "created_at", "updated_at",
	).
		From("trips").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return model.Trip{}, fmt.Errorf("build select trip query: %w", err)
	}

	var d dbTrip
	err = exec.QueryRow(ctx, query, args...).Scan(
		&d.ID, &d.UserID, &d.DriverID,
		&d.StartLatitude, &d.StartLongitude,
		&d.EndLatitude, &d.EndLongitude,
		&d.Price, &d.Status, &d.StartedAt, &d.FinishedAt, &d.LastPositionAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Trip{}, domaintrip.ErrTripNotFound
		}
		return model.Trip{}, fmt.Errorf("scan trip: %w", err)
	}

	return toModel(d), nil
}

func (r *Repository) HasActiveTrip(ctx context.Context, driverID uuid.UUID) (bool, error) {
	exec := r.getExecutor(ctx)

	query, args, err := psql.Select("count(*) > 0").
		From("trips").
		Where(sq.Eq{
			"driver_id": driverID,
			colStatus:   string(model.StatusActive),
		}).
		ToSql()
	if err != nil {
		return false, fmt.Errorf("build has active trip query: %w", err)
	}

	var exists bool
	if err := exec.QueryRow(ctx, query, args...).Scan(&exists); err != nil {
		return false, fmt.Errorf("check active trip: %w", err)
	}

	return exists, nil
}

func (r *Repository) GetPositions(ctx context.Context, tripID uuid.UUID) ([]model.TripPosition, error) {
	exec := r.getExecutor(ctx)

	query, args, err := psql.Select(
		"id", "trip_id", "latitude", "longitude", "recorded_at", "created_at",
	).
		From("trip_positions").
		Where(sq.Eq{"trip_id": tripID}).
		OrderBy("recorded_at ASC", "id ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select positions query: %w", err)
	}

	rows, err := exec.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("exec select positions query: %w", err)
	}
	defer rows.Close()

	positions := make([]model.TripPosition, 0)
	for rows.Next() {
		var p model.TripPosition
		if err := rows.Scan(
			&p.ID,
			&p.TripID,
			&p.Coordinates.Latitude,
			&p.Coordinates.Longitude,
			&p.RecordedAt,
			&p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan position: %w", err)
		}
		positions = append(positions, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate positions rows: %w", err)
	}

	return positions, nil
}

func (r *Repository) GetStaleTrips(
	ctx context.Context,
	staleAfter time.Duration,
	limit int,
	lastStaleAt time.Time,
	lastID uuid.UUID,
) ([]model.StaleTrip, error) {
	exec := r.getExecutor(ctx)

	staleSecs := int64(staleAfter.Seconds())
	if staleSecs <= 0 {
		staleSecs = 1
	}

	builder := psql.Select(
		"id",
		"driver_id",
		"COALESCE(last_position_at, started_at) AS stale_at",
	).
		From("trips").
		Where(sq.Eq{colStatus: string(model.StatusActive)}).
		Where("COALESCE(last_position_at, started_at) <= now() - make_interval(secs => ?)", staleSecs)

	if !lastStaleAt.IsZero() && lastID != uuid.Nil {
		builder = builder.Where(
			sq.Expr("(COALESCE(last_position_at, started_at) > ? OR (COALESCE(last_position_at, started_at) = ? AND id > ?))",
				lastStaleAt, lastStaleAt, lastID),
		)
	}

	builder = builder.OrderBy(
		"COALESCE(last_position_at, started_at) ASC",
		"id ASC",
	)

	if limit > 0 {
		builder = builder.Limit(uint64(limit))
	}

	query, args, err := builder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build stale trips query: %w", err)
	}

	rows, err := exec.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("exec stale trips query: %w", err)
	}
	defer rows.Close()

	var result []model.StaleTrip
	for rows.Next() {
		var item model.StaleTrip
		if err := rows.Scan(&item.ID, &item.DriverID, &item.StaleAt); err != nil {
			return nil, fmt.Errorf("scan stale trip: %w", err)
		}
		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stale trips rows: %w", err)
	}

	return result, nil
}
