package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/noggrj/hacktown-fase-5-video-service/internal/video/domain"
)

const statusListCacheTTL = time.Minute

type ListVideosUseCase struct {
	videos domain.VideoRepository
	cache  Cache
	log    *slog.Logger
}

func NewListVideos(videos domain.VideoRepository, cache Cache, log *slog.Logger) *ListVideosUseCase {
	return &ListVideosUseCase{videos: videos, cache: cache, log: log}
}

// Execute is a read-through cache: a hit skips Postgres entirely; a miss
// (including any cache error, which GetList reports as ok=false) always
// falls back to a normal query, so a Redis outage degrades latency, not
// correctness.
func (uc *ListVideosUseCase) Execute(ctx context.Context, userID uuid.UUID) ([]*domain.Video, error) {
	if cached, ok := uc.cache.GetList(ctx, userID.String()); ok {
		var videos []*domain.Video
		if err := json.Unmarshal(cached, &videos); err == nil {
			return videos, nil
		}
		uc.log.Warn("discarding malformed cache entry", slog.String("userId", userID.String()))
	}

	videos, err := uc.videos.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list videos: %w", err)
	}

	if encoded, err := json.Marshal(videos); err == nil {
		uc.cache.SetList(ctx, userID.String(), encoded, statusListCacheTTL)
	}
	return videos, nil
}
