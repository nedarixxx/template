package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/nedarixxx/template/api"
	"github.com/nedarixxx/template/internal/model"
	"github.com/nedarixxx/template/internal/service"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type Handler struct {
	tripService *service.TripService
	pinger      Pinger
}

func New(tripService *service.TripService, pinger Pinger) *Handler {
	return &Handler{
		tripService: tripService,
		pinger:      pinger,
	}
}

// POST /api/v1/trips
func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	var req api.TripData
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Invalid Request", "Request body is invalid JSON", "invalid_request")
		return
	}

	dto := service.CreateTripDTO{
		UserID:   req.UserId,
		DriverID: req.DriverId,
		StartPoint: model.Coordinates{
			Latitude:  req.StartPoint.Latitude,
			Longitude: req.StartPoint.Longitude,
		},
		EndPoint: model.Coordinates{
			Latitude:  req.EndPoint.Latitude,
			Longitude: req.EndPoint.Longitude,
		},
		Price: req.Price,
	}

	trip, err := h.tripService.CreateTrip(r.Context(), dto)
	if err != nil {
		if errors.Is(err, model.ErrDriverBusy) {
			writeProblem(w, r, http.StatusConflict, "Driver busy", "Driver already has an active trip", "driver_busy")
			return
		}
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "Internal server error", "internal_error")
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/api/v1/trips/%s", trip.ID))
	writeJSON(w, http.StatusCreated, toAPITrip(trip))
}

// GET /api/v1/trips/{tripId}
func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	trip, err := h.tripService.GetTrip(r.Context(), tripID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "Trip was not found", "trip_not_found")
			return
		}
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "Internal server error", "internal_error")
		return
	}

	writeJSON(w, http.StatusOK, toAPITrip(trip))
}

// POST /api/v1/trips/{tripId}/finish
func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	trip, err := h.tripService.CompleteTrip(r.Context(), tripID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "Trip was not found", "trip_not_found")
			return
		}
		if errors.Is(err, model.ErrAlreadyCompleted) {
			writeProblem(w, r, http.StatusConflict, "Trip completed", "Operation is not allowed for a completed trip", "trip_completed")
			return
		}
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "Internal server error", "internal_error")
		return
	}

	writeJSON(w, http.StatusOK, toAPITrip(trip))
}

// GET /health
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

// GET /ready
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	if err := h.pinger.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, api.HealthResponse{Status: api.Unavailable})
		return
	}
	writeJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func (h *Handler) ListTripPositions(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	writeProblem(w, r, http.StatusNotImplemented, "Not Implemented", "Position tracking will be implemented in lab 3", "not_implemented")
}

func (h *Handler) CreateTripPosition(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	writeProblem(w, r, http.StatusNotImplemented, "Not Implemented", "Position tracking will be implemented in lab 3", "not_implemented")
}

func toAPITrip(t *model.Trip) api.Trip {
	return api.Trip{
		Id:       t.ID,
		UserId:   t.UserID,
		DriverId: t.DriverID,
		StartPoint: api.Coordinates{
			Latitude:  t.StartPoint.Latitude,
			Longitude: t.StartPoint.Longitude,
		},
		EndPoint: api.Coordinates{
			Latitude:  t.EndPoint.Latitude,
			Longitude: t.EndPoint.Longitude,
		},
		Price:      t.Price,
		Status:     api.TripStatus(t.Status),
		StartedAt:  t.StartedAt,
		FinishedAt: t.FinishedAt,
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeProblem(w http.ResponseWriter, r *http.Request, status int, title, detail, code string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	instance := r.URL.Path
	problemType := fmt.Sprintf("https://tripgo.example/problems/%s", code)
	_ = json.NewEncoder(w).Encode(api.Problem{
		Type:     problemType,
		Title:    title,
		Status:   int32(status),
		Detail:   &detail,
		Code:     code,
		Instance: &instance,
	})
}