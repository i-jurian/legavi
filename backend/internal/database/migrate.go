package database

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"

	_ "github.com/jackc/pgx/v5/stdlib" // pgx as a database/sql driver, required for goose
	"github.com/pressly/goose/v3"
)

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
	defer func() { _ = sqlDB.Close() }()
	if err := goose.Up(sqlDB, "."); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
