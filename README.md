# WB tasks L0 (Демонстрационный сервис Orders)

Демонстрационный сервис на Go: читает заказы из Kafka, валидирует, сохраняет в PostgreSQL, кеширует в памяти и отдаёт ответ через HTTP API + простая веб-страница.

## Пример страницы сервиса:

<img width="1575" height="1318" alt="Снимок экрана 2025-08-27 225243" src="https://github.com/user-attachments/assets/b2bfa729-a789-449a-bd05-b1969d13b482" />

## Видео:
https://drive.google.com/file/d/1Ma8FEB2OuzWb8JFPC5EFdrA9CGX3nsDZ/view?usp=sharing

## Архитектура и стек

Docker Compose: `app` (Go/gin), `postgres`, `migrate` (goose), `kafka`+`zookeeper`, `kafka-ui`, `prometheus`, `grafana`, `jaeger`.

Go, gin, pgx, Kafka, goose, Prometheus, Grafana, Jaeger, Taskfile.

## Структура проекта
```text
.
├─ cmd/
│  ├─ server/                # main-сервис (HTTP + Kafka consumer)
│  │  └─ main.go
│  └─ validate-orders/       # CLI-предвалидатор JSON/JSONL перед отправкой в Kafka
│     └─ main.go
├─ internal/
│  ├─ app/                   # Bootstrap/DI, запуск HTTP и Kafka
│  ├─ cache/memory/          # LRU-кэш с TTL (+ тесты)
│  ├─ domain/                # Доменные модели
│  ├─ kafka/                 # Консьюмер Kafka (+ интеграционные тесты)
│  ├─ ports/                 # Интерфейсы (Logger, Repo, Cache, Validator, Consumer)
│  ├─ repo/postgres/         # Репозиторий PG, пул соединений (+ интеграционные тесты)
│  ├─ transport/http/        # Роутер/хендлеры, бенчмарки и интеграционные тесты
│  ├─ usecase/               # Бизнес-логика (OrderService)
│  └─ testutil/              # Вспомогательные файлы для тестов 
├─ pkg/
│  ├─ ctxmeta/               # Трассировка/метаданные запроса
│  ├─ httpx/                 # Хелперы для HTTP (request id, логгер и т.п.)
│  ├─ logger/                # Обёртка над zap
│  ├─ metrics/               # Prometheus-метрики и регистрация
│  ├─ telemetry/             # OTEL-трейсинг (Jaeger)
│  └─ validate/              # Доменный валидатор + утилиты для JSON/JSONL (producer-side)
├─ migrations/               # Goose-миграции (schema + индексы)
├─ config/                   # Конфигурация, парсинг .env
├─ docker/prometheus/        # prometheus.yml
├─ scripts/                  # Скрипты (отправка файлов в Kafka и т.п.)
├─ web/                      # Простой фронт (index.html)
├─ Taskfile.yml              # Удобные команды (task …)
├─ Dockerfile                # Сборка образа сервиса (server + web)
├─ docker-compose.yml        # Postgres + Kafka + app + Prometheus + Grafana + Jaeger
├─ .env.compose(.example)    # Переменные окружения для docker compose
```

## Быстрый старт

Требуется: **Docker** + **Docker Compose**. 

```bash
# 1) поднять всё
docker compose up -d --build

# 2) проверить сервис
curl http://localhost:8081/ping          # -> pong
open  http://localhost:8081/             # простая страница поиска заказа

# 3) отправить тестовые заказы в Kafka (из JSONL файла)
task kafka:send-file FILE=scripts/kafka-model.jsonl      # отправит scripts/kafka-model.jsonl  в топик orders
```

## Эндпоинты:

- `GET /order/:id` — отдать заказ в JSON
- `GET /customer/:id/orders?limit&offset` — список заказов
- `GET /metrics` — Prometheus метрики
- `GET /ping` — health

## UI/панели:

