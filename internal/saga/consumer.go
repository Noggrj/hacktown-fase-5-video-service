package saga

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"

	events "github.com/noggrj/hacktown-fase-5-events"
	"github.com/noggrj/hacktown-fase-5-events/idempotency"
	"github.com/noggrj/hacktown-fase-5-events/payloads"
	eventskafka "github.com/noggrj/hacktown-fase-5-events/transport/kafka"

	"github.com/noggrj/hacktown-fase-5-video-service/internal/video/usecase"
)

// ResultConsumer reads a single topic (video.processed OR video.failed —
// callers construct one of each) and applies it via
// usecase.HandleVideoResultUseCase, guarded by idempotency so a
// redelivered Kafka message doesn't double-apply a status transition.
type ResultConsumer struct {
	reader       *kafkago.Reader
	dlq          eventskafka.Writer
	consumerName string
	store        idempotency.Store
	handler      *usecase.HandleVideoResultUseCase
}

func NewResultConsumer(
	reader *kafkago.Reader,
	dlq eventskafka.Writer,
	consumerName string,
	store idempotency.Store,
	handler *usecase.HandleVideoResultUseCase,
) *ResultConsumer {
	return &ResultConsumer{reader: reader, dlq: dlq, consumerName: consumerName, store: store, handler: handler}
}

func (c *ResultConsumer) Start(ctx context.Context, log *slog.Logger) error {
	inner, err := eventskafka.NewConsumer(c.reader, c.dlq)
	if err != nil {
		return fmt.Errorf("setup consumer: %w", err)
	}

	dispatch := func(ctx context.Context, env *events.Envelope, _ kafkago.Message) error {
		videoID, err := uuid.Parse(env.CorrelationID)
		if err != nil {
			return fmt.Errorf("invalid correlationId %q: %w", env.CorrelationID, err)
		}
		log.Info("event received",
			slog.String("eventId", env.EventID),
			slog.String("type", string(env.Type)),
			slog.String("correlationId", env.CorrelationID))

		switch env.Type {
		case events.TopicVideoProcessed:
			var p payloads.VideoProcessed
			if err := env.Decode(&p); err != nil {
				return err
			}
			return c.handler.MarkProcessed(ctx, videoID, p.S3ZipKey, p.FrameCount)
		case events.TopicVideoFailed:
			var p payloads.VideoFailed
			if err := env.Decode(&p); err != nil {
				return err
			}
			return c.handler.MarkFailed(ctx, videoID, p.Reason)
		default:
			return fmt.Errorf("no handler for topic %s", env.Type)
		}
	}

	withIdem := idempotency.Middleware(c.store, c.consumerName, dispatch)
	log.Info("saga consumer ready", slog.String("consumer", c.consumerName))
	return inner.Consume(ctx, withIdem)
}
