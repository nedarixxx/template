package trip

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	domaintrip "github.com/nedarixxx/template/internal/domain/trip"
	"github.com/nedarixxx/template/internal/model"
)

func (r *Repository) Save(ctx context.Context, trip model.Trip) error {
	exec := r.getExecutor(ctx)

	query, args, err := psql.Insert("trips").
		Columns(
			"id", "user_id", "driver_id",
			"start_latitude", "start_longitude",
			"end_latitude", "end_longitude",
			"price", "status", "started_at", "created_at", "updated_at",
		).
		Values(
			trip.ID, trip.UserID, trip.DriverID,
			trip.StartPoint.Latitude, trip.StartPoint.Longitude,
			trip.EndPoint.Latitude, trip.EndPoint.Longitude,
			trip.Price, string(trip.Status), trip.StartedAt, trip.CreatedAt, trip.UpdatedAt,
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert trip query: %w", err)
	}

	_, err = exec.Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "trips_driver_active_idx" {
			return domaintrip.ErrDriverBusy
		}
		return fmt.Errorf("exec insert trip: %w", err)
	}

	return nil
}

func (r *Repository) Update(ctx context.Context, trip model.Trip) error {
	exec := r.getExecutor(ctx)

	query, args, err := psql.Update("trips").
		Set("status", string(trip.Status)).
		Set("finished_at", trip.FinishedAt).
		Set("updated_at", trip.UpdatedAt).
		Where(sq.Eq{"id": trip.ID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build update trip query: %w", err)
	}

	tag, err := exec.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("exec update trip: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return domaintrip.ErrTripNotFound
	}

	return nil
}

func (r *Repository) AddStatusHistory(ctx context.Context, h model.TripStatusHistory) error {
	exec := r.getExecutor(ctx)

	query, args, err := psql.Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "reason", "changed_at").
		Values(h.TripID, h.FromStatus, string(h.ToStatus), h.Reason, h.ChangedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert status history query: %w", err)
	}

	if _, err := exec.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("exec insert status history: %w", err)
	}

	return nil
}

func (r *Repository) SavePosition(ctx context.Context, pos model.TripPosition) (model.TripPosition, error) {
	exec := r.getExecutor(ctx)

	query, args, err := psql.Insert("trip_positions").
		Columns("trip_id", "latitude", "longitude", "recorded_at").
		Values(pos.TripID, pos.Coordinates.Latitude, pos.Coordinates.Longitude, pos.RecordedAt).
		Suffix("RETURNING id, created_at").
		ToSql()
	if err != nil {
		return model.TripPosition{}, fmt.Errorf("build insert position query: %w", err)
	}

	var (
		id        int64
		createdAt time.Time
	)
	if err := exec.QueryRow(ctx, query, args...).Scan(&id, &createdAt); err != nil {
		return model.TripPosition{}, fmt.Errorf("exec insert position: %w", err)
	}

	pos.ID = id
	pos.CreatedAt = createdAt
	return pos, nil
}

func (r *Repository) UpdateLastPositionAt(ctx context.Context, tripID uuid.UUID, recordedAt time.Time) error {
	exec := r.getExecutor(ctx)

	query, args, err := psql.Update("trips").
		Set("last_position_at", sq.Expr("CASE WHEN last_position_at IS NULL OR ? > last_position_at THEN ? ELSE last_position_at END", recordedAt, recordedAt)).
		Set("updated_at", sq.Expr("now()")).
		Where(sq.Eq{
			"id":      tripID,
			colStatus: string(model.StatusActive),
		}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build update last position query: %w", err)
	}

	tag, err := exec.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("exec update last position: %w", err)
	}

	if tag.RowsAffected() == 0 {
		if _, err := r.GetByID(ctx, tripID); err != nil {
			return domaintrip.ErrTripNotFound
		}
		return domaintrip.ErrTripCompleted
	}

	return nil
}
