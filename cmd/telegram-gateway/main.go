package main

import (
	"log"

	"ghostnet/internal/common/config"
	"ghostnet/internal/common/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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

	r := gin.Default()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	if err := r.Run(":" + cfg.HTTPPort); err != nil {
		logg.Fatal("http server failed", zap.Error(err))
	}
}
