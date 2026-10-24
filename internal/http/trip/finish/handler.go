package finish

import (
	"encoding/json"
	"net/http"

	"go.opentelemetry.io/otel/metric"

	"github.com/nedarixxx/template/api"
	"github.com/nedarixxx/template/internal/http/domainerror"
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

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	trip, err := h.useCase.Execute(r.Context(), tripID)
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	if h.counter != nil {
		h.counter.Add(r.Context(), 1)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ToResponse(trip))
}