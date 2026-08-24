package usecase_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/noggrj/fiapx-video-service/internal/video/domain"
	"github.com/noggrj/fiapx-video-service/internal/video/usecase"
)

func TestUploadVideo_HappyPath(t *testing.T) {
	repo := newFakeVideoRepo()
	storage := newFakeStorage()
	cache := newFakeCache()
	pub := &fakePublisher{}
	uc := usecase.NewUploadVideo(repo, storage, cache, pub, silentLogger())

	userID := uuid.New()
	content := strings.NewReader("fake video bytes")
	v, err := uc.Execute(context.Background(), userID, "clip.mp4", content, int64(content.Len()), "")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if v.Status != domain.StatusPending {
		t.Fatalf("expected PENDING, got %s", v.Status)
	}
	if len(storage.uploaded) != 1 {
		t.Fatalf("expected 1 object uploaded, got %d", len(storage.uploaded))
	}
	if pub.calls != 1 {
		t.Fatalf("expected 1 publish call, got %d", pub.calls)
	}
	stored, err := repo.GetByID(context.Background(), v.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.S3RawKey != v.S3RawKey {
		t.Fatal("s3RawKey mismatch between returned and stored video")
	}
}

func TestUploadVideo_RejectsUnsupportedFormat(t *testing.T) {
	repo := newFakeVideoRepo()
	storage := newFakeStorage()
	uc := usecase.NewUploadVideo(repo, storage, newFakeCache(), &fakePublisher{}, silentLogger())

	_, err := uc.Execute(context.Background(), uuid.New(), "doc.pdf", strings.NewReader("x"), 1, "")
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
	if len(storage.uploaded) != 0 {
		t.Fatal("must not upload to storage when validation fails")
	}
}

func TestUploadVideo_PropagatesStorageError(t *testing.T) {
	repo := newFakeVideoRepo()
	storage := newFakeStorage()
	storage.uploadErr = errBoom
	uc := usecase.NewUploadVideo(repo, storage, newFakeCache(), &fakePublisher{}, silentLogger())

	_, err := uc.Execute(context.Background(), uuid.New(), "clip.mp4", strings.NewReader("x"), 1, "")
	if err == nil {
		t.Fatal("expected storage error to propagate")
	}
}

func TestUploadVideo_InvalidatesUserCache(t *testing.T) {
	repo := newFakeVideoRepo()
	storage := newFakeStorage()
	cache := newFakeCache()
	userID := uuid.New()
	cache.SetList(context.Background(), userID.String(), []byte(`[]`), 0)

	uc := usecase.NewUploadVideo(repo, storage, cache, &fakePublisher{}, silentLogger())
	if _, err := uc.Execute(context.Background(), userID, "clip.mp4", strings.NewReader("x"), 1, ""); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, ok := cache.GetList(context.Background(), userID.String()); ok {
		t.Fatal("expected cache entry to be invalidated after upload")
	}
}

func TestUploadVideo_SurvivesPublishFailure(t *testing.T) {
	repo := newFakeVideoRepo()
	storage := newFakeStorage()
	pub := &fakePublisher{err: errBoom}
	uc := usecase.NewUploadVideo(repo, storage, newFakeCache(), pub, silentLogger())

	v, err := uc.Execute(context.Background(), uuid.New(), "clip.mp4", strings.NewReader("x"), 1, "")
	if err != nil {
		t.Fatalf("Execute must not fail just because publish failed: %v", err)
	}
	if v.Status != domain.StatusPending {
		t.Fatalf("expected PENDING despite publish failure, got %s", v.Status)
	}
}
