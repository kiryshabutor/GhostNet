package feed

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository инкапсулирует выборку постов для ленты.
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// NextPost возвращает следующий пост для пользователя или false, если постов нет.
func (r *Repository) NextPost(ctx context.Context, userID int64) (postID int64, authorID int64, found bool, err error) {
	err = r.pool.QueryRow(ctx, `
		SELECT p.id, p.author_user_id
		FROM posts p
		LEFT JOIN post_views v ON v.post_id = p.id AND v.user_id = $1
		WHERE p.author_user_id <> $1
		  AND p.is_deleted = FALSE
		  AND v.id IS NULL
		ORDER BY p.created_at DESC
		LIMIT 1
	`, userID).Scan(&postID, &authorID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, 0, false, nil
		}
		return 0, 0, false, fmt.Errorf("select next post: %w", err)
	}
	return postID, authorID, true, nil
}
