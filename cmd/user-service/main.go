package main

import (
	"context"
	"log"

	userv1 "ghostnet/gen/go/proto/user/v1"
	"ghostnet/internal/common/config"
	"ghostnet/internal/common/db"
	"ghostnet/internal/common/logger"
	"ghostnet/internal/common/server"
	usersvc "ghostnet/internal/user"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type Config struct {
	GRPCPort string `env:"GRPC_PORT" envDefault:"9090"`
	DBURL    string `env:"DB_URL" envDefault:"postgres://app_user:app_password@localhost:5432/app_db?sslmode=disable"`
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

	logg.Info("user-service starting", zap.String("grpc_port", cfg.GRPCPort), zap.String("db", cfg.DBURL))

	pool, err := db.Connect(ctx, cfg.DBURL)
	if err != nil {
		logg.Fatal("db connect failed", zap.Error(err))
	}
	defer pool.Close()

	if err := usersvc.RunMigrations(ctx, pool); err != nil {
		logg.Fatal("db migration failed", zap.Error(err))
	}

	repo := usersvc.NewRepository(pool)
	svc := usersvc.NewService(repo, logg)

	register := func(g *grpc.Server) {
		userv1.RegisterUserServiceServer(g, svc)
	}

	if err := server.RunGRPC(logg, cfg.GRPCPort, register); err != nil {
		logg.Fatal("grpc server failed", zap.Error(err))
	}
}
