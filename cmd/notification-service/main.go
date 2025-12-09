package main

import (
	"context"
	"log"

	postv1 "ghostnet/gen/go/proto/post/v1"
	userv1 "ghostnet/gen/go/proto/user/v1"
	"ghostnet/internal/common/config"
	"ghostnet/internal/common/kafka"
	"ghostnet/internal/common/logger"
	"ghostnet/internal/common/server"
	"ghostnet/internal/notification"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Config struct {
	GRPCPort         string `env:"GRPC_PORT" envDefault:"9090"`
	KafkaBrokers     string `env:"KAFKA_BROKERS" envDefault:"kafka:9092"`
	UserServiceAddr  string `env:"USER_SERVICE_ADDR" envDefault:"user-service:9090"`
	PostServiceAddr  string `env:"POST_SERVICE_ADDR" envDefault:"post-service:9090"`
	TelegramBotToken string `env:"TELEGRAM_BOT_TOKEN" envDefault:"SET_ME"`
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

	logg.Info("notification-service starting",
		zap.String("grpc_port", cfg.GRPCPort),
		zap.String("kafka", cfg.KafkaBrokers),
		zap.String("user_service", cfg.UserServiceAddr),
		zap.String("post_service", cfg.PostServiceAddr),
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

	userClient := userv1.NewUserServiceClient(userConn)
	postClient := postv1.NewPostServiceClient(postConn)

	handler := notification.NewHandler(userClient, postClient, logg)

	go func() {
		if err := kafka.RunConsumerGroup(ctx, logg, cfg.KafkaBrokers, "notification-service", []string{"post-events"}, handler); err != nil && ctx.Err() == nil {
			logg.Fatal("kafka consumer stopped", zap.Error(err))
		}
	}()

	if err := server.RunGRPC(logg, cfg.GRPCPort, nil); err != nil {
		logg.Fatal("grpc server failed", zap.Error(err))
	}
}
