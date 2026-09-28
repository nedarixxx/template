.PHONY: generate migrate build run

# Чтение переменных из .env, если файл существует
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

DATABASE_URL ?= postgres://tripgo:tripgo@localhost:21032/tripgo?sslmode=disable
HTTP_ADDR ?= :8080

generate:
	go tool oapi-codegen -generate types,chi-server -package api -o api/api.gen.go contracts/openapi/trip-service.openapi.yaml

test:
	go test -v -race ./...

migrate:
	go tool goose -dir ./migrations postgres "$(DATABASE_URL)" up

build:
	CGO_ENABLED=0 go build -o bin/trip-service ./cmd/trip-service

run: build
	HTTP_ADDR=$(HTTP_ADDR) DATABASE_URL="$(DATABASE_URL)" ./bin/trip-service