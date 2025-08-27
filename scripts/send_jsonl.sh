#!/usr/bin/env bash
set -euo pipefail

TOPIC="${1:-orders}"
FILE="${2:-scripts/kafka-model.jsonl}"

echo "→ Проверяю/создаю топик '$TOPIC'..."
docker exec orders-kafka bash -lc \
  "kafka-topics --bootstrap-server kafka:9092 \
   --create --if-not-exists --topic '$TOPIC' --partitions 1 --replication-factor 1"

echo "→ Отправляю JSONL из '$FILE' в '$TOPIC'..."
# Удаляем CR, чтобы не ломать producer, и шлём в контейнер через stdin
cat "$FILE" | tr -d '\r' | docker exec -i orders-kafka bash -lc \
  "kafka-console-producer --bootstrap-server kafka:9092 --topic '$TOPIC'"

echo "✓ Готово."
