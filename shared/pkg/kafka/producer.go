package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/rs/zerolog/log"
	kafkago "github.com/segmentio/kafka-go"
)

type Producer struct {
	brokers []string
}

func NewProducer(brokers []string) *Producer {
	return &Producer{brokers: brokers}
}

func (p *Producer) Publish(ctx context.Context, topic string, key string, event any) error {
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}

	w := &kafkago.Writer{
		Addr:                   kafkago.TCP(p.brokers...),
		Topic:                  topic,
		Balancer:               &kafkago.LeastBytes{},
		WriteTimeout:           5 * time.Second,
		RequiredAcks:           kafkago.RequireOne,
		AllowAutoTopicCreation: true,
	}
	defer w.Close()

	msg := kafkago.Message{Value: b}
	if key != "" {
		msg.Key = []byte(key)
	}

	if err := w.WriteMessages(ctx, msg); err != nil {
		// Publish to DLQ on failure so no event is lost.
		p.publishDLQ(ctx, topic, b)
		return err
	}
	return nil
}

func (p *Producer) publishDLQ(ctx context.Context, originalTopic string, payload []byte) {
	w := &kafkago.Writer{
		Addr:                   kafkago.TCP(p.brokers...),
		Topic:                  DLQ(originalTopic),
		AllowAutoTopicCreation: true,
		WriteTimeout:           3 * time.Second,
	}
	defer w.Close()
	if err := w.WriteMessages(ctx, kafkago.Message{Value: payload}); err != nil {
		log.Error().Err(err).Str("dlq", DLQ(originalTopic)).Msg("failed to write to DLQ")
	}
}
