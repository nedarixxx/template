package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/nedarixxx/template/api"
	createTrip "github.com/nedarixxx/template/internal/http/trip/create"
	finishTrip "github.com/nedarixxx/template/internal/http/trip/finish"
	getTrip "github.com/nedarixxx/template/internal/http/trip/get"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type Server struct {
	createHandler *createTrip.Handler
	getHandler    *getTrip.Handler
	finishHandler *finishTrip.Handler
	pinger        Pinger
}

func NewServer(
	createHandler *createTrip.Handler,
	getHandler *getTrip.Handler,
	finishHandler *finishTrip.Handler,
	pinger Pinger,
) *Server {
	return &Server{
		createHandler: createHandler,
		getHandler:    getHandler,
		finishHandler: finishHandler,
		pinger:        pinger,
	}
}

func (s *Server) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	s.createHandler.Handle(w, r, params)
}

func (s *Server) GetTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	s.getHandler.Handle(w, r, tripID)
}

func (s *Server) FinishTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	s.finishHandler.Handle(w, r, tripID)
}

func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Ok})
}

func (s *Server) Ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.pinger != nil {
		if err := s.pinger.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Unavailable})
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Ok})
}

func (s *Server) ListTripPositions(w http.ResponseWriter, r *http.Request, _ api.TripId) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusNotImplemented)
}

func (s *Server) CreateTripPosition(w http.ResponseWriter, r *http.Request, _ api.TripId) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusNotImplemented)
}