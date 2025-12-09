package gateway

import (
	"net/http"
	"strconv"

	feedv1 "ghostnet/gen/go/proto/feed/v1"
	interactionv1 "ghostnet/gen/go/proto/interaction/v1"
	postv1 "ghostnet/gen/go/proto/post/v1"
	userv1 "ghostnet/gen/go/proto/user/v1"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Handler struct {
	userClient        userv1.UserServiceClient
	postClient        postv1.PostServiceClient
	interactionClient interactionv1.InteractionServiceClient
	feedClient        feedv1.FeedServiceClient
	logger            *zap.Logger
}

func NewHandler(
	userClient userv1.UserServiceClient,
	postClient postv1.PostServiceClient,
	interactionClient interactionv1.InteractionServiceClient,
	feedClient feedv1.FeedServiceClient,
	logger *zap.Logger,
) *Handler {
	return &Handler{
		userClient:        userClient,
		postClient:        postClient,
		interactionClient: interactionClient,
		feedClient:        feedClient,
		logger:            logger,
	}
}

type telegramUpdate struct {
	Message *struct {
		MessageID int64 `json:"message_id"`
		Chat      *struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
}

// HandleWebhook — минимальная заглушка вебхука Telegram.
func (h *Handler) HandleWebhook(c *gin.Context) {
	var upd telegramUpdate
	if err := c.BindJSON(&upd); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid update"})
		return
	}

	var telegramID int64
	if upd.Message != nil && upd.Message.Chat != nil {
		telegramID = upd.Message.Chat.ID
	}

	if telegramID > 0 {
		if _, err := h.userClient.GetOrCreateUser(c, &userv1.GetOrCreateUserRequest{TelegramId: telegramID}); err != nil {
			h.logger.Warn("failed to register user", zap.Error(err))
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// DebugNextPost возвращает следующий пост для пользователя (упрощено).
func (h *Handler) DebugNextPost(c *gin.Context) {
	userIDStr := c.Param("user_id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil || userID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	resp, err := h.feedClient.GetNextPostForUser(c, &feedv1.GetNextPostForUserRequest{UserId: userID})
	if err != nil {
		h.logger.Warn("failed to fetch next post", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch"})
		return
	}

	if !resp.GetHasPost() {
		c.JSON(http.StatusOK, gin.H{"has_post": false})
		return
	}

	post, err := h.postClient.GetPost(c, &postv1.GetPostRequest{PostId: resp.GetPostId()})
	if err != nil {
		h.logger.Warn("failed to fetch post", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch post"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"has_post": true,
		"post":     post,
	})
}
