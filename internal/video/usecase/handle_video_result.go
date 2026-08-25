package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/noggrj/hacktown-fase-5-video-service/internal/video/domain"
)

// HandleVideoResultUseCase is the single writer for status transitions
// triggered by the Processing Worker's events — Video Service owns the
// videos table, so both video.processed and video.failed land here
// instead of the worker updating Postgres directly.
type HandleVideoResultUseCase struct {
	videos domain.VideoRepository
	cache  Cache
	log    *slog.Logger
}

func NewHandleVideoResult(videos domain.VideoRepository, cache Cache, log *slog.Logger) *HandleVideoResultUseCase {
	return &HandleVideoResultUseCase{videos: videos, cache: cache, log: log}
}

func (uc *HandleVideoResultUseCase) MarkProcessed(ctx context.Context, videoID uuid.UUID, s3ZipKey string, frameCount int) error {
	v, err := uc.videos.GetByID(ctx, videoID)
	if err != nil {
		return fmt.Errorf("get video %s: %w", videoID, err)
	}
	v.MarkProcessed(s3ZipKey, frameCount, time.Now().UTC())
	if err := uc.videos.Update(ctx, v); err != nil {
		return fmt.Errorf("update video %s: %w", videoID, err)
	}
	uc.cache.Invalidate(ctx, v.UserID.String())
	uc.log.Info("video processed", slog.String("videoId", videoID.String()), slog.Int("frameCount", frameCount))
	return nil
}

func (uc *HandleVideoResultUseCase) MarkFailed(ctx context.Context, videoID uuid.UUID, reason string) error {
	v, err := uc.videos.GetByID(ctx, videoID)
	if err != nil {
		return fmt.Errorf("get video %s: %w", videoID, err)
	}
	v.MarkFailed(reason, time.Now().UTC())
	if err := uc.videos.Update(ctx, v); err != nil {
		return fmt.Errorf("update video %s: %w", videoID, err)
	}
	uc.cache.Invalidate(ctx, v.UserID.String())
	uc.log.Warn("video failed", slog.String("videoId", videoID.String()), slog.String("reason", reason))
	return nil
}
