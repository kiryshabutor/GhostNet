package main

import (
	"context"
	"log"

	postv1 "ghostnet/gen/go/proto/post/v1"
	"ghostnet/internal/common/config"
	"ghostnet/internal/common/db"
	"ghostnet/internal/common/kafka"
	"ghostnet/internal/common/logger"
	"ghostnet/internal/common/server"
	postsvc "ghostnet/internal/post"
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

	logg.Info("post-service starting", zap.String("grpc_port", cfg.GRPCPort), zap.String("db", cfg.DBURL), zap.String("kafka", cfg.KafkaBrokers))

	pool, err := db.Connect(ctx, cfg.DBURL)
	if err != nil {
		logg.Fatal("db connect failed", zap.Error(err))
	}
	defer pool.Close()

	if err := postsvc.RunMigrations(ctx, pool); err != nil {
		logg.Fatal("db migration failed", zap.Error(err))
	}

	var producer *kafka.Producer
	if cfg.KafkaBrokers != "" {
		p, err := kafka.NewProducer(cfg.KafkaBrokers)
		if err != nil {
			logg.Warn("kafka producer init failed, events will be skipped", zap.Error(err))
		} else {
			producer = p
			defer producer.Close()
		}
	}

	repo := postsvc.NewRepository(pool)
	svc := postsvc.NewService(repo, producer, logg)

	register := func(g *grpc.Server) {
		postv1.RegisterPostServiceServer(g, svc)
	}

	if err := server.RunGRPC(logg, cfg.GRPCPort, register); err != nil {
		logg.Fatal("grpc server failed", zap.Error(err))
	}
}
