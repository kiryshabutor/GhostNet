package kafka

import (
	"context"
	"errors"
	"fmt"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// MessageHandler описывает логику обработки одного сообщения.
type MessageHandler interface {
	Setup(sarama.ConsumerGroupSession) error
	Cleanup(sarama.ConsumerGroupSession) error
	Consume(ctx context.Context, msg *sarama.ConsumerMessage) error
}

// RunConsumerGroup запускает consumer group и блокируется, пока контекст не завершится.
func RunConsumerGroup(ctx context.Context, logger *zap.Logger, brokers, group string, topics []string, handler MessageHandler) error {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V2_8_0_0
	cfg.Consumer.Return.Errors = true
	cfg.Consumer.Offsets.Initial = sarama.OffsetNewest

	list := splitAndTrim(brokers)
	if len(list) == 0 {
		return fmt.Errorf("empty brokers list")
	}

	groupConsumer, err := sarama.NewConsumerGroup(list, group, cfg)
	if err != nil {
		return fmt.Errorf("create consumer group: %w", err)
	}
	defer groupConsumer.Close()

	handlerWrapper := &consumerGroupHandler{handler: handler, logger: logger}

	errs := make(chan error, 1)

	go func() {
		for {
			if err := groupConsumer.Consume(ctx, topics, handlerWrapper); err != nil {
				errs <- err
				return
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()

	for {
		select {
		case err := <-groupConsumer.Errors():
			logger.Warn("consumer error", zap.Error(err))
		case err := <-errs:
			return err
		case <-ctx.Done():
			return nil
		}
	}
}

type consumerGroupHandler struct {
	handler MessageHandler
	logger  *zap.Logger
}

func (h *consumerGroupHandler) Setup(sess sarama.ConsumerGroupSession) error {
	return h.handler.Setup(sess)
}

func (h *consumerGroupHandler) Cleanup(sess sarama.ConsumerGroupSession) error {
	return h.handler.Cleanup(sess)
}

func (h *consumerGroupHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		if err := h.handler.Consume(sess.Context(), msg); err != nil {
			h.logger.Error("failed to handle message", zap.Error(err))
			if !errors.Is(err, context.Canceled) {
				// сохраняем возможность повторной попытки обработки
				continue
			}
			return err
		}
		sess.MarkMessage(msg, "")
	}
	return nil
}

