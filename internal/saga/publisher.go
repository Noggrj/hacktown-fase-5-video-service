// Package saga wires Kafka transport for the Video Service:
//   - Publisher emits video.uploaded
//   - ResultConsumer subscribes to video.processed and video.failed
package saga

import (
	"context"
	"fmt"
	"time"

	events "github.com/noggrj/fiapx-events"
	"github.com/noggrj/fiapx-events/payloads"
	eventskafka "github.com/noggrj/fiapx-events/transport/kafka"
)

const sourceName = "video-service"

// Publisher implements usecase.Publisher.
type Publisher struct {
	inner *eventskafka.Publisher
}

func NewPublisher(inner *eventskafka.Publisher) (*Publisher, error) {
	if inner == nil {
		return nil, fmt.Errorf("nil kafka publisher")
	}
	return &Publisher{inner: inner}, nil
}

func (p *Publisher) PublishVideoUploaded(ctx context.Context, traceparent, videoID, userID, userEmail, filename, s3RawKey string) error {
	payload := payloads.VideoUploaded{
		VideoID:    videoID,
		UserID:     userID,
		UserEmail:  userEmail,
		Filename:   filename,
		S3RawKey:   s3RawKey,
		UploadedAt: time.Now().UTC(),
	}
	env, err := events.NewEnvelope(events.TopicVideoUploaded, sourceName, videoID, traceparent, payload)
	if err != nil {
		return err
	}
	return p.inner.Publish(ctx, env)
}
