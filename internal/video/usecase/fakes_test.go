package usecase_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/noggrj/fiapx-video-service/internal/video/domain"
)

// ---------- fake VideoRepository ----------

type fakeVideoRepo struct {
	mu        sync.Mutex
	byID      map[uuid.UUID]*domain.Video
	createErr error
}

func newFakeVideoRepo() *fakeVideoRepo {
	return &fakeVideoRepo{byID: make(map[uuid.UUID]*domain.Video)}
}

func (f *fakeVideoRepo) Create(_ context.Context, v *domain.Video) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return f.createErr
	}
	cp := *v
	f.byID[v.ID] = &cp
	return nil
}

func (f *fakeVideoRepo) Update(_ context.Context, v *domain.Video) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[v.ID]; !ok {
		return domain.ErrNotFound
	}
	cp := *v
	f.byID[v.ID] = &cp
	return nil
}

func (f *fakeVideoRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Video, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *v
	return &cp, nil
}

func (f *fakeVideoRepo) ListByUser(_ context.Context, userID uuid.UUID) ([]*domain.Video, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*domain.Video
	for _, v := range f.byID {
		if v.UserID == userID {
			cp := *v
			out = append(out, &cp)
		}
	}
	return out, nil
}

// ---------- fake Storage ----------

type fakeStorage struct {
	mu         sync.Mutex
	uploaded   map[string][]byte
	uploadErr  error
	presignErr error
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{uploaded: make(map[string][]byte)}
}

func (f *fakeStorage) Upload(_ context.Context, key string, body io.Reader, _ int64) error {
	if f.uploadErr != nil {
		return f.uploadErr
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.uploaded[key] = data
	return nil
}

func (f *fakeStorage) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	if f.presignErr != nil {
		return "", f.presignErr
	}
	return "https://fake-s3/" + key, nil
}

// ---------- fake Cache (real in-memory behavior, not a no-op) ----------

type fakeCache struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newFakeCache() *fakeCache { return &fakeCache{data: make(map[string][]byte)} }

func (c *fakeCache) GetList(_ context.Context, userID string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.data[userID]
	return v, ok
}

func (c *fakeCache) SetList(_ context.Context, userID string, data []byte, _ time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[userID] = data
}

func (c *fakeCache) Invalidate(_ context.Context, userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, userID)
}

// ---------- fake Publisher ----------

type fakePublisher struct {
	mu    sync.Mutex
	calls int
	err   error
}

func (p *fakePublisher) PublishVideoUploaded(context.Context, string, string, string, string, string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	return p.err
}

var errBoom = errors.New("boom")

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
