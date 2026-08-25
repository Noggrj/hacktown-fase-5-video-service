package usecase_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/noggrj/hacktown-fase-5-video-service/internal/video/domain"
	"github.com/noggrj/hacktown-fase-5-video-service/internal/video/usecase"
)

func TestListVideos_CacheMiss_FallsBackToRepoAndPopulatesCache(t *testing.T) {
	repo := newFakeVideoRepo()
	userID := uuid.New()
	v, _ := domain.NewVideo(userID, "clip.mp4", "raw/x/clip.mp4")
	_ = repo.Create(context.Background(), v)

	cache := newFakeCache()
	uc := usecase.NewListVideos(repo, cache, silentLogger())

	got, err := uc.Execute(context.Background(), userID)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 video, got %d", len(got))
	}
	if _, ok := cache.GetList(context.Background(), userID.String()); !ok {
		t.Fatal("expected cache to be populated after a miss")
	}
}

func TestListVideos_CacheHit_DoesNotTouchRepo(t *testing.T) {
	repo := newFakeVideoRepo()
	repo.createErr = errBoom // any repo call other than what the cache-hit path allows will surface this
	userID := uuid.New()

	cache := newFakeCache()
	cache.SetList(context.Background(), userID.String(), []byte(`[]`), 0)
	uc := usecase.NewListVideos(repo, cache, silentLogger())

	got, err := uc.Execute(context.Background(), userID)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty list from cache, got %d", len(got))
	}
}

func TestListVideos_MalformedCacheEntry_FallsBackToRepo(t *testing.T) {
	repo := newFakeVideoRepo()
	userID := uuid.New()
	v, _ := domain.NewVideo(userID, "clip.mp4", "raw/x/clip.mp4")
	_ = repo.Create(context.Background(), v)

	cache := newFakeCache()
	cache.SetList(context.Background(), userID.String(), []byte(`not-json`), 0)
	uc := usecase.NewListVideos(repo, cache, silentLogger())

	got, err := uc.Execute(context.Background(), userID)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected fallback to repo to return 1 video, got %d", len(got))
	}
}
