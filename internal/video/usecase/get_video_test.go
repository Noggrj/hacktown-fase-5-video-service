package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/noggrj/fiapx-video-service/internal/video/domain"
	"github.com/noggrj/fiapx-video-service/internal/video/usecase"
)

func TestGetVideo_OwnerCanFetch(t *testing.T) {
	repo := newFakeVideoRepo()
	userID := uuid.New()
	v, _ := domain.NewVideo(userID, "clip.mp4", "raw/x/clip.mp4")
	_ = repo.Create(context.Background(), v)

	uc := usecase.NewGetVideo(repo, newFakeStorage())
	got, err := uc.Execute(context.Background(), userID, v.ID)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got.ID != v.ID {
		t.Fatal("id mismatch")
	}
}

func TestGetVideo_OtherUsersVideo_ReturnsNotFound(t *testing.T) {
	repo := newFakeVideoRepo()
	owner := uuid.New()
	v, _ := domain.NewVideo(owner, "clip.mp4", "raw/x/clip.mp4")
	_ = repo.Create(context.Background(), v)

	uc := usecase.NewGetVideo(repo, newFakeStorage())
	_, err := uc.Execute(context.Background(), uuid.New(), v.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for a video owned by someone else, got %v", err)
	}
}

func TestPresignDownload_RejectsWhenNotDone(t *testing.T) {
	repo := newFakeVideoRepo()
	userID := uuid.New()
	v, _ := domain.NewVideo(userID, "clip.mp4", "raw/x/clip.mp4")
	_ = repo.Create(context.Background(), v)

	uc := usecase.NewGetVideo(repo, newFakeStorage())
	_, err := uc.PresignDownload(context.Background(), userID, v.ID)
	if !errors.Is(err, usecase.ErrNotReady) {
		t.Fatalf("expected ErrNotReady, got %v", err)
	}
}

func TestPresignDownload_HappyPath(t *testing.T) {
	repo := newFakeVideoRepo()
	userID := uuid.New()
	v, _ := domain.NewVideo(userID, "clip.mp4", "raw/x/clip.mp4")
	v.MarkProcessed("processed/x.zip", 10, v.CreatedAt)
	_ = repo.Create(context.Background(), v)

	uc := usecase.NewGetVideo(repo, newFakeStorage())
	url, err := uc.PresignDownload(context.Background(), userID, v.ID)
	if err != nil {
		t.Fatalf("PresignDownload: %v", err)
	}
	if url == "" {
		t.Fatal("expected non-empty presigned url")
	}
}
