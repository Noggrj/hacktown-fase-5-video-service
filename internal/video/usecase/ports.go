// ports.go declares the infrastructure interfaces the usecases depend
// on. Concrete implementations live under internal/platform (Storage,
// Cache) and internal/saga (Publisher) — kept out of internal/video so
// this package has zero knowledge of S3, Redis or Kafka.
package usecase

import (
	"context"
	"io"
	"time"
)

// Storage is the subset of S3 operations the Video Service needs.
type Storage interface {
	Upload(ctx context.Context, key string, body io.Reader, contentLength int64) error
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// Cache holds the JSON-encoded status listing per user. A miss or error
// is communicated as (nil, false) — callers must fall back to Postgres,
// never treat a cache outage as a hard failure.
type Cache interface {
	GetList(ctx context.Context, userID string) ([]byte, bool)
	SetList(ctx context.Context, userID string, data []byte, ttl time.Duration)
	Invalidate(ctx context.Context, userID string)
}

// Publisher emits the one event this service produces.
type Publisher interface {
	PublishVideoUploaded(ctx context.Context, traceparent, videoID, userID, filename, s3RawKey string) error
}
