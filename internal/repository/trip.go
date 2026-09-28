package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/nedarixxx/template/internal/database"
	"github.com/nedarixxx/template/internal/model"
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

type TripRepository struct {
	db *database.DB
}

func NewTripRepository(db *database.DB) *TripRepository {
	return &TripRepository{db: db}
}

func (r *TripRepository) Create(ctx context.Context, trip *model.Trip) error {
	exec := r.db.GetExecutor(ctx)

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
			return model.ErrDriverBusy
		}
		return fmt.Errorf("exec insert trip: %w", err)
	}

	return nil
}

func (r *TripRepository) AddHistoryStatus(ctx context.Context, h *model.TripStatusHistory) error {
	exec := r.db.GetExecutor(ctx)

	query, args, err := psql.Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "reason", "changed_at").
		Values(h.TripID, h.FromStatus, string(h.ToStatus), h.Reason, h.ChangedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert status history query: %w", err)
	}

	_, err = exec.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("exec insert status history: %w", err)
	}

	return nil
}

func (r *TripRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	exec := r.db.GetExecutor(ctx)

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
		return nil, fmt.Errorf("build select trip query: %w", err)
	}

	var t model.Trip
	var statusStr string

	err = exec.QueryRow(ctx, query, args...).Scan(
		&t.ID, &t.UserID, &t.DriverID,
		&t.StartPoint.Latitude, &t.StartPoint.Longitude,
		&t.EndPoint.Latitude, &t.EndPoint.Longitude,
		&t.Price, &statusStr, &t.StartedAt, &t.FinishedAt, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("scan trip: %w", err)
	}

	t.Status = model.TripStatus(statusStr)
	return &t, nil
}

func (r *TripRepository) Complete(ctx context.Context, id uuid.UUID, finishedAt time.Time) error {
	exec := r.db.GetExecutor(ctx)

	query, args, err := psql.Update("trips").
		Set("status", string(model.StatusCompleted)).
		Set("finished_at", finishedAt).
		Set("updated_at", time.Now().UTC()).
		Where(sq.Eq{"id": id, "status": string(model.StatusActive)}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build complete trip query: %w", err)
	}

	tag, err := exec.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("exec complete trip: %w", err)
	}

	if tag.RowsAffected() == 0 {
		t, err := r.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if t.Status == model.StatusCompleted {
			return model.ErrAlreadyCompleted
		}
		return model.ErrNotFound
	}

	return nil
}