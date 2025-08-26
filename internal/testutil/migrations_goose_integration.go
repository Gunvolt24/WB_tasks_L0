//go:build integration

package testutil

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver name = "pgx"
	"github.com/pressly/goose/v3"
)

// ApplyMigrationsGoose жестко применяет миграции из <repo_root>/migrations
// (<repo_root> вычисляем как два уровня вверх от этого файла).
func ApplyMigrationsGoose(dsn string) error {
	// Этот файл: <repo>/internal/testutil/migrations_goose_integration.go
	// repoRoot = .. (internal) -> .. (repo root)
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	dir := filepath.Join(repoRoot, "migrations")

	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return fmt.Errorf("migrations dir not found: %q (рассчитан от %s)", dir, thisFile)
	}

	goose.SetLogger(log.New(os.Stdout, "", 0))
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose set dialect: %w", err)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	if err := goose.Up(db, dir); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
