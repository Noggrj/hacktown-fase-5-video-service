package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	videohttp "github.com/noggrj/fiapx-video-service/internal/video/delivery/http"
	"github.com/noggrj/fiapx-video-service/internal/video/domain"
	"github.com/noggrj/fiapx-video-service/internal/video/usecase"

	"github.com/noggrj/fiapx-video-service/internal/platform/httpauth"
	"github.com/noggrj/fiapx-video-service/internal/platform/jwt"
)

const testSecret = "test-secret-at-least-16-bytes"

var testUserID = uuid.New()

// ---------- fakes ----------

type fakeRepo struct {
	mu   sync.Mutex
	byID map[uuid.UUID]*domain.Video
}

func newFakeRepo() *fakeRepo { return &fakeRepo{byID: map[uuid.UUID]*domain.Video{}} }

func (f *fakeRepo) Create(_ context.Context, v *domain.Video) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *v
	f.byID[v.ID] = &cp
	return nil
}
func (f *fakeRepo) Update(_ context.Context, v *domain.Video) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *v
	f.byID[v.ID] = &cp
	return nil
}
func (f *fakeRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Video, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *v
	return &cp, nil
}
func (f *fakeRepo) ListByUser(_ context.Context, userID uuid.UUID) ([]*domain.Video, error) {
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

type fakeStorage struct{}

func (fakeStorage) Upload(context.Context, string, io.Reader, int64) error { return nil }
func (fakeStorage) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://fake-s3/" + key, nil
}

type fakeCache struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newFakeCache() *fakeCache { return &fakeCache{data: map[string][]byte{}} }
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

type fakePublisher struct{}

func (fakePublisher) PublishVideoUploaded(context.Context, string, string, string, string, string) error {
	return nil
}

func silentLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// newTestRouter wires the real handler behind the real httpauth.Middleware,
// so tests exercise the exact same auth path production uses.
func newTestRouter(t *testing.T) (*chi.Mux, *fakeRepo, string) {
	t.Helper()
	repo := newFakeRepo()
	log := silentLogger()
	uploadUC := usecase.NewUploadVideo(repo, fakeStorage{}, newFakeCache(), fakePublisher{}, log)
	listUC := usecase.NewListVideos(repo, newFakeCache(), log)
	getUC := usecase.NewGetVideo(repo, fakeStorage{})
	h := videohttp.NewHandler(uploadUC, listUC, getUC, log)

	issuer, err := jwt.NewIssuer(testSecret)
	if err != nil {
		t.Fatalf("NewIssuer: %v", err)
	}
	verifier, err := jwt.NewVerifier(testSecret)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	token, err := issuer.Issue(testUserID.String(), "user@fiapx.com", time.Hour)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	r := chi.NewRouter()
	r.Use(httpauth.Middleware(verifier))
	h.Register(r)
	return r, repo, "Bearer " + token
}

func multipartVideoBody(t *testing.T, filename, content string) (*bytes.Buffer, string) {
	t.Helper()
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	fw, err := w.CreateFormFile("video", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write([]byte(content)); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return buf, w.FormDataContentType()
}

func TestUpload_MissingFileField_Returns400(t *testing.T) {
	r, _, bearer := newTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/videos", nil)
	req.Header.Set("Authorization", bearer)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestList_WithoutToken_Returns401(t *testing.T) {
	r, _, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/videos", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestUploadThenList_HappyPath(t *testing.T) {
	r, _, bearer := newTestRouter(t)

	body, contentType := multipartVideoBody(t, "clip.mp4", "fake bytes")
	req := httptest.NewRequest(http.MethodPost, "/videos", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", bearer)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/videos", nil)
	listReq.Header.Set("Authorization", bearer)
	listRec := httptest.NewRecorder()
	r.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listRec.Code, listRec.Body.String())
	}
	var resp map[string]any
	_ = json.NewDecoder(listRec.Body).Decode(&resp)
	if resp["total"].(float64) != 1 {
		t.Fatalf("expected 1 video listed, got %v", resp["total"])
	}
}

func TestDownload_NotReady_Returns409(t *testing.T) {
	r, repo, bearer := newTestRouter(t)
	v, _ := domain.NewVideo(testUserID, "clip.mp4", "raw/x/clip.mp4")
	_ = repo.Create(context.Background(), v)

	req := httptest.NewRequest(http.MethodGet, "/videos/"+v.ID.String()+"/download", nil)
	req.Header.Set("Authorization", bearer)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDownload_Ready_ReturnsURL(t *testing.T) {
	r, repo, bearer := newTestRouter(t)
	v, _ := domain.NewVideo(testUserID, "clip.mp4", "raw/x/clip.mp4")
	v.MarkProcessed("processed/x.zip", 5, v.CreatedAt)
	_ = repo.Create(context.Background(), v)

	req := httptest.NewRequest(http.MethodGet, "/videos/"+v.ID.String()+"/download", nil)
	req.Header.Set("Authorization", bearer)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp["url"] == "" {
		t.Fatal("expected non-empty download url")
	}
}

func TestGet_OtherUsersVideo_Returns404(t *testing.T) {
	r, repo, bearer := newTestRouter(t)
	otherOwner := uuid.New()
	v, _ := domain.NewVideo(otherOwner, "clip.mp4", "raw/x/clip.mp4")
	_ = repo.Create(context.Background(), v)

	req := httptest.NewRequest(http.MethodGet, "/videos/"+v.ID.String(), nil)
	req.Header.Set("Authorization", bearer)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestGet_InvalidID_Returns400(t *testing.T) {
	r, _, bearer := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/videos/not-a-uuid", nil)
	req.Header.Set("Authorization", bearer)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
