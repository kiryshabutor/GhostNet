package server

import (
	"fmt"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// RunGRPC поднимает gRPC-сервер и блокируется до остановки.
func RunGRPC(logger *zap.Logger, port string, register func(*grpc.Server)) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	srv := grpc.NewServer()
	if register != nil {
		register(srv)
	}

	logger.Info("gRPC server started", zap.String("port", port))
	return srv.Serve(lis)
}
