package trip

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
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
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
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