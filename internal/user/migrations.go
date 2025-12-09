package user

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

const createUsersTable = `
CREATE TABLE IF NOT EXISTS users (
    id           BIGSERIAL PRIMARY KEY,
    telegram_id  BIGINT NOT NULL UNIQUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

const createSettingsTable = `
CREATE TABLE IF NOT EXISTS user_notification_settings (
    user_id             BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    notify_on_like      BOOLEAN NOT NULL DEFAULT TRUE,
    notify_on_dislike   BOOLEAN NOT NULL DEFAULT FALSE,
    notify_on_comment   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

// RunMigrations выполняет минимальный набор миграций для user-service.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	stmts := []string{
		createUsersTable,
		createSettingsTable,
	}

	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("apply migration: %w", err)
		}
	}

	return nil
}
