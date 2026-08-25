package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/noggrj/hacktown-fase-5-video-service/internal/video/domain"
)

type GetVideoUseCase struct {
	videos  domain.VideoRepository
	storage Storage
}

func NewGetVideo(videos domain.VideoRepository, storage Storage) *GetVideoUseCase {
	return &GetVideoUseCase{videos: videos, storage: storage}
}

// Execute fetches a video, enforcing ownership: a video that exists but
// belongs to another user is reported as ErrNotFound (not ErrForbidden)
// so the API never confirms another user's video ID exists.
func (uc *GetVideoUseCase) Execute(ctx context.Context, userID, videoID uuid.UUID) (*domain.Video, error) {
	v, err := uc.videos.GetByID(ctx, videoID)
	if err != nil {
		return nil, err
	}
	if v.UserID != userID {
		return nil, domain.ErrNotFound
	}
	return v, nil
}

const downloadURLTTL = 15 * time.Minute

// PresignDownload returns a time-limited S3 URL for a DONE video's zip.
func (uc *GetVideoUseCase) PresignDownload(ctx context.Context, userID, videoID uuid.UUID) (string, error) {
	v, err := uc.Execute(ctx, userID, videoID)
	if err != nil {
		return "", err
	}
	if v.Status != domain.StatusDone {
		return "", fmt.Errorf("video is not ready for download (status=%s): %w", v.Status, ErrNotReady)
	}
	return uc.storage.PresignGet(ctx, v.S3ZipKey, downloadURLTTL)
}

// ErrNotReady lets HTTP handlers map "video exists but isn't DONE yet"
// to 409/425 instead of a generic 500.
var ErrNotReady = errors.New("video not ready")
