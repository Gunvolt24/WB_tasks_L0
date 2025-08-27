# syntax=docker/dockerfile:1

### build-stage
FROM golang:1.23-alpine AS builder
WORKDIR /src

# ускоряем кеширование зависимостей
COPY go.mod go.sum ./
RUN go mod download

# копируем исходный код
COPY . .
# сервер
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags=otel -o /out/server ./cmd/server
# CLI-валидатор (печатает валидные JSON/JSONL в stdout)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/validate-orders ./cmd/validate-orders

### runtime-stage
FROM alpine:3.20

RUN adduser -D -g '' appuser
WORKDIR /app

COPY --from=builder /out/server /app/server
COPY --from=builder /out/validate-orders /app/validate-orders
COPY web /app/web

EXPOSE 8081
USER appuser
ENV GIN_MODE=release
CMD ["/app/server"]
