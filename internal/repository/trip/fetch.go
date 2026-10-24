package trip

import (
	"context"
	"errors"
	"fmt"

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
		"price", "status", "started_at", "finished_at", "created_at", "updated_at",
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
		&d.Price, &d.Status, &d.StartedAt, &d.FinishedAt, &d.CreatedAt, &d.UpdatedAt,
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
			"status":    "active",
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