package create

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/metric"

	"github.com/nedarixxx/template/api"
	domaintrip "github.com/nedarixxx/template/internal/domain/trip"
	"github.com/nedarixxx/template/internal/http/domainerror"
	"github.com/nedarixxx/template/internal/model"
	usecasecreate "github.com/nedarixxx/template/internal/usecase/trip/create"
)

type Option func(*Handler)

func WithCounter(c metric.Int64Counter) Option {
	return func(h *Handler) {
		h.counter = c
	}
}

type Handler struct {
	useCase useCase
	counter metric.Int64Counter
}

func New(uc useCase, opts ...Option) *Handler {
	h := &Handler{useCase: uc}
	for _, opt := range opts {
		opt(h)
	}
	return h
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

func validateCoordinates(p *coordinatesRequest, field string) error {
	if p == nil || p.Latitude == nil || p.Longitude == nil {
		return domaintrip.ValidationError{Field: field, Message: "latitude and longitude are required"}
	}
	if math.IsNaN(*p.Latitude) || math.IsInf(*p.Latitude, 0) || *p.Latitude < -90 || *p.Latitude > 90 {
		return domaintrip.ValidationError{Field: field + ".latitude", Message: "must be between -90 and 90"}
	}
	if math.IsNaN(*p.Longitude) || math.IsInf(*p.Longitude, 0) || *p.Longitude < -180 || *p.Longitude > 180 {
		return domaintrip.ValidationError{Field: field + ".longitude", Message: "must be between -180 and 180"}
	}
	return nil
}

func validateCreateTripRequest(req *createTripRequest) error {
	if req.UserID == nil || *req.UserID == uuid.Nil {
		return domaintrip.ValidationError{Field: "user_id", Message: "required and must be a valid non-empty UUID"}
	}
	if req.DriverID == nil || *req.DriverID == uuid.Nil {
		return domaintrip.ValidationError{Field: "driver_id", Message: "required and must be a valid non-empty UUID"}
	}
	if err := validateCoordinates(req.StartPoint, "start_point"); err != nil {
		return err
	}
	if err := validateCoordinates(req.EndPoint, "end_point"); err != nil {
		return err
	}
	if req.Price == nil || *req.Price < 0 {
		return domaintrip.ValidationError{Field: "price", Message: "must be non-negative"}
	}
	return nil
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, _ api.CreateTripParams) {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req createTripRequest
	if err := dec.Decode(&req); err != nil {
		domainerror.Write(w, r, domainerror.ErrInvalidRequest)
		return
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		domainerror.Write(w, r, domainerror.ErrInvalidRequest)
		return
	}

	if err := validateCreateTripRequest(&req); err != nil {
		domainerror.Write(w, r, err)
		return
	}

	cmd := usecasecreate.Command{
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

	trip, err := h.useCase.Execute(r.Context(), cmd)
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	if h.counter != nil {
		h.counter.Add(r.Context(), 1)
	}

	w.Header().Set("Location", fmt.Sprintf("/api/v1/trips/%s", trip.ID))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(ToResponse(trip))
}
