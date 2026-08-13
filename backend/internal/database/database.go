package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"time"

	_ "github.com/lib/pq"

	"github.com/jPurin-gg/myfitlog-backend/internal/config"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Open(ctx context.Context, cfg config.DBConfig, logger *slog.Logger) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	var lastErr error
	for attempt := 1; attempt <= 10; attempt++ {
		if err := db.PingContext(ctx); err == nil {
			return db, nil
		} else {
			lastErr = err
			logger.Warn("database connection failed", "attempt", attempt, "max_attempts", 10)
		}
		select {
		case <-ctx.Done():
			db.Close()
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	db.Close()
	return nil, fmt.Errorf("connect to database: %w", lastErr)
}

func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}
	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		migrationSQL, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		if err := applyMigration(ctx, db, entry.Name(), string(migrationSQL)); err != nil {
			return err
		}
	}
	return nil
}

func applyMigration(ctx context.Context, db *sql.DB, version, migrationSQL string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", version, err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(88420510732026)`); err != nil {
		return fmt.Errorf("lock migration %s: %w", version, err)
	}
	var applied bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&applied); err != nil {
		return fmt.Errorf("check migration %s: %w", version, err)
	}
	if applied {
		return tx.Commit()
	}
	if _, err := tx.ExecContext(ctx, migrationSQL); err != nil {
		return fmt.Errorf("apply migration %s: %w", version, err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
		return fmt.Errorf("record migration %s: %w", version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", version, err)
	}
	return nil
}