- **Orders (веб-страница):** http://localhost:8081/
- **Kafka UI:** http://localhost:8090
- **Prometheus:** http://localhost:9090
- **Grafana:** http://localhost:3000 (admin/admin)
- **Jaeger:** http://localhost:16686

## Конфигурация

Основные параметры задаются через `.env.compose` и `config.Config`:

- Postgres DSN, пул соединений
- Kafka brokers / group / topic
- HTTP таймауты и режим Gin
- Кэш: `capacity`, `ttl`, `warmUpN`
- Трейсинг OTEL (вкл/выкл, endpoint)

## Модель данных и миграции

Миграции в `./migrations` применяются контейнером **orders-migrate** при старте.

## Ключевые ограничения:

- `orders.order_uid` - PRIMARY KEY
- `payments.transaction` - PRIMARY KEY
- `payments.order_uid` - UNIQUE (1:1 к заказу)

Пример JSON см. `scripts/kafka-model.jsonl`.

## Кэширование

- **Объектный LRU-кэш с TTL (in-memory).**
- **Прогрев кэша при старте:** берём последние N заказов из БД.
- **Порядок обработки запроса:** кэш → БД (+запись в кэш).

## Валидация

**Consumer-side (в приложении):**
- **Строгий JSON-парсинг:** `DisallowUnknownFields()` + проверка на пустые поля;
- **Доменная валидация** всех обязательных полей (заказ/платёж/доставка/товары).

**Producer-side:**
- Каждая запись **валидируется** теми же правилами, до публикации.

## Работа с Kafka

Отправка файла JSONL в топик orders:
```bash
# Отправка тестового файла с одним сообщением
task kafka:send-file FILE=scripts/kafka-model.jsonl

# Отправка тестового файла с несколькими сообщениями
task kafka:send-file FILE=scripts/orders_user777.jsonl
```

Проверить доставку сообщений можно через Kafka UI → local → orders.

## Мониторинг (Prometheus/Grafana/Jaeger)

**Метрики (основные):**

- `cache_operations_total` - все счётчики операций ({op="hit|miss|evicted|expired"}).
- `cache_size` — текущее число элементов в кэше.

**Полезные PromQL-запросы:**
- Hit ratio (за 5 минут):
```bash
sum(rate(cache_operations_total{op="hit"}[5m]))
/
sum(rate(cache_operations_total[5m]))
```

- Частота истекших элементов кэша:
```bash
rate(cache_operations_total{op="expired"}[15m])
```

- Miss ratio (за 5 минут):
```bash
sum(rate(cache_operations_total{op="miss"}[5m]))
/
sum(rate(cache_operations_total{op=~"hit|miss"}[5m]))
```

- Скорость вытеснений (evicted):
```bash
rate(cache_operations_total{op="evicted"}[5m])
```

### Метрики Kafka (по топикам)

- `kafka_messages_consumed_total{topic="orders"}`
- `kafka_messages_processed_total{topic="orders"}`
- `kafka_messages_failed_total{topic="orders"}`

**Полезные PromQL-запросы:**
- Пропускная способность потребления:
```scss
rate(kafka_messages_consumed_total{topic="orders"}[5m])
```

- Успешность обработки:
```scss
sum(rate(kafka_messages_processed_total[5m])) by (topic)
/
(
  sum(rate(kafka_messages_processed_total[5m])) by (topic)
  + sum(rate(kafka_messages_failed_total[5m])) by (topic)
)
```

- Ошибки обработки:

```scss
rate(kafka_messages_failed_total{topic="orders"}[5m])
```

## Трейсинг 

Трейсы HTTP доступны в Jaeger; включение трейсинга находится в `.env.compose` и включается путем переключения флага в `ORDER_TRACING_OTEL_ENABLED`.

## Тесты и бенчмарки

```go
// unit-тесты
go test ./...

// интеграционные (помечены тегом)
go test -tags=integration ./...

// HTTP-бенчи
go test -run ^$ -bench . -benchmem -benchtime=10s ./internal/transport/http
```



