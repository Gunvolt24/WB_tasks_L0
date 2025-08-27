package validate

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateFile_JSON_Auto_OK(t *testing.T) {
	ctx := context.Background()
	validator := NewOrderValidator()

	dir := t.TempDir()
	path := filepath.Join(dir, "one.json")
	if err := os.WriteFile(path, []byte(minimalValidOrderJSON("uid-1", "txn-1", "u@e.com")), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var out bytes.Buffer
	summary, err := ValidateFile(ctx, validator, path, FormatAuto, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != "1 valid / 0 invalid" {
		t.Fatalf("unexpected summary: %s", summary)
	}
	if strings.TrimSpace(out.String()) == "" {
		t.Fatalf("expected non-empty output")
	}
}

func TestValidateFile_JSONL_Auto_Mixed(t *testing.T) {
	ctx := context.Background()
	validator := NewOrderValidator()

	dir := t.TempDir()
	path := filepath.Join(dir, "list.jsonl")
	content := oneLineJSON(minimalValidOrderJSON("uid-1", "txn-1", "u1@e.com")) + "\n" +
		oneLineJSON(minimalValidOrderJSON("uid-2", "txn-2", "")) + "\n" + // invalid email
		oneLineJSON(minimalValidOrderJSON("uid-3", "txn-3", "u3@e.com")) + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var out bytes.Buffer
	summary, err := ValidateFile(ctx, validator, path, FormatAuto, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != "2 valid / 1 invalid" {
		t.Fatalf("unexpected summary: %s", summary)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 output lines, got %d", len(lines))
	}
}

func TestValidateFile_JSON_Invalid(t *testing.T) {
	ctx := context.Background()
	validator := NewOrderValidator()

	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	// неизвестное поле
	raw := `{"unknown":1}` + minimalValidOrderJSONFields("uid-x", "txn-x", "u@e.com")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var out bytes.Buffer
	summary, err := ValidateFile(ctx, validator, path, FormatJSON, &out)
	if err == nil {
		t.Fatalf("expected error for invalid json")
	}
	if summary != "0 valid / 1 invalid" {
		t.Fatalf("unexpected summary: %s", summary)
	}
	if out.String() != "" {
		t.Fatalf("output must be empty for invalid single JSON")
	}
}

func TestValidateFile_ExplicitFormat_IgnoresExt(t *testing.T) {
	ctx := context.Background()
	validator := NewOrderValidator()

	dir := t.TempDir()
	path := filepath.Join(dir, "data.txt")
	content := oneLineJSON(minimalValidOrderJSON("uid-1", "txn-1", "u1@e.com")) + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var out bytes.Buffer
	summary, err := ValidateFile(ctx, validator, path, FormatJSONL, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != "1 valid / 0 invalid" {
		t.Fatalf("unexpected summary: %s", summary)
	}
}

func TestValidateFile_OpenError(t *testing.T) {
	ctx := context.Background()
	validator := NewOrderValidator()

	var out bytes.Buffer
	_, err := ValidateFile(ctx, validator, "no-such-file.json", FormatAuto, &out)
	if err == nil {
		t.Fatalf("expected open error")
	}
}

func TestValidateFile_UnsupportedFormat(t *testing.T) {
	ctx := context.Background()
	validator := NewOrderValidator()

	dir := t.TempDir()
	path := filepath.Join(dir, "one.json")
	_ = os.WriteFile(path, []byte(minimalValidOrderJSON("uid-1", "txn-1", "u@e.com")), 0o600)

	var out bytes.Buffer
	_, err := ValidateFile(ctx, validator, path, InputFormat("yaml"), &out)
	if err == nil || !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("expected unsupported format error, got: %v", err)
	}
}

// ---- функции для тестирования ----

func oneLineJSON(s string) string {
	var b bytes.Buffer
	_ = json.Compact(&b, []byte(s))
	return b.String()
}
