package post

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

const createPostsTable = `
CREATE TABLE IF NOT EXISTS posts (
    id              BIGSERIAL PRIMARY KEY,
    author_user_id  BIGINT NOT NULL REFERENCES users(id),
    text            TEXT   NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_deleted      BOOLEAN NOT NULL DEFAULT FALSE
);`

const createMediaTable = `
CREATE TABLE IF NOT EXISTS post_media (
    id                 BIGSERIAL PRIMARY KEY,
    post_id            BIGINT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    telegram_file_id   TEXT   NOT NULL,
    telegram_unique_id TEXT,
    media_type         VARCHAR(16) NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

// RunMigrations выполняет минимальный набор миграций для post-service.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	stmts := []string{
		createPostsTable,
		createMediaTable,
	}

	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("apply migration: %w", err)
		}
	}
	return nil
}
