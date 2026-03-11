package db

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jpfortier/gym-app/internal/env"
)

func New(ctx context.Context) (*sql.DB, error) {
	connStr := env.DatabaseURL()
	if connStr == "" {
		return nil, fmt.Errorf("database URL not configured (GYM_DATABASE_URL locally, DATABASE_URL on Fly)")
	}

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return db, nil
}
