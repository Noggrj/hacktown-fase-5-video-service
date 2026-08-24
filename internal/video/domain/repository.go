package domain

import (
	"context"

	"github.com/google/uuid"
)

// VideoRepository persists Video aggregates. Update is used both by the
// HTTP-triggered flows and by the Kafka consumers reacting to
// video.processed/video.failed.
type VideoRepository interface {
	Create(ctx context.Context, v *Video) error
	Update(ctx context.Context, v *Video) error
	GetByID(ctx context.Context, id uuid.UUID) (*Video, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*Video, error)
}
