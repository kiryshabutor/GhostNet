package notification

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	eventsv1 "ghostnet/gen/go/proto/events/v1"
	postv1 "ghostnet/gen/go/proto/post/v1"
	userv1 "ghostnet/gen/go/proto/user/v1"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// Handler реализует обработку Kafka-сообщений.
type Handler struct {
	userClient userv1.UserServiceClient
	postClient postv1.PostServiceClient
	botToken   string
	logger     *zap.Logger
}

func NewHandler(userClient userv1.UserServiceClient, postClient postv1.PostServiceClient, botToken string, logger *zap.Logger) *Handler {
	return &Handler{
		userClient: userClient,
		postClient: postClient,
		botToken:   botToken,
		logger:     logger,
	}
}

func (h *Handler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *Handler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *Handler) Consume(ctx context.Context, msg *sarama.ConsumerMessage) error {
	var event eventsv1.PostEvent
	if err := proto.Unmarshal(msg.Value, &event); err != nil {
		h.logger.Warn("failed to unmarshal event", zap.Error(err))
		return nil
	}

	switch event.EventType {
	case eventsv1.EventType_EVENT_TYPE_POST_LIKED:
		return h.handleReaction(ctx, &event, "👍 Новый лайк", func(settings *userv1.GetNotificationSettingsResponse) bool {
			return settings.GetNotifyOnLike()
		})
	case eventsv1.EventType_EVENT_TYPE_POST_DISLIKED:
		return h.handleReaction(ctx, &event, "👎 Новый дизлайк", func(settings *userv1.GetNotificationSettingsResponse) bool {
			return settings.GetNotifyOnDislike()
		})
	case eventsv1.EventType_EVENT_TYPE_COMMENT_ADDED:
		return h.handleReaction(ctx, &event, "💬 Новый комментарий", func(settings *userv1.GetNotificationSettingsResponse) bool {
			return settings.GetNotifyOnComment()
		})
	default:
		// для постов/просмотров уведомлений не шлём
		return nil
	}
}

func (h *Handler) handleReaction(ctx context.Context, event *eventsv1.PostEvent, title string, allow func(*userv1.GetNotificationSettingsResponse) bool) error {
	if event.GetPostAuthorId() == 0 {
		return nil
	}

	settings, err := h.userClient.GetNotificationSettings(ctx, &userv1.GetNotificationSettingsRequest{UserId: event.GetPostAuthorId()})
	if err != nil {
		h.logger.Error("failed to get notification settings", zap.Error(err))
		return nil
	}

	if !allow(settings) {
		return nil
	}

	post, err := h.postClient.GetPost(ctx, &postv1.GetPostRequest{PostId: event.GetPostId()})
	if err != nil {
		h.logger.Error("failed to fetch post for notification", zap.Error(err))
		return nil
	}

	preview := truncateText(post.GetText(), 80)

	userResp, err := h.userClient.GetUser(ctx, &userv1.GetUserRequest{UserId: post.GetAuthorUserId()})
	if err != nil {
		h.logger.Error("failed to get user telegram id", zap.Error(err))
		return nil
	}

	if userResp.GetTelegramId() == 0 {
		return nil
	}

	message := title + "\n" + preview
	if err := h.sendTelegram(ctx, userResp.GetTelegramId(), message); err != nil {
		h.logger.Warn("failed to send telegram notification", zap.Error(err))
		return nil
	}

	return nil
}

func truncateText(text string, limit int) string {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) <= limit {
		return trimmed
	}
	return trimmed[:limit] + "…"
}

func (h *Handler) sendTelegram(ctx context.Context, chatID int64, text string) error {
	if h.botToken == "" || h.botToken == "SET_ME" {
		return nil
	}
	reqBody := strings.NewReader(`{"chat_id":` + strconv.FormatInt(chatID, 10) + `,"text":"` + escape(text) + `"}`)
	url := "https://api.telegram.org/bot" + h.botToken + "/sendMessage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram send status %d", resp.StatusCode)
	}
	return nil
}

func escape(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return replacer.Replace(s)
}
