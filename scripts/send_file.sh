#!/usr/bin/env bash
set -euo pipefail

TOPIC="${1:-orders}"
FILE_PATH="${2:-scripts/orders_user777.jsonl}"

EXT="${FILE_PATH##*.}"
FORMAT="auto"
if [[ "$EXT" == "json" ]]; then
  FORMAT="json"
elif [[ "$EXT" == "jsonl" ]]; then
  FORMAT="jsonl"
fi

echo "→ Валидация ($FORMAT) и отправка в Kafka (topic=$TOPIC)..."

# валидируем и сразу шлём в Kafka как по строке
go run ./cmd/validate-orders -in "$FILE_PATH" -format "$FORMAT" \
  | docker exec -i orders-kafka kafka-console-producer \
      --bootstrap-server kafka:9092 \
      --topic "$TOPIC"

echo "✓ Готово."
