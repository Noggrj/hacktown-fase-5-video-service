// Package http provides Chi handlers for the Video REST surface. Every
// route here is protected — httpauth.Middleware is applied by main.go
// for the whole /videos subtree.
package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/noggrj/hacktown-fase-5-video-service/internal/platform/httpauth"
	"github.com/noggrj/hacktown-fase-5-video-service/internal/video/domain"
	"github.com/noggrj/hacktown-fase-5-video-service/internal/video/usecase"
)

const maxUploadBytes = 500 << 20 // 500MB

type Handler struct {
	upload *usecase.UploadVideoUseCase
	list   *usecase.ListVideosUseCase
	get    *usecase.GetVideoUseCase
	log    *slog.Logger
}

func NewHandler(u *usecase.UploadVideoUseCase, l *usecase.ListVideosUseCase, g *usecase.GetVideoUseCase, log *slog.Logger) *Handler {
	return &Handler{upload: u, list: l, get: g, log: log}
}

func (h *Handler) Register(r chi.Router) {
	r.Post("/videos", h.HandleUpload)
	r.Get("/videos", h.HandleList)
	r.Get("/videos/{id}", h.HandleGet)
	r.Get("/videos/{id}/download", h.HandleDownload)
}

// ---------- POST /videos ----------

type videoDTO struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	Status      string `json:"status"`
	FrameCount  int    `json:"frameCount,omitempty"`
	ErrorReason string `json:"errorReason,omitempty"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

func toVideoDTO(v *domain.Video) videoDTO {
	return videoDTO{
		ID: v.ID.String(), Filename: v.Filename, Status: string(v.Status),
		FrameCount: v.FrameCount, ErrorReason: v.ErrorReason,
		CreatedAt: v.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: v.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (h *Handler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(w, r)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	file, header, err := r.FormFile("video")
	if err != nil {
		writeErr(w, http.StatusBadRequest, errors.New("missing 'video' form field"))
		return
	}
	defer file.Close()

	v, err := h.upload.Execute(r.Context(), userID, httpauth.Email(r.Context()), header.Filename, file, header.Size, r.Header.Get("traceparent"))
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusAccepted, toVideoDTO(v))
}

// ---------- GET /videos ----------

func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(w, r)
	if !ok {
		return
	}
	videos, err := h.list.Execute(r.Context(), userID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	dtos := make([]videoDTO, len(videos))
	for i, v := range videos {
		dtos[i] = toVideoDTO(v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"videos": dtos, "total": len(dtos)})
}

// ---------- GET /videos/{id} ----------

func (h *Handler) HandleGet(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	v, err := h.get.Execute(r.Context(), userID, id)
	if errors.Is(err, domain.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, toVideoDTO(v))
}

// ---------- GET /videos/{id}/download ----------

func (h *Handler) HandleDownload(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	url, err := h.get.PresignDownload(r.Context(), userID, id)
	if errors.Is(err, domain.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if errors.Is(err, usecase.ErrNotReady) {
		writeErr(w, http.StatusConflict, err)
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

// ---------- helpers ----------

func authenticatedUserID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(httpauth.UserID(r.Context()))
	if err != nil {
		writeErr(w, http.StatusUnauthorized, errors.New("invalid token subject"))
		return uuid.Nil, false
	}
	return id, true
}

func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, errors.New("invalid video id"))
		return uuid.Nil, false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
