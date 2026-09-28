package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nedarixxx/template/api"
	"github.com/nedarixxx/template/internal/config"
	"github.com/nedarixxx/template/internal/database"
	"github.com/nedarixxx/template/internal/handler"
	"github.com/nedarixxx/template/internal/repository"
	"github.com/nedarixxx/template/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1 Инициализация пула БД
	db, err := database.New(ctx, cfg)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()

	// 2 Сборка слоев
	repo := repository.NewTripRepository(db)
	tripService := service.NewTripService(repo, db)
	h := handler.New(tripService, db)

	// 3 Роутер
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Регистрация
	api.HandlerFromMux(h, r)

	// 4 HTTP сервер
	server := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      r,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	shutdownErr := make(chan error, 1)
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		log.Println("shutting down server...")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer shutdownCancel()

		shutdownErr <- server.Shutdown(shutdownCtx)
	}()

	log.Printf("server is listening on %s", cfg.HTTPAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("http server error: %v", err)
	}

	if err := <-shutdownErr; err != nil {
		log.Printf("error during shutdown: %v", err)
	}

	log.Println("server stopped successfully")
}