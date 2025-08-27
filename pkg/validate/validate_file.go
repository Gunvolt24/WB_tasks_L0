package validate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Gunvolt24/wb_l0/internal/ports"
)

// InputFormat допустимые значения.
type InputFormat string

const (
	FormatAuto  InputFormat = "auto"
	FormatJSON  InputFormat = "json"
	FormatJSONL InputFormat = "jsonl"
)

// ValidateFile — валидирует файл как JSON или JSONL и пишет валидный вывод в writer.
func ValidateFile(ctx context.Context, validator ports.OrderValidator, filePath string, format InputFormat, ow io.Writer) (string, error) {
	resSummary := ""

	// auto по расширению
	if format == FormatAuto {
		switch strings.ToLower(filepath.Ext(filePath)) {
		case ".jsonl":
			format = FormatJSONL
		case ".json":
			format = FormatJSON
		default:
			// по умолчанию считаем JSON
			format = FormatJSON
		}
	}

	file, err := os.Open(filePath)
	if err != nil {
		return resSummary, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	switch format {
	case FormatJSON:
		raw, err := io.ReadAll(file)
		if err != nil {
			return resSummary, fmt.Errorf("read file: %w", err)
		}
		order, err := ValidateOrderFromJSON(ctx, validator, raw)
		if err != nil {
			return "0 valid / 1 invalid", err
		}
		canonical, _ := json.Marshal(order)
		if _, err := ow.Write(canonical); err != nil {
			return resSummary, fmt.Errorf("write json: %w", err)
		}
		if _, err := ow.Write([]byte("\n")); err != nil {
			return resSummary, fmt.Errorf("write newline: %w", err)
		}
		return "1 valid / 0 invalid", nil

	case FormatJSONL:
		result, err := ValidateJSONLStream(ctx, validator, file, ow)
		if err != nil {
			return resSummary, err
		}
		return fmt.Sprintf("%d valid / %d invalid", result.ValidLinesCount, result.InvalidLinesCount), nil

	default:
		return resSummary, fmt.Errorf("unsupported format: %s", format)
	}
}
