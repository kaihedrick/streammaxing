package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

var Pool *pgxpool.Pool

// Connect establishes a connection pool to the database using the given URL.
func Connect(databaseURL string) error {
	if databaseURL == "" {
		return fmt.Errorf("database URL is empty")
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Configure connection pool for Lambda
	config.MaxConns = 10
	config.MinConns = 2

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(context.Background()); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	Pool = pool
	return nil
}

// Close closes the database connection pool
func Close() {
	if Pool != nil {
		Pool.Close()
	}
}
