# Stage 1: Build
FROM golang:1.27-alpine AS builder

WORKDIR /app

ENV GOTOOLCHAIN=auto

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o /bin/trip-service ./cmd/trip-service

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S appgroup && adduser -S appuser -G appgroup

USER appuser

COPY --from=builder /bin/trip-service /bin/trip-service

EXPOSE 8080

ENTRYPOINT ["/bin/trip-service"]
