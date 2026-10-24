package get

import (
	"encoding/json"
	"net/http"

	"github.com/nedarixxx/template/api"
	"github.com/nedarixxx/template/internal/http/domainerror"
)

type Handler struct {
	useCase useCase
}

func New(uc useCase) *Handler {
	return &Handler{useCase: uc}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	trip, err := h.useCase.Execute(r.Context(), tripID)
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ToResponse(trip))
}