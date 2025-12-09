package main

import (
	"context"
	"log"

	feedv1 "ghostnet/gen/go/proto/feed/v1"
	"ghostnet/internal/common/config"
	"ghostnet/internal/common/db"
	"ghostnet/internal/common/kafka"
	"ghostnet/internal/common/logger"
	"ghostnet/internal/common/server"
	feedsvc "ghostnet/internal/feed"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type Config struct {
	GRPCPort     string `env:"GRPC_PORT" envDefault:"9090"`
	DBURL        string `env:"DB_URL" envDefault:"postgres://app_user:app_password@localhost:5432/app_db?sslmode=disable"`
	KafkaBrokers string `env:"KAFKA_BROKERS" envDefault:"kafka:9092"`
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

	logg.Info("feed-service starting", zap.String("grpc_port", cfg.GRPCPort), zap.String("db", cfg.DBURL), zap.String("kafka", cfg.KafkaBrokers))

	pool, err := db.Connect(ctx, cfg.DBURL)
	if err != nil {
		logg.Fatal("db connect failed", zap.Error(err))
	}
	defer pool.Close()

	repo := feedsvc.NewRepository(pool)
	svc := feedsvc.NewService(repo, logg)

	if cfg.KafkaBrokers != "" {
		handler := &feedsvc.KafkaHandler{Logger: logg}
		go func() {
			if err := kafka.RunConsumerGroup(ctx, logg, cfg.KafkaBrokers, "feed-service", []string{"post-events"}, handler); err != nil && ctx.Err() == nil {
				logg.Fatal("kafka consumer stopped", zap.Error(err))
			}
		}()
	}

	register := func(g *grpc.Server) {
		feedv1.RegisterFeedServiceServer(g, svc)
	}

	if err := server.RunGRPC(logg, cfg.GRPCPort, register); err != nil {
		logg.Fatal("grpc server failed", zap.Error(err))
	}
}
