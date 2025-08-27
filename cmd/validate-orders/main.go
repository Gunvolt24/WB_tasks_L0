package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/Gunvolt24/wb_l0/pkg/validate"
)

// CLI-приложение для валидации заказов.
func main() {
	inputPath := flag.String("in", "", "path to input (.json or .jsonl). If empty, reads from stdin.")
	formatStr := flag.String("format", "auto", "input format: auto|json|jsonl")
	flag.Parse()

	ctx := context.Background()
	orderValidator := validate.NewOrderValidator()

	format := validate.InputFormat(*formatStr)

	// stdin вариант: считаем, что jsonl
	if *inputPath == "" {
		if format == validate.FormatAuto {
			format = validate.FormatJSONL
		}
		summary, err := validate.ValidateFile(ctx, orderValidator, "/dev/stdin", format, os.Stdout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "validation: %v (%s)\n", err, summary)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "validation ok (%s)\n", summary)
		return
	}

	summary, err := validate.ValidateFile(ctx, orderValidator, *inputPath, format, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "validation: %v (%s)\n", err, summary)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "validation ok (%s)\n", summary)
}
