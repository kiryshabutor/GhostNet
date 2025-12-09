package interaction

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrPostNotFound используется, когда пост отсутствует.
	ErrPostNotFound = errors.New("post not found")
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) GetPostAuthor(ctx context.Context, postID int64) (int64, error) {
	var authorID int64
	err := r.pool.QueryRow(ctx, `SELECT author_user_id FROM posts WHERE id = $1 AND is_deleted = FALSE`, postID).Scan(&authorID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrPostNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get post author: %w", err)
	}
	return authorID, nil
}

func (r *Repository) UpsertVote(ctx context.Context, userID, postID int64, voteType int16) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO post_votes (post_id, user_id, vote_type, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (post_id, user_id) DO UPDATE
		SET vote_type = EXCLUDED.vote_type,
		    updated_at = NOW()
	`, postID, userID, voteType)
	if err != nil {
		return fmt.Errorf("upsert vote: %w", err)
	}
	return nil
}

func (r *Repository) AddComment(ctx context.Context, userID, postID int64, text string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO post_comments (post_id, user_id, text)
		VALUES ($1, $2, $3)
		RETURNING id
	`, postID, userID, text).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert comment: %w", err)
	}
	return id, nil
}

func (r *Repository) MarkViewed(ctx context.Context, userID, postID int64) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO post_views (post_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, postID, userID)
	if err != nil {
		return false, fmt.Errorf("insert view: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *Repository) GetStats(ctx context.Context, postID int64) (likes, dislikes, comments, views int64, err error) {
	err = r.pool.QueryRow(ctx, `
		WITH votes AS (
			SELECT
				COUNT(*) FILTER (WHERE vote_type = 1)  AS likes,
				COUNT(*) FILTER (WHERE vote_type = -1) AS dislikes
			FROM post_votes
			WHERE post_id = $1
		),
		comments AS (
			SELECT COUNT(*) AS comments
			FROM post_comments
			WHERE post_id = $1 AND is_deleted = FALSE
		),
		views AS (
			SELECT COUNT(*) AS views
			FROM post_views
			WHERE post_id = $1
		)
		SELECT
			COALESCE(v.likes, 0),
			COALESCE(v.dislikes, 0),
			COALESCE(c.comments, 0),
			COALESCE(vw.views, 0)
		FROM votes v, comments c, views vw
	`, postID).Scan(&likes, &dislikes, &comments, &views)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("get stats: %w", err)
	}
	return
}

type CommentItem struct {
	ID        int64
	Text      string
	CreatedAt string
}

func (r *Repository) ListComments(ctx context.Context, postID int64, page, pageSize int32) ([]CommentItem, bool, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize
	limit := pageSize + 1

	rows, err := r.pool.Query(ctx, `
		SELECT id, text, created_at
		FROM post_comments
		WHERE post_id = $1 AND is_deleted = FALSE
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, postID, limit, offset)
	if err != nil {
		return nil, false, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()

	items := make([]CommentItem, 0, pageSize)
	for rows.Next() {
		var item CommentItem
		if err := rows.Scan(&item.ID, &item.Text, &item.CreatedAt); err != nil {
			return nil, false, fmt.Errorf("scan comment: %w", err)
		}
		items = append(items, item)
	}

	hasMore := false
	if int32(len(items)) > pageSize {
		hasMore = true
		items = items[:pageSize]
	}

	return items, hasMore, nil
}
