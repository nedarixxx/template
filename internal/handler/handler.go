package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/google/uuid"
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

type coordinatesRequest struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

type createTripRequest struct {
	UserID     *uuid.UUID          `json:"user_id"`
	DriverID   *uuid.UUID          `json:"driver_id"`
	StartPoint *coordinatesRequest `json:"start_point"`
	EndPoint   *coordinatesRequest `json:"end_point"`
	Price      *int64              `json:"price"`
}

// POST /api/v1/trips
func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req createTripRequest
	if err := dec.Decode(&req); err != nil {
		WriteProblem(w, r, http.StatusBadRequest, "Invalid request", "Request validation failed", "invalid_request")
		return
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		WriteProblem(w, r, http.StatusBadRequest, "Invalid request", "Request validation failed", "invalid_request")
		return
	}

	if req.UserID == nil || *req.UserID == uuid.Nil ||
		req.DriverID == nil || *req.DriverID == uuid.Nil ||
		req.StartPoint == nil || req.StartPoint.Latitude == nil || req.StartPoint.Longitude == nil ||
		req.EndPoint == nil || req.EndPoint.Latitude == nil || req.EndPoint.Longitude == nil ||
		req.Price == nil || *req.Price < 0 ||
		math.IsNaN(*req.StartPoint.Latitude) || math.IsInf(*req.StartPoint.Latitude, 0) ||
		math.IsNaN(*req.StartPoint.Longitude) || math.IsInf(*req.StartPoint.Longitude, 0) ||
		math.IsNaN(*req.EndPoint.Latitude) || math.IsInf(*req.EndPoint.Latitude, 0) ||
		math.IsNaN(*req.EndPoint.Longitude) || math.IsInf(*req.EndPoint.Longitude, 0) ||
		*req.StartPoint.Latitude < -90 || *req.StartPoint.Latitude > 90 ||
		*req.StartPoint.Longitude < -180 || *req.StartPoint.Longitude > 180 ||
		*req.EndPoint.Latitude < -90 || *req.EndPoint.Latitude > 90 ||
		*req.EndPoint.Longitude < -180 || *req.EndPoint.Longitude > 180 {
		WriteProblem(w, r, http.StatusBadRequest, "Invalid request", "Request validation failed", "invalid_request")
		return
	}

	dto := service.CreateTripDTO{
		UserID:   *req.UserID,
		DriverID: *req.DriverID,
		StartPoint: model.Coordinates{
			Latitude:  *req.StartPoint.Latitude,
			Longitude: *req.StartPoint.Longitude,
		},
		EndPoint: model.Coordinates{
			Latitude:  *req.EndPoint.Latitude,
			Longitude: *req.EndPoint.Longitude,
		},
		Price: *req.Price,
	}

	trip, err := h.tripService.CreateTrip(r.Context(), dto)
	if err != nil {
		if errors.Is(err, model.ErrDriverBusy) {
			WriteProblem(w, r, http.StatusConflict, "Driver busy", "Driver already has an active trip", "driver_busy")
			return
		}
		WriteProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "Internal server error", "internal_error")
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
			WriteProblem(w, r, http.StatusNotFound, "Trip not found", "Trip was not found", "trip_not_found")
			return
		}
		WriteProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "Internal server error", "internal_error")
		return
	}

	writeJSON(w, http.StatusOK, toAPITrip(trip))
}

// POST /api/v1/trips/{tripId}/finish
func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	trip, err := h.tripService.CompleteTrip(r.Context(), tripID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			WriteProblem(w, r, http.StatusNotFound, "Trip not found", "Trip was not found", "trip_not_found")
			return
		}
		if errors.Is(err, model.ErrAlreadyCompleted) {
			WriteProblem(w, r, http.StatusConflict, "Trip completed", "Operation is not allowed for a completed trip", "trip_completed")
			return
		}
		WriteProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "Internal server error", "internal_error")
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
	WriteProblem(w, r, http.StatusNotImplemented, "Not Implemented", "Position tracking will be implemented in lab 3", "not_implemented")
}

func (h *Handler) CreateTripPosition(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	WriteProblem(w, r, http.StatusNotImplemented, "Not Implemented", "Position tracking will be implemented in lab 3", "not_implemented")
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

func WriteProblem(w http.ResponseWriter, r *http.Request, status int, title, detail, code string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	instance := r.URL.Path
	problemType := fmt.Sprintf("https://tripgo.example/problems/%s", strings.ReplaceAll(code, "_", "-"))
	_ = json.NewEncoder(w).Encode(api.Problem{
		Type:     problemType,
		Title:    title,
		Status:   int32(status),
		Detail:   &detail,
		Code:     code,
		Instance: &instance,
	})
}
