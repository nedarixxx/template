package domainerror_test

import (
	"errors"
	"net/http"
	"testing"

	domaintrip "github.com/nedarixxx/template/internal/domain/trip"
	"github.com/nedarixxx/template/internal/http/domainerror"
)

func TestProblem(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		wantStatus     int
		wantCode       string
		wantTitle      string
		wantDetail     string
		forbidInDetail string 
	}{
		{
			name:       "trip not found error",
			err:        domaintrip.ErrTripNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "trip_not_found",
			wantTitle:  "Not Found",
			wantDetail: "Trip was not found",
		},
		{
			name:       "driver busy conflict",
			err:        domaintrip.ErrDriverBusy,
			wantStatus: http.StatusConflict,
			wantCode:   "driver_busy",
			wantTitle:  "Driver busy",
			wantDetail: "Driver already has an active trip",
		},
		{
			name:       "trip already completed conflict",
			err:        domaintrip.ErrTripCompleted,
			wantStatus: http.StatusConflict,
			wantCode:   "trip_completed",
			wantTitle:  "Trip completed",
			wantDetail: "Operation is not allowed for a completed trip",
		},
		{
			name: "validation error returns 400",
			err: domaintrip.ValidationError{
				Field:   "price",
				Message: "must be positive",
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
			wantTitle:  "Invalid Request",
			wantDetail: "price: must be positive",
		},
		{
			name:       "invalid request JSON decode error",
			err:        domainerror.ErrInvalidRequest,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
			wantTitle:  "Invalid Request",
			wantDetail: "Request body is invalid JSON",
		},
		{
			name:           "internal db error masked to 500 without leaking details",
			err:            errors.New("fatal: pq connection lost on pg_catalog"),
			wantStatus:     http.StatusInternalServerError,
			wantCode:       "internal_error",
			wantTitle:      "Internal Server Error",
			wantDetail:     "Internal server error",
			forbidInDetail: "pq connection lost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, prob := domainerror.Problem(tt.err)

			if status != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, status)
			}
			if prob.Code != tt.wantCode {
				t.Fatalf("expected code %q, got %q", tt.wantCode, prob.Code)
			}
			if prob.Title != tt.wantTitle {
				t.Fatalf("expected title %q, got %q", tt.wantTitle, prob.Title)
			}
			if prob.Detail == nil || *prob.Detail != tt.wantDetail {
				t.Fatalf("expected detail %q, got %v", tt.wantDetail, prob.Detail)
			}
			if tt.forbidInDetail != "" && prob.Detail != nil {
				if *prob.Detail == tt.forbidInDetail {
					t.Fatalf("leaked internal detail: %s", *prob.Detail)
				}
			}
		})
	}
}