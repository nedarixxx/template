package create

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/metric"

	"github.com/nedarixxx/template/api"
	domaintrip "github.com/nedarixxx/template/internal/domain/trip"
	"github.com/nedarixxx/template/internal/http/domainerror"
	"github.com/nedarixxx/template/internal/model"
	usecasecreate "github.com/nedarixxx/template/internal/usecase/trip/position/create"
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

const msgRequired = "required"

type createPositionRequest struct {
	Coordinates *struct {
		Latitude  *float64 `json:"latitude"`
		Longitude *float64 `json:"longitude"`
	} `json:"coordinates"`
	RecordedAt *time.Time `json:"recorded_at"`
}

func validateCoordinates(p *struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
},
) error {
	if p == nil || p.Latitude == nil || p.Longitude == nil {
		return domaintrip.ValidationError{Field: "coordinates", Message: msgRequired}
	}
	if math.IsNaN(*p.Latitude) || math.IsInf(*p.Latitude, 0) || *p.Latitude < -90 || *p.Latitude > 90 {
		return domaintrip.ValidationError{Field: "coordinates.latitude", Message: "must be between -90 and 90"}
	}
	if math.IsNaN(*p.Longitude) || math.IsInf(*p.Longitude, 0) || *p.Longitude < -180 || *p.Longitude > 180 {
		return domaintrip.ValidationError{Field: "coordinates.longitude", Message: "must be between -180 and 180"}
	}
	return nil
}

func validatePositionRequest(req *createPositionRequest) error {
	if err := validateCoordinates(req.Coordinates); err != nil {
		return err
	}
	if req.RecordedAt == nil || req.RecordedAt.IsZero() {
		return domaintrip.ValidationError{Field: "recorded_at", Message: msgRequired}
	}
	if req.RecordedAt.After(time.Now().Add(1 * time.Minute)) {
		return domaintrip.ValidationError{Field: "recorded_at", Message: "cannot be in the future by more than 1 minute"}
	}
	return nil
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	if tripID == uuid.Nil {
		domainerror.Write(w, r, domaintrip.ValidationError{Field: "trip_id", Message: msgRequired})
		return
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req createPositionRequest
	if err := dec.Decode(&req); err != nil {
		domainerror.Write(w, r, domainerror.ErrInvalidRequest)
		return
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		domainerror.Write(w, r, domainerror.ErrInvalidRequest)
		return
	}

	if err := validatePositionRequest(&req); err != nil {
		domainerror.Write(w, r, err)
		return
	}

	cmd := usecasecreate.Command{
		TripID: tripID,
		Coordinates: model.Coordinates{
			Latitude:  *req.Coordinates.Latitude,
			Longitude: *req.Coordinates.Longitude,
		},
		RecordedAt: *req.RecordedAt,
	}

	pos, err := h.useCase.Execute(r.Context(), cmd)
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	if h.counter != nil {
		h.counter.Add(r.Context(), 1)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(ToResponse(pos))
}
