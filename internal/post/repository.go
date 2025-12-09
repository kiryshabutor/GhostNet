package post

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrPostNotFound возвращается, когда пост не найден.
	ErrPostNotFound = errors.New("post not found")
)

// Post описывает сохранённый пост.
type Post struct {
	ID               int64
	AuthorUserID     int64
	Text             string
	MediaType        string
	TelegramFileID   string
	TelegramUniqueID string
}

// PostPreview используется в списках автора.
type PostPreview struct {
	PostID      int64
	TextPreview string
	Likes       int64
	Dislikes    int64
	Comments    int64
	Views       int64
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) CreatePost(ctx context.Context, authorID int64, text string, mediaType string, fileID string, uniqueID string) (int64, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var postID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO posts (author_user_id, text)
		VALUES ($1, $2)
		RETURNING id
	`, authorID, text).Scan(&postID); err != nil {
		return 0, fmt.Errorf("insert post: %w", err)
	}

	if mediaType != "" && fileID != "" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO post_media (post_id, telegram_file_id, telegram_unique_id, media_type)
			VALUES ($1, $2, $3, $4)
		`, postID, fileID, uniqueID, mediaType); err != nil {
			return 0, fmt.Errorf("insert media: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}

	return postID, nil
}

func (r *Repository) GetPost(ctx context.Context, postID int64) (Post, error) {
	var p Post
	err := r.pool.QueryRow(ctx, `
		SELECT p.id,
		       p.author_user_id,
		       p.text,
		       COALESCE(m.media_type, ''),
		       COALESCE(m.telegram_file_id, ''),
		       COALESCE(m.telegram_unique_id, '')
		FROM posts p
		LEFT JOIN post_media m ON m.post_id = p.id
		WHERE p.id = $1 AND p.is_deleted = FALSE
	`, postID).Scan(&p.ID, &p.AuthorUserID, &p.Text, &p.MediaType, &p.TelegramFileID, &p.TelegramUniqueID)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrPostNotFound
	}
	if err != nil {
		return p, fmt.Errorf("get post: %w", err)
	}
	return p, nil
}

func (r *Repository) ListUserPosts(ctx context.Context, userID int64, page, pageSize int32) ([]PostPreview, bool, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize
	limit := pageSize + 1 // на один больше для определения has_more

	rows, err := r.pool.Query(ctx, `
		WITH votes AS (
			SELECT post_id,
				   COUNT(*) FILTER (WHERE vote_type = 1)  AS likes,
				   COUNT(*) FILTER (WHERE vote_type = -1) AS dislikes
			FROM post_votes
			GROUP BY post_id
		),
		comments AS (
			SELECT post_id, COUNT(*) AS comments
			FROM post_comments
			WHERE is_deleted = FALSE
			GROUP BY post_id
		),
		views AS (
			SELECT post_id, COUNT(*) AS views
			FROM post_views
			GROUP BY post_id
		)
		SELECT p.id,
		       LEFT(p.text, 64) AS preview,
		       COALESCE(v.likes, 0),
		       COALESCE(v.dislikes, 0),
		       COALESCE(c.comments, 0),
		       COALESCE(vw.views, 0)
		FROM posts p
		LEFT JOIN votes v ON v.post_id = p.id
		LEFT JOIN comments c ON c.post_id = p.id
		LEFT JOIN views vw ON vw.post_id = p.id
		WHERE p.author_user_id = $1 AND p.is_deleted = FALSE
		ORDER BY p.created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, false, fmt.Errorf("list posts: %w", err)
	}
	defer rows.Close()

	previews := make([]PostPreview, 0, pageSize)
	for rows.Next() {
		var item PostPreview
		if err := rows.Scan(&item.PostID, &item.TextPreview, &item.Likes, &item.Dislikes, &item.Comments, &item.Views); err != nil {
			return nil, false, fmt.Errorf("scan post: %w", err)
		}
		previews = append(previews, item)
	}

	hasMore := false
	if int32(len(previews)) > pageSize {
		hasMore = true
		previews = previews[:pageSize]
	}

	return previews, hasMore, nil
}
