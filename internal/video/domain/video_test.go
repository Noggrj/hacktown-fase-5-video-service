package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/noggrj/fiapx-video-service/internal/video/domain"
)

func TestNewVideo_HappyPath(t *testing.T) {
	userID := uuid.New()
	v, err := domain.NewVideo(userID, "clip.mp4", "raw/some-id/clip.mp4")
	if err != nil {
		t.Fatalf("NewVideo: %v", err)
	}
	if v.Status != domain.StatusPending {
		t.Fatalf("expected PENDING, got %s", v.Status)
	}
	if v.UserID != userID {
		t.Fatal("userId mismatch")
	}
}

func TestNewVideo_RejectsUnsupportedExtension(t *testing.T) {
	if _, err := domain.NewVideo(uuid.New(), "document.pdf", "raw/x/document.pdf"); err == nil {
		t.Fatal("expected error for unsupported extension")
	}
}

func TestNewVideo_RejectsNilUser(t *testing.T) {
	if _, err := domain.NewVideo(uuid.Nil, "clip.mp4", "raw/x/clip.mp4"); err == nil {
		t.Fatal("expected error for nil userId")
	}
}

func TestIsValidExtension_IsCaseInsensitive(t *testing.T) {
	if !domain.IsValidExtension("CLIP.MP4") {
		t.Fatal("expected .MP4 (uppercase) to be valid")
	}
	if domain.IsValidExtension("clip.txt") {
		t.Fatal("expected .txt to be invalid")
	}
}

func TestMarkProcessed_SetsFieldsAndStatus(t *testing.T) {
	v, _ := domain.NewVideo(uuid.New(), "clip.mp4", "raw/x/clip.mp4")
	at := time.Now().UTC()
	v.MarkProcessed("processed/x.zip", 42, at)
	if v.Status != domain.StatusDone || v.S3ZipKey != "processed/x.zip" || v.FrameCount != 42 {
		t.Fatalf("unexpected state after MarkProcessed: %+v", v)
	}
}

func TestMarkFailed_SetsFieldsAndStatus(t *testing.T) {
	v, _ := domain.NewVideo(uuid.New(), "clip.mp4", "raw/x/clip.mp4")
	at := time.Now().UTC()
	v.MarkFailed("ffmpeg exit 1", at)
	if v.Status != domain.StatusFailed || v.ErrorReason != "ffmpeg exit 1" {
		t.Fatalf("unexpected state after MarkFailed: %+v", v)
	}
}

func TestRawKeyAndZipKey_AreDeterministic(t *testing.T) {
	id := uuid.New()
	if got := domain.RawKey(id, "clip.mp4"); got != "raw/"+id.String()+"/clip.mp4" {
		t.Fatalf("unexpected raw key: %s", got)
	}
	if got := domain.ZipKey(id); got != "processed/"+id.String()+".zip" {
		t.Fatalf("unexpected zip key: %s", got)
	}
}
