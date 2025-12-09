package kafka

import (
	"context"
	"fmt"
	"strings"

	"github.com/IBM/sarama"
)

// Producer обёртка над синхронным продюсером для Kafka.
type Producer struct {
	inner sarama.SyncProducer
}

// NewProducer создаёт продюсер с базовыми настройками.
func NewProducer(brokers string) (*Producer, error) {
	cfg := sarama.NewConfig()
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Retry.Max = 3
	cfg.Producer.Return.Successes = true

	list := splitAndTrim(brokers)
	if len(list) == 0 {
		return nil, fmt.Errorf("empty brokers list")
	}

	p, err := sarama.NewSyncProducer(list, cfg)
	if err != nil {
		return nil, fmt.Errorf("create producer: %w", err)
	}
	return &Producer{inner: p}, nil
}

// Send публикует сообщение.
func (p *Producer) Send(ctx context.Context, topic string, key, value []byte) error {
	_, _, err := p.inner.SendMessage(&sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.ByteEncoder(key),
		Value: sarama.ByteEncoder(value),
	})
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	return nil
}

// Close закрывает продюсер.
func (p *Producer) Close() error {
	return p.inner.Close()
}

func splitAndTrim(brokers string) []string {
	raw := strings.Split(brokers, ",")
	out := make([]string, 0, len(raw))
	for _, b := range raw {
		if trimmed := strings.TrimSpace(b); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
