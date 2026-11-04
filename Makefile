.PHONY: generate test test-all lint migrate build run up down

ifneq (,$(wildcard ./.env))
    include .env
    export
endif

DATABASE_URL ?= postgres://tripgo:tripgo@localhost:22032/tripgo?sslmode=disable
HTTP_ADDR ?= :8080

generate:
	go tool oapi-codegen -generate types,chi-server -package api -o api/api.gen.go contracts/openapi/trip-service.openapi.yaml

test:
	go test -v -race ./internal/domain/... ./internal/usecase/... ./internal/http/...

test-all:
	go test -v -race -tags=integration ./...

lint:
	golangci-lint run ./...

migrate:
	go tool goose -dir ./migrations postgres "$(DATABASE_URL)" up

build:
	CGO_ENABLED=0 go build -o bin/trip-service ./cmd/trip-service

run: build
	HTTP_ADDR=$(HTTP_ADDR) DATABASE_URL="$(DATABASE_URL)" ./bin/trip-service

up:
	docker compose up -d --build

down:
	docker compose down
