# syntax=docker/dockerfile:1

### build-stage
FROM golang:1.23-alpine AS builder
WORKDIR /src

# ускоряем кеширование зависимостей
COPY go.mod go.sum ./
RUN go mod download

# собираем приложение
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server ./cmd/server

### runtime-stage
FROM alpine:3.20
# необязательно, но приятнее
RUN adduser -D -g '' appuser
WORKDIR /app

COPY --from=builder /out/server /app/server
COPY web /app/web

EXPOSE 8081
USER appuser
ENV GIN_MODE=release
CMD ["/app/server"]
