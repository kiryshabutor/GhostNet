package logger

import "go.uber.org/zap"

// New возвращает production-логгер.
func New() (*zap.Logger, error) {
	return zap.NewProduction()
}
