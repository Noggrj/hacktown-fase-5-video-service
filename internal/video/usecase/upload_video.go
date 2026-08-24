package usecase

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/google/uuid"

	"github.com/noggrj/fiapx-video-service/internal/video/domain"
)

type UploadVideoUseCase struct {
	videos  domain.VideoRepository
	storage Storage
	cache   Cache
	pub     Publisher
	log     *slog.Logger
}

func NewUploadVideo(videos domain.VideoRepository, storage Storage, cache Cache, pub Publisher, log *slog.Logger) *UploadVideoUseCase {
	return &UploadVideoUseCase{videos: videos, storage: storage, cache: cache, pub: pub, log: log}
}

// Execute uploads the raw file to S3, persists a PENDING row and
// publishes video.uploaded — the actual ffmpeg processing happens
// asynchronously in the Processing Worker. traceparent may be empty.
func (uc *UploadVideoUseCase) Execute(ctx context.Context, userID uuid.UUID, filename string, content io.Reader, size int64, traceparent string) (*domain.Video, error) {
	if !domain.IsValidExtension(filename) {
		return nil, fmt.Errorf("unsupported video format: %s", filename)
	}

	videoID := uuid.New()
	s3Key := domain.RawKey(videoID, filename)

	if err := uc.storage.Upload(ctx, s3Key, content, size); err != nil {
		return nil, fmt.Errorf("upload to storage: %w", err)
	}

	v, err := domain.NewVideo(userID, filename, s3Key)
	if err != nil {
		return nil, err
	}
	v.ID = videoID // keep the ID used to build s3Key

	if err := uc.videos.Create(ctx, v); err != nil {
		return nil, fmt.Errorf("persist video: %w", err)
	}

	if err := uc.pub.PublishVideoUploaded(ctx, traceparent, v.ID.String(), v.UserID.String(), v.Filename, v.S3RawKey); err != nil {
		// The row and the S3 object already exist — log and let the video
		// stay PENDING. A future retry/reconciliation job could republish;
		// out of scope for this hackathon's timeline.
		uc.log.Error("failed to publish video.uploaded", slog.String("videoId", v.ID.String()), slog.Any("error", err))
	}

	uc.cache.Invalidate(ctx, userID.String())
	uc.log.Info("video uploaded", slog.String("videoId", v.ID.String()), slog.String("userId", userID.String()))
	return v, nil
}
