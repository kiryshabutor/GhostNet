package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrUserNotFound возвращается, когда пользователь не найден.
	ErrUserNotFound = errors.New("user not found")
)

// NotificationSettings хранит пользовательские настройки уведомлений.
type NotificationSettings struct {
	NotifyOnLike    bool
	NotifyOnDislike bool
	NotifyOnComment bool
}

// Repository инкапсулирует доступ к данным user-service.
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) GetUser(ctx context.Context, userID int64) (int64, error) {
	var telegramID int64
	err := r.pool.QueryRow(ctx, `SELECT telegram_id FROM users WHERE id = $1`, userID).Scan(&telegramID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrUserNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get user: %w", err)
	}
	return telegramID, nil
}

func (r *Repository) GetOrCreateUser(ctx context.Context, telegramID int64) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (telegram_id) VALUES ($1)
		ON CONFLICT (telegram_id) DO UPDATE SET telegram_id = EXCLUDED.telegram_id
		RETURNING id
	`, telegramID).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert user: %w", err)
	}

	if err := r.ensureSettingsRow(ctx, id); err != nil {
		return 0, err
	}

	return id, nil
}

func (r *Repository) GetNotificationSettings(ctx context.Context, userID int64) (NotificationSettings, error) {
	var s NotificationSettings
	err := r.pool.QueryRow(ctx, `
		SELECT notify_on_like, notify_on_dislike, notify_on_comment
		FROM user_notification_settings
		WHERE user_id = $1
	`, userID).Scan(&s.NotifyOnLike, &s.NotifyOnDislike, &s.NotifyOnComment)
	if err == nil {
		return s, nil
	}

	if errors.Is(err, pgx.ErrNoRows) {
		exists, checkErr := r.userExists(ctx, userID)
		if checkErr != nil {
			return s, checkErr
		}
		if !exists {
			return s, ErrUserNotFound
		}
		if err := r.ensureSettingsRow(ctx, userID); err != nil {
			return s, err
		}
		return r.GetNotificationSettings(ctx, userID)
	}

	return s, fmt.Errorf("get settings: %w", err)
}

func (r *Repository) UpsertNotificationSettings(ctx context.Context, userID int64, s NotificationSettings) error {
	exists, err := r.userExists(ctx, userID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrUserNotFound
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO user_notification_settings (user_id, notify_on_like, notify_on_dislike, notify_on_comment, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (user_id) DO UPDATE
		SET notify_on_like = EXCLUDED.notify_on_like,
		    notify_on_dislike = EXCLUDED.notify_on_dislike,
		    notify_on_comment = EXCLUDED.notify_on_comment,
		    updated_at = NOW()
	`, userID, s.NotifyOnLike, s.NotifyOnDislike, s.NotifyOnComment)
	if err != nil {
		return fmt.Errorf("upsert settings: %w", err)
	}
	return nil
}

func (r *Repository) ensureSettingsRow(ctx context.Context, userID int64) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_notification_settings (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING
	`, userID)
	if err != nil {
		return fmt.Errorf("ensure settings row: %w", err)
	}
	return nil
}

func (r *Repository) userExists(ctx context.Context, userID int64) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check user exists: %w", err)
	}
	return exists, nil
}
