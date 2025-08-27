package validate

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Gunvolt24/wb_l0/internal/ports"
)

// JSONLResult — статистика валидации потока JSONL.
type JSONLResult struct {
	ValidLinesCount   int
	InvalidLinesCount int
}

// ValidateJSONLStream — читает JSONL из reader’а, валидирует каждую строку, валидные пишет в writer.
// Печатает КАНОНИЧЕСКИЙ JSON одной строкой на каждую валидную запись.
// Пустые строки пропускаются.
func ValidateJSONLStream(ctx context.Context, validator ports.OrderValidator, ir io.Reader, ow io.Writer) (JSONLResult, error) {
	var res JSONLResult

	scanner := bufio.NewScanner(ir)
	// запас на большие строки
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		lineBytes := scanner.Bytes()
		if len(strings.TrimSpace(string(lineBytes))) == 0 {
			continue
		}

		order, err := ValidateOrderFromJSON(ctx, validator, lineBytes)
		if err != nil {
			res.InvalidLinesCount++
			// не возвращаем ошибку — просто пропускаем невалидную строку
			continue
		}

		marshal, _ := json.Marshal(order) // маршалим в компактный JSON
		if _, err := ow.Write(marshal); err != nil {
			return res, fmt.Errorf("write valid line: %w", err)
		}
		if _, err := ow.Write([]byte("\n")); err != nil {
			return res, fmt.Errorf("write newline: %w", err)
		}
		res.ValidLinesCount++
	}
	if err := scanner.Err(); err != nil {
		return res, fmt.Errorf("scan: %w", err)
	}
	return res, nil
}
