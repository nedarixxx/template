package container

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nedarixxx/template/internal/config"
	httpTransport "github.com/nedarixxx/template/internal/http"
	createHttp "github.com/nedarixxx/template/internal/http/trip/create"
	finishHttp "github.com/nedarixxx/template/internal/http/trip/finish"
	getHttp "github.com/nedarixxx/template/internal/http/trip/get"
	"github.com/nedarixxx/template/internal/infra/postgres"
	"github.com/nedarixxx/template/internal/infra/postgres/transaction"
	"github.com/nedarixxx/template/internal/infra/telemetry"
	tripRepo "github.com/nedarixxx/template/internal/repository/trip"
	createUC "github.com/nedarixxx/template/internal/usecase/trip/create"
	finishUC "github.com/nedarixxx/template/internal/usecase/trip/finish"
	getUC "github.com/nedarixxx/template/internal/usecase/trip/get"
)

type Container struct {
	DB     *postgres.Client
	Server *httpTransport.Server
}

func New(ctx context.Context, cfg *config.Config, tel *telemetry.Telemetry, _ *slog.Logger) (*Container, error) {
	// 1 Инфраструктура БД
	db, err := postgres.New(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("init postgres: %w", err)
	}

	// 2 Репозиторий и транзакции
	txManager := transaction.NewManager(db.Pool)
	repo := tripRepo.New(db.Pool)

	// 3 Сценарии (Use Cases)
	ucCreate := createUC.New(repo, txManager)
	ucGet := getUC.New(repo)
	ucFinish := finishUC.New(repo, txManager)

	// 4 HTTP-ручки с бизнес-метриками
	var createOpts []createHttp.Option
	if tel != nil && tel.TripsCreatedCounter != nil {
		createOpts = append(createOpts, createHttp.WithCounter(tel.TripsCreatedCounter))
	}
	hCreate := createHttp.New(ucCreate, createOpts...)

	hGet := getHttp.New(ucGet)

	var finishOpts []finishHttp.Option
	if tel != nil && tel.TripsCompletedCounter != nil {
		finishOpts = append(finishOpts, finishHttp.WithCounter(tel.TripsCompletedCounter))
	}
	hFinish := finishHttp.New(ucFinish, finishOpts...)

	// 5 Композиция сервера
	server := httpTransport.NewServer(hCreate, hGet, hFinish, db)

	return &Container{
		DB:     db,
		Server: server,
	}, nil
}

func (c *Container) Close() {
	if c.DB != nil {
		c.DB.Close()
	}
}