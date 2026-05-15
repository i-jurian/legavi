// Package database wraps the application's Postgres connection pool and migration runner.
package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
	dsn  string // kept for migrations; goose opens its own database/sql connection
}

func Connect(ctx context.Context, databaseURL string) (*DB, error) {
	p, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := p.Ping(pingCtx); err != nil {
		p.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &DB{Pool: p, dsn: databaseURL}, nil
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
