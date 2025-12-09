package main

import (
	"log"

	"ghostnet/internal/common/config"
	"ghostnet/internal/common/logger"
	"ghostnet/internal/common/server"
	"go.uber.org/zap"
)

type Config struct {
	GRPCPort        string `env:"GRPC_PORT" envDefault:"9090"`
	KafkaBrokers    string `env:"KAFKA_BROKERS" envDefault:"kafka:9092"`
	UserServiceAddr string `env:"USER_SERVICE_ADDR" envDefault:"user-service:9090"`
	PostServiceAddr string `env:"POST_SERVICE_ADDR" envDefault:"post-service:9090"`
	TelegramToken   string `env:"TELEGRAM_BOT_TOKEN" envDefault:"SET_ME"`
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

	logg.Info("notification-service starting",
		zap.String("grpc_port", cfg.GRPCPort),
		zap.String("kafka", cfg.KafkaBrokers),
		zap.String("user_service", cfg.UserServiceAddr),
		zap.String("post_service", cfg.PostServiceAddr),
	)

	if err := server.RunGRPC(logg, cfg.GRPCPort, nil); err != nil {
		logg.Fatal("grpc server failed", zap.Error(err))
	}
}
