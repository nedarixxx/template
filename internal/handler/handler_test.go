package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nedarixxx/template/api"
	"github.com/nedarixxx/template/internal/handler"
	"github.com/nedarixxx/template/internal/model"
	"github.com/nedarixxx/template/internal/service"
)

type mockRepo struct {
	createFunc func(ctx context.Context, trip *model.Trip) error
	getByIDFunc func(ctx context.Context, id uuid.UUID) (*model.Trip, error)
	completeFunc func(ctx context.Context, id uuid.UUID, finishedAt time.Time) error
	addHistoryFunc func(ctx context.Context, h *model.TripStatusHistory) error
}

func (m *mockRepo) Create(ctx context.Context, trip *model.Trip) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, trip)
	}
	return nil
}

func (m *mockRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockRepo) Complete(ctx context.Context, id uuid.UUID, finishedAt time.Time) error {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, id, finishedAt)
	}
	return nil
}

func (m *mockRepo) AddHistoryStatus(ctx context.Context, h *model.TripStatusHistory) error {
	if m.addHistoryFunc != nil {
		return m.addHistoryFunc(ctx, h)
	}
	return nil
}

type mockTxManager struct{}

func (m *mockTxManager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type mockPinger struct {
	err error
}

func (m *mockPinger) Ping(ctx context.Context) error {
	return m.err
}

func setupTestRouter(repo *mockRepo, pinger handler.Pinger) http.Handler {
	tx := &mockTxManager{}
	svc := service.NewTripService(repo, tx)
	h := handler.New(svc, pinger)

	r := chi.NewRouter()
	return api.HandlerWithOptions(h, api.ChiServerOptions{
		BaseRouter: r,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			handler.WriteProblem(w, r, http.StatusBadRequest, "Invalid request", "Request validation failed", "invalid_request")
		},
	})
}

func TestCreateTrip_Validation(t *testing.T) {
	repo := &mockRepo{}
	router := setupTestRouter(repo, &mockPinger{})

	validPayload := `{
		"user_id": "5cb72c04-7650-45c9-a79b-bcdba0631e0c",
		"driver_id": "8860b315-ec86-42eb-a17c-7c163d721ff5",
		"start_point": {"latitude": 59.9398, "longitude": 30.3146},
		"end_point": {"latitude": 59.9290, "longitude": 30.3626},
		"price": 1450
	}`

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "valid request",
			body:       validPayload,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "empty body",
			body:       "",
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "empty object",
			body:       "{}",
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "unknown field",
			body:       `{"user_id": "5cb72c04-7650-45c9-a79b-bcdba0631e0c", "driver_id": "8860b315-ec86-42eb-a17c-7c163d721ff5", "start_point": {"latitude": 59.9398, "longitude": 30.3146}, "end_point": {"latitude": 59.9290, "longitude": 30.3626}, "price": 1450, "extra": "field"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "nil user_id",
			body:       `{"user_id": "00000000-0000-0000-0000-000000000000", "driver_id": "8860b315-ec86-42eb-a17c-7c163d721ff5", "start_point": {"latitude": 59.9398, "longitude": 30.3146}, "end_point": {"latitude": 59.9290, "longitude": 30.3626}, "price": 1450}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "nil driver_id",
			body:       `{"user_id": "5cb72c04-7650-45c9-a79b-bcdba0631e0c", "driver_id": "00000000-0000-0000-0000-000000000000", "start_point": {"latitude": 59.9398, "longitude": 30.3146}, "end_point": {"latitude": 59.9290, "longitude": 30.3626}, "price": 1450}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "invalid latitude over 90",
			body:       `{"user_id": "5cb72c04-7650-45c9-a79b-bcdba0631e0c", "driver_id": "8860b315-ec86-42eb-a17c-7c163d721ff5", "start_point": {"latitude": 90.1, "longitude": 30.3146}, "end_point": {"latitude": 59.9290, "longitude": 30.3626}, "price": 1450}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "invalid longitude over 180",
			body:       `{"user_id": "5cb72c04-7650-45c9-a79b-bcdba0631e0c", "driver_id": "8860b315-ec86-42eb-a17c-7c163d721ff5", "start_point": {"latitude": 59.9398, "longitude": 180.1}, "end_point": {"latitude": 59.9290, "longitude": 30.3626}, "price": 1450}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "negative price",
			body:       `{"user_id": "5cb72c04-7650-45c9-a79b-bcdba0631e0c", "driver_id": "8860b315-ec86-42eb-a17c-7c163d721ff5", "start_point": {"latitude": 59.9398, "longitude": 30.3146}, "end_point": {"latitude": 59.9290, "longitude": 30.3626}, "price": -10}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "trailing json data",
			body:       validPayload + ` {"extra": 1}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d. Body: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			if tt.wantCode != "" {
				if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
					t.Errorf("expected Content-Type application/problem+json, got %s", ct)
				}
				var p api.Problem
				if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
					t.Fatalf("failed to unmarshal Problem response: %v", err)
				}
				if p.Code != tt.wantCode {
					t.Errorf("expected code %s, got %s", tt.wantCode, p.Code)
				}
			}
		})
	}
}

func TestChiServerErrorHandler_InvalidPathUUID(t *testing.T) {
	repo := &mockRepo{}
	router := setupTestRouter(repo, &mockPinger{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/not-a-valid-uuid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("expected Content-Type application/problem+json, got %s", ct)
	}

	var p api.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatalf("failed to decode Problem: %v", err)
	}
	if p.Code != "invalid_request" {
		t.Errorf("expected code invalid_request, got %s", p.Code)
	}
	if !strings.Contains(p.Type, "invalid-request") {
		t.Errorf("expected type to contain invalid-request, got %s", p.Type)
	}
}

func TestCreateTrip_DriverBusy(t *testing.T) {
	repo := &mockRepo{
		createFunc: func(ctx context.Context, trip *model.Trip) error {
			return model.ErrDriverBusy
		},
	}
	router := setupTestRouter(repo, &mockPinger{})

	validPayload := `{
		"user_id": "5cb72c04-7650-45c9-a79b-bcdba0631e0c",
		"driver_id": "8860b315-ec86-42eb-a17c-7c163d721ff5",
		"start_point": {"latitude": 59.9398, "longitude": 30.3146},
		"end_point": {"latitude": 59.9290, "longitude": 30.3626},
		"price": 1450
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewBufferString(validPayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", w.Code)
	}

	var p api.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatalf("failed to decode Problem: %v", err)
	}
	if p.Code != "driver_busy" {
		t.Errorf("expected code driver_busy, got %s", p.Code)
	}
}

func TestHealthAndReady(t *testing.T) {
	repo := &mockRepo{}
	pinger := &mockPinger{}
	router := setupTestRouter(repo, pinger)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected /health 200, got %d", w.Code)
	}

	reqReady := httptest.NewRequest(http.MethodGet, "/ready", nil)
	wReady := httptest.NewRecorder()
	router.ServeHTTP(wReady, reqReady)
	if wReady.Code != http.StatusOK {
		t.Errorf("expected /ready 200, got %d", wReady.Code)
	}

	pinger.err = errors.New("db down")
	reqReadyDown := httptest.NewRequest(http.MethodGet, "/ready", nil)
	wReadyDown := httptest.NewRecorder()
	router.ServeHTTP(wReadyDown, reqReadyDown)
	if wReadyDown.Code != http.StatusServiceUnavailable {
		t.Errorf("expected /ready 503, got %d", wReadyDown.Code)
	}
}
