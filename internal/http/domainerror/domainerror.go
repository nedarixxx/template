package domainerror

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/nedarixxx/template/api"
	domaintrip "github.com/nedarixxx/template/internal/domain/trip"
)

var ErrInvalidRequest = errors.New("invalid request")

func Problem(err error) (int, api.Problem) {
	var (
		status int
		title  string
		code   string
		detail string
	)

	var validationErr domaintrip.ValidationError
	switch {
	case errors.As(err, &validationErr):
		status = http.StatusBadRequest
		title = "Invalid Request"
		code = "invalid_request"
		detail = validationErr.Error()

	case errors.Is(err, ErrInvalidRequest):
		status = http.StatusBadRequest
		title = "Invalid Request"
		code = "invalid_request"
		detail = "Request body is invalid JSON"

	case errors.Is(err, domaintrip.ErrTripNotFound):
		status = http.StatusNotFound
		title = "Not Found"
		code = "trip_not_found"
		detail = "Trip was not found"

	case errors.Is(err, domaintrip.ErrDriverBusy):
		status = http.StatusConflict
		title = "Driver busy"
		code = "driver_busy"
		detail = "Driver already has an active trip"

	case errors.Is(err, domaintrip.ErrTripCompleted):
		status = http.StatusConflict
		title = "Trip completed"
		code = "trip_completed"
		detail = "Operation is not allowed for a completed trip"

	default:
		status = http.StatusInternalServerError
		title = "Internal Server Error"
		code = "internal_error"
		detail = "Internal server error"
	}

	problemType := fmt.Sprintf("https://tripgo.example/problems/%s", code)

	return status, api.Problem{
		Type:   problemType,
		Title:  title,
		Status: int32(status),
		Detail: &detail,
		Code:   code,
	}
}

func Write(w http.ResponseWriter, r *http.Request, err error) {
	status, prob := Problem(err)
	if r != nil {
		path := r.URL.Path
		prob.Instance = &path
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(prob)
}