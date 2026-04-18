package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/rs/zerolog/log"
	kafkago "github.com/segmentio/kafka-go"
)

type Handler func(ctx context.Context, msg []byte) error

type Consumer struct {
	reader  *kafkago.Reader
	maxRetries int
}

func NewConsumer(brokers []string, topic, groupID string) *Consumer {
	r := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: 0, // manual commit
		StartOffset:    kafkago.FirstOffset,
		MaxWait:        500 * time.Millisecond,
	})
	return &Consumer{reader: r, maxRetries: 3}
}

// Consume runs the consume loop until ctx is cancelled.
// On handler error it retries up to maxRetries, then publishes to DLQ.
func (c *Consumer) Consume(ctx context.Context, brokers []string, handler Handler) {
	producer := NewProducer(brokers)
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Error().Err(err).Msg("kafka fetch error")
			continue
		}

		var lastErr error
		for attempt := range c.maxRetries {
			lastErr = handler(ctx, msg.Value)
			if lastErr == nil {
				break
			}
			log.Warn().Err(lastErr).Int("attempt", attempt+1).Msg("handler error, retrying")
			time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
		}

		if lastErr != nil {
			log.Error().Err(lastErr).Str("topic", msg.Topic).Msg("max retries exhausted, sending to DLQ")
			_ = producer.Publish(ctx, DLQ(msg.Topic), string(msg.Key), json.RawMessage(msg.Value))
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			log.Error().Err(err).Msg("kafka commit error")
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
