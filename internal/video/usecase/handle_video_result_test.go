package usecase_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/noggrj/fiapx-video-service/internal/video/domain"
	"github.com/noggrj/fiapx-video-service/internal/video/usecase"
)

func TestHandleVideoResult_MarkProcessed(t *testing.T) {
	repo := newFakeVideoRepo()
	userID := uuid.New()
	v, _ := domain.NewVideo(userID, "clip.mp4", "raw/x/clip.mp4")
	_ = repo.Create(context.Background(), v)

	cache := newFakeCache()
	cache.SetList(context.Background(), userID.String(), []byte(`[]`), 0)

	uc := usecase.NewHandleVideoResult(repo, cache, silentLogger())
	if err := uc.MarkProcessed(context.Background(), v.ID, "processed/x.zip", 7); err != nil {
		t.Fatalf("MarkProcessed: %v", err)
	}

	got, _ := repo.GetByID(context.Background(), v.ID)
	if got.Status != domain.StatusDone || got.S3ZipKey != "processed/x.zip" || got.FrameCount != 7 {
		t.Fatalf("unexpected state: %+v", got)
	}
	if _, ok := cache.GetList(context.Background(), userID.String()); ok {
		t.Fatal("expected cache to be invalidated")
	}
}

func TestHandleVideoResult_MarkFailed(t *testing.T) {
	repo := newFakeVideoRepo()
	userID := uuid.New()
	v, _ := domain.NewVideo(userID, "clip.mp4", "raw/x/clip.mp4")
	_ = repo.Create(context.Background(), v)

	uc := usecase.NewHandleVideoResult(repo, newFakeCache(), silentLogger())
	if err := uc.MarkFailed(context.Background(), v.ID, "ffmpeg exit 1"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	got, _ := repo.GetByID(context.Background(), v.ID)
	if got.Status != domain.StatusFailed || got.ErrorReason != "ffmpeg exit 1" {
		t.Fatalf("unexpected state: %+v", got)
	}
}

func TestHandleVideoResult_UnknownVideo_ReturnsError(t *testing.T) {
	repo := newFakeVideoRepo()
	uc := usecase.NewHandleVideoResult(repo, newFakeCache(), silentLogger())
	if err := uc.MarkProcessed(context.Background(), uuid.New(), "x.zip", 1); err == nil {
		t.Fatal("expected error for unknown video id")
	}
}
