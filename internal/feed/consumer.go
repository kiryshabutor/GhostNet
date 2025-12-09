package feed

import (
	"context"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// KafkaHandler слушает post-events, чтобы иметь возможность реагировать на события (логирование/префетч).
type KafkaHandler struct {
	Logger *zap.Logger
}

func (h *KafkaHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (h *KafkaHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h *KafkaHandler) Consume(ctx context.Context, msg *sarama.ConsumerMessage) error {
	h.Logger.Debug("feed-service received event", zap.String("topic", msg.Topic), zap.Int32("partition", msg.Partition), zap.Int64("offset", msg.Offset))
	return nil
}
