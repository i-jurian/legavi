// Package db wraps the application's Postgres connection pool and provides
// goose-based migrations.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // pgx as a database/sql driver, required for goose
	"github.com/pressly/goose/v3"
)

type DB struct {
	Pool *pgxpool.Pool
	dsn  string // kept for migrations; goose opens its own database/sql connection
}

func Connect(ctx context.Context, databaseURL string) (*DB, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &DB{Pool: pool, dsn: databaseURL}, nil
}

func (db *DB) Close() {
	if db != nil && db.Pool != nil {
		db.Pool.Close()
	}
}

func (db *DB) Ping(ctx context.Context) error {
	if db == nil || db.Pool == nil {
		return errors.New("db pool is nil")
	}
	return db.Pool.Ping(ctx)
}

// Migrate applies all pending goose migrations from migrationsFS (typically an
// embed.FS pointing at migration SQL files). Migrations run on a separate
// database/sql connection because goose requires that interface; the
// application's pgxpool is untouched.
func (db *DB) Migrate(migrationsFS fs.FS) error {
	if db == nil || db.dsn == "" {
		return errors.New("db not initialized")
	}
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose set dialect: %w", err)
	}
	sqlDB, err := sql.Open("pgx", db.dsn)
	if err != nil {
		return fmt.Errorf("open sql db for migrations: %w", err)
	}
	defer sqlDB.Close()
	if err := goose.Up(sqlDB, "."); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
