package config

import "github.com/caarlos0/env/v10"

// Parse заполняет структуру из переменных окружения.
func Parse(cfg interface{}) error {
	return env.Parse(cfg)
}

