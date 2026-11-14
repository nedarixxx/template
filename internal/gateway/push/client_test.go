package push_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nedarixxx/template/internal/gateway/push"
)

func TestClient_RequestPosition(t *testing.T) {
	driverID := uuid.New()
	tripID := uuid.New()

	t.Run("success accepted 202", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/api/v1/push" {
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			w.WriteHeader(http.StatusAccepted)
		}))
		defer server.Close()

		client := push.NewClient(server.URL, 1*time.Second)
		if err := client.RequestPosition(context.Background(), driverID, tripID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("client error 400", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		client := push.NewClient(server.URL, 1*time.Second)
		err := client.RequestPosition(context.Background(), driverID, tripID)
		if err == nil {
			t.Fatalf("expected error for 400 status")
		}
	})

	t.Run("server error 503", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		client := push.NewClient(server.URL, 1*time.Second)
		err := client.RequestPosition(context.Background(), driverID, tripID)
		if err == nil {
			t.Fatalf("expected error for 503 status")
		}
	})

	t.Run("transport timeout", func(t *testing.T) {
		done := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-done
		}))
		defer func() {
			close(done)
			server.Close()
		}()

		client := push.NewClient(server.URL, 5*time.Millisecond)
		err := client.RequestPosition(context.Background(), driverID, tripID)
		if err == nil {
			t.Fatalf("expected transport error on timeout")
		}
	})
}
