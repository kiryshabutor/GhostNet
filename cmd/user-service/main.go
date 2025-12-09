package main

import (
	"log"

	"ghostnet/internal/common/config"
	"ghostnet/internal/common/logger"
	"ghostnet/internal/common/server"
	"go.uber.org/zap"
)

type Config struct {
	GRPCPort string `env:"GRPC_PORT" envDefault:"9090"`
	DBURL    string `env:"DB_URL" envDefault:"postgres://app_user:app_password@localhost:5432/app_db?sslmode=disable"`
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

	logg.Info("user-service starting", zap.String("grpc_port", cfg.GRPCPort), zap.String("db", cfg.DBURL))

	if err := server.RunGRPC(logg, cfg.GRPCPort, nil); err != nil {
		logg.Fatal("grpc server failed", zap.Error(err))
	}
}

