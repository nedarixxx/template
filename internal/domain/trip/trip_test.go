package trip_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nedarixxx/template/internal/domain/trip"
)

func TestTrip_Finish(t *testing.T) {
	now := time.Now().UTC()
	finishTime := now.Add(20 * time.Minute)

	tests := []struct {
		name          string
		initialStatus trip.Status
		wantErr       error
		wantStatus    trip.Status
	}{
		{
			name:          "success finish active trip",
			initialStatus: trip.StatusActive,
			wantErr:       nil,
			wantStatus:    trip.StatusCompleted,
		},
		{
			name:          "fail finish already completed trip",
			initialStatus: trip.StatusCompleted,
			wantErr:       trip.ErrTripCompleted,
			wantStatus:    trip.StatusCompleted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &trip.Trip{
				ID:        uuid.New(),
				Status:    tt.initialStatus,
				StartedAt: now,
			}

			err := tr.Finish(finishTime)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}

			if tr.Status != tt.wantStatus {
				t.Fatalf("expected status %v, got %v", tt.wantStatus, tr.Status)
			}

			if tt.wantErr == nil {
				if tr.FinishedAt == nil || !tr.FinishedAt.Equal(finishTime) {
					t.Fatalf("expected FinishedAt %v, got %v", finishTime, tr.FinishedAt)
				}
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	err := trip.ValidationError{
		Field:   "price",
		Message: "must be positive",
	}

	expected := "price: must be positive"
	if err.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
}