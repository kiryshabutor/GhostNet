package main

import (
	"context"
	"log"

	feedv1 "ghostnet/gen/go/proto/feed/v1"
	interactionv1 "ghostnet/gen/go/proto/interaction/v1"
	postv1 "ghostnet/gen/go/proto/post/v1"
	userv1 "ghostnet/gen/go/proto/user/v1"
	"ghostnet/internal/common/config"
	"ghostnet/internal/common/logger"
	"ghostnet/internal/gateway"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Config struct {
	HTTPPort           string `env:"HTTP_PORT" envDefault:"8080"`
	TelegramBotToken   string `env:"TELEGRAM_BOT_TOKEN" envDefault:"SET_ME"`
	UserServiceAddr    string `env:"USER_SERVICE_ADDR" envDefault:"user-service:9090"`
	PostServiceAddr    string `env:"POST_SERVICE_ADDR" envDefault:"post-service:9090"`
	InteractionService string `env:"INTERACTION_SERVICE_ADDR" envDefault:"interaction-service:9090"`
	FeedServiceAddr    string `env:"FEED_SERVICE_ADDR" envDefault:"feed-service:9090"`
}

func main() {
	ctx := context.Background()

	var cfg Config
	if err := config.Parse(&cfg); err != nil {
		log.Fatalf("parse env: %v", err)
	}

	logg, err := logger.New()
	if err != nil {
		log.Fatalf("logger init: %v", err)
	}
	defer logg.Sync()

	logg.Info("telegram-gateway starting",
		zap.String("http_port", cfg.HTTPPort),
		zap.String("user_service", cfg.UserServiceAddr),
		zap.String("post_service", cfg.PostServiceAddr),
		zap.String("interaction_service", cfg.InteractionService),
		zap.String("feed_service", cfg.FeedServiceAddr),
	)

	userConn, err := grpc.DialContext(ctx, cfg.UserServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logg.Fatal("dial user-service failed", zap.Error(err))
	}
	defer userConn.Close()

	postConn, err := grpc.DialContext(ctx, cfg.PostServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logg.Fatal("dial post-service failed", zap.Error(err))
	}
	defer postConn.Close()

	interactionConn, err := grpc.DialContext(ctx, cfg.InteractionService, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logg.Fatal("dial interaction-service failed", zap.Error(err))
	}
	defer interactionConn.Close()

	feedConn, err := grpc.DialContext(ctx, cfg.FeedServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logg.Fatal("dial feed-service failed", zap.Error(err))
	}
	defer feedConn.Close()

	handler := gateway.NewHandler(
		userv1.NewUserServiceClient(userConn),
		postv1.NewPostServiceClient(postConn),
		interactionv1.NewInteractionServiceClient(interactionConn),
		feedv1.NewFeedServiceClient(feedConn),
		logg,
	)

	r := gin.Default()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	r.POST("/telegram/webhook", handler.HandleWebhook)
	r.GET("/debug/next/:user_id", handler.DebugNextPost)

	if err := r.Run(":" + cfg.HTTPPort); err != nil {
		logg.Fatal("http server failed", zap.Error(err))
	}
}
