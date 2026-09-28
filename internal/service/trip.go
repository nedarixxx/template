package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nedarixxx/template/internal/database"
	"github.com/nedarixxx/template/internal/model"
)

type TripRepository interface {
	Create(ctx context.Context, trip *model.Trip) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Trip, error)
	Complete(ctx context.Context, id uuid.UUID, finishedAt time.Time) error
	AddHistoryStatus(ctx context.Context, h *model.TripStatusHistory) error
}

type TripService struct {
	repo       TripRepository
	transactor database.TxManager
}

func NewTripService(repo TripRepository, transactor database.TxManager) *TripService {
	return &TripService{
		repo:       repo,
		transactor: transactor,
	}
}

type CreateTripDTO struct {
	UserID     uuid.UUID
	DriverID   uuid.UUID
	StartPoint model.Coordinates
	EndPoint   model.Coordinates
	Price      int64
}

func (s *TripService) CreateTrip(ctx context.Context, dto CreateTripDTO) (*model.Trip, error) {
	now := time.Now().UTC()
	trip := &model.Trip{
		ID:         uuid.New(),
		UserID:     dto.UserID,
		DriverID:   dto.DriverID,
		StartPoint: dto.StartPoint,
		EndPoint:   dto.EndPoint,
		Price:      dto.Price,
		Status:     model.StatusActive,
		StartedAt:  now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	err := s.transactor.Do(ctx, func(txCtx context.Context) error {
		if err := s.repo.Create(txCtx, trip); err != nil {
			return err
		}

		history := &model.TripStatusHistory{
			TripID:     trip.ID,
			FromStatus: nil,
			ToStatus:   model.StatusActive,
			Reason:     nil,
			ChangedAt:  now,
		}
		if err := s.repo.AddHistoryStatus(txCtx, history); err != nil {
			return fmt.Errorf("add initial status history: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return trip, nil
}

func (s *TripService) GetTrip(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *TripService) CompleteTrip(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	finishedAt := time.Now().UTC()
	reason := "completed by client"
	activeStatus := model.StatusActive

	err := s.transactor.Do(ctx, func(txCtx context.Context) error {
		if err := s.repo.Complete(txCtx, id, finishedAt); err != nil {
			return err
		}

		history := &model.TripStatusHistory{
			TripID:     id,
			FromStatus: &activeStatus,
			ToStatus:   model.StatusCompleted,
			Reason:     &reason,
			ChangedAt:  finishedAt,
		}
		if err := s.repo.AddHistoryStatus(txCtx, history); err != nil {
			return fmt.Errorf("add complete status history: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, id)
}