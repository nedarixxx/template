package container

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nedarixxx/template/internal/config"
	pushGateway "github.com/nedarixxx/template/internal/gateway/push"
	httpTransport "github.com/nedarixxx/template/internal/http"
	createHttp "github.com/nedarixxx/template/internal/http/trip/create"
	finishHttp "github.com/nedarixxx/template/internal/http/trip/finish"
	getHttp "github.com/nedarixxx/template/internal/http/trip/get"
	createPosHttp "github.com/nedarixxx/template/internal/http/trip/position/create"
	listPosHttp "github.com/nedarixxx/template/internal/http/trip/position/list"
	"github.com/nedarixxx/template/internal/infra/postgres"
	"github.com/nedarixxx/template/internal/infra/postgres/transaction"
	"github.com/nedarixxx/template/internal/infra/telemetry"
	tripRepo "github.com/nedarixxx/template/internal/repository/trip"
	createUC "github.com/nedarixxx/template/internal/usecase/trip/create"
	finishUC "github.com/nedarixxx/template/internal/usecase/trip/finish"
	getUC "github.com/nedarixxx/template/internal/usecase/trip/get"
	createPosUC "github.com/nedarixxx/template/internal/usecase/trip/position/create"
	listPosUC "github.com/nedarixxx/template/internal/usecase/trip/position/list"
	"github.com/nedarixxx/template/internal/worker/positions"
)

type Container struct {
	DB            *postgres.Client
	Server        *httpTransport.Server
	PositionsPool *positions.Pool
}

func New(ctx context.Context, cfg *config.Config, tel *telemetry.Telemetry, logger *slog.Logger) (*Container, error) {
	// 1 Инфраструктура БД
	db, err := postgres.New(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("init postgres: %w", err)
	}

	// 2 Репозиторий и транзакции
	txManager := transaction.NewManager(db.Pool)
	repo := tripRepo.New(db.Pool)

	// Фоновый пул и гейтвей
	pushClient := pushGateway.NewClient(cfg.PushHTTPURL, cfg.PushRequestTimeout)

	poolCfg := positions.PoolConfig{
		Workers:        cfg.PositionWorkers,
		QueueSize:      cfg.PositionQueueSize,
		RequestTimeout: cfg.PushRequestTimeout,
		CheckInterval:  cfg.PositionCheckInterval,
		StaleAfter:     cfg.PositionStaleAfter,
		BatchSize:      cfg.PositionBatchSize,
	}

	var poolMetrics positions.PoolMetrics
	if tel != nil {
		poolMetrics = positions.PoolMetrics{
			StaleChecks:              tel.StaleChecksCounter,
			StaleTripsRowsRead:       tel.StaleTripsRowsReadCounter,
			PositionRequests:         tel.PositionRequestsCounter,
			PositionRequestsDuration: tel.PositionRequestsDuration,
			QueueSize:                tel.PositionRequestsQueueSize,
			WorkersActive:            tel.PositionRequestsWorkersActive,
		}
	}

	positionsPool := positions.NewPool(repo, pushClient, poolCfg, poolMetrics, logger, nil)

	// 3 Сценарии (Use Cases)
	ucCreate := createUC.New(repo, txManager)
	ucGet := getUC.New(repo)
	ucFinish := finishUC.New(repo, txManager)
	ucCreatePos := createPosUC.New(repo, txManager, positionsPool)
	ucListPos := listPosUC.New(repo)

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

	var createPosOpts []createPosHttp.Option
	if tel != nil && tel.PositionsSavedCounter != nil {
		createPosOpts = append(createPosOpts, createPosHttp.WithCounter(tel.PositionsSavedCounter))
	}
	hCreatePos := createPosHttp.New(ucCreatePos, createPosOpts...)
	hListPos := listPosHttp.New(ucListPos)

	// 5 Композиция сервера
	server := httpTransport.NewServer(hCreate, hGet, hFinish, hCreatePos, hListPos, db)

	return &Container{
		DB:            db,
		Server:        server,
		PositionsPool: positionsPool,
	}, nil
}

func (c *Container) Close() {
	if c.DB != nil {
		c.DB.Close()
	}
}
