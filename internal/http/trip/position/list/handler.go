package list

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/nedarixxx/template/api"
	domaintrip "github.com/nedarixxx/template/internal/domain/trip"
	"github.com/nedarixxx/template/internal/http/domainerror"
)

type Handler struct {
	useCase useCase
}

func New(uc useCase) *Handler {
	return &Handler{useCase: uc}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	if tripID == uuid.Nil {
		domainerror.Write(w, r, domaintrip.ValidationError{Field: "trip_id", Message: "required"})
		return
	}

	positions, err := h.useCase.Execute(r.Context(), tripID)
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ToResponse(positions))
}
