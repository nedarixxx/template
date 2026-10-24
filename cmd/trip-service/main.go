package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	chi "github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/nedarixxx/template/api"
	"github.com/nedarixxx/template/internal/config"
	"github.com/nedarixxx/template/internal/container"
	appmiddleware "github.com/nedarixxx/template/internal/http/middleware"
	"github.com/nedarixxx/template/internal/infra/telemetry"
)

func parseLogLevel(lvl string) slog.Level {
	switch strings.ToLower(lvl) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// 1. Инициализация структурированного логгера
	logLevel := parseLogLevel(cfg.LogLevel)
	logger := telemetry.NewLogger(os.Stdout, logLevel, "trip-service")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. Инициализация
	tel, shutdownTel, err := telemetry.Init(ctx, "trip-service")
	if err != nil {
		logger.Error("failed to init telemetry", slog.Any("error", err))
	} else {
		defer func() {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer shutdownCancel()
			_ = shutdownTel(shutdownCtx)
		}()
	}

	// 3. контейнер
	c, err := container.New(ctx, cfg, tel, logger)
	if err != nil {
		logger.Error("failed to init container", slog.Any("error", err))
		os.Exit(1)
	}
	defer c.Close()

	// 4. Роутер и цепочка middleware
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(appmiddleware.Recover(logger))
	r.Use(appmiddleware.Tracing("trip-service"))
	r.Use(appmiddleware.Logging(logger))
	if tel != nil {
		r.Use(appmiddleware.Metrics(tel))
	}

	api.HandlerFromMux(c.Server, r)

	// 5. HTTP сервер
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	shutdownErr := make(chan error, 1)
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		logger.Info("shutting down server...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer shutdownCancel()

		shutdownErr <- server.Shutdown(shutdownCtx)
	}()

	logger.Info("server is listening", slog.String("addr", cfg.HTTPAddr))
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("http server failed", slog.Any("error", err))
		os.Exit(1)
	}

	if err := <-shutdownErr; err != nil {
		logger.Error("error during shutdown", slog.Any("error", err))
	}

	logger.Info("server stopped successfully")
}