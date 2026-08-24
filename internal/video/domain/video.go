// Package domain has the Video aggregate, its status machine and the
// repository interface. No knowledge of HTTP, S3, Kafka or SQL.
package domain

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("not found")

// ErrForbidden signals the authenticated user does not own the video
// being accessed — the handler must return 404, not 403, so a video ID
// can't be used to probe for other users' videos.
var ErrForbidden = errors.New("forbidden")

type Status string

const (
	StatusPending    Status = "PENDING"
	StatusProcessing Status = "PROCESSING"
	StatusDone       Status = "DONE"
	StatusFailed     Status = "FAILED"
)

var validExtensions = map[string]bool{
	".mp4": true, ".avi": true, ".mov": true, ".mkv": true,
	".wmv": true, ".flv": true, ".webm": true,
}

// IsValidExtension mirrors the format allow-list from the original
// FIAP X proof of concept, so behavior doesn't regress for users.
func IsValidExtension(filename string) bool {
	return validExtensions[strings.ToLower(filepath.Ext(filename))]
}

// Video is the aggregate root for the Video domain.
type Video struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Filename    string
	Status      Status
	S3RawKey    string
	S3ZipKey    string
	FrameCount  int
	ErrorReason string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewVideo creates a PENDING video ready to be persisted right after the
// raw file finishes uploading to S3 at s3RawKey.
func NewVideo(userID uuid.UUID, filename, s3RawKey string) (*Video, error) {
	if userID == uuid.Nil {
		return nil, errors.New("userId is required")
	}
	if !IsValidExtension(filename) {
		return nil, fmt.Errorf("unsupported video format: %s", filepath.Ext(filename))
	}
	if s3RawKey == "" {
		return nil, errors.New("s3RawKey is required")
	}
	now := time.Now().UTC()
	return &Video{
		ID:        uuid.New(),
		UserID:    userID,
		Filename:  filename,
		Status:    StatusPending,
		S3RawKey:  s3RawKey,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// RawKey builds the deterministic S3 key for a not-yet-created video.
func RawKey(videoID uuid.UUID, filename string) string {
	return fmt.Sprintf("raw/%s/%s", videoID, filename)
}

// ZipKey builds the deterministic S3 key for the processed frames zip.
func ZipKey(videoID uuid.UUID) string {
	return fmt.Sprintf("processed/%s.zip", videoID)
}

func (v *Video) MarkProcessed(s3ZipKey string, frameCount int, at time.Time) {
	v.Status = StatusDone
	v.S3ZipKey = s3ZipKey
	v.FrameCount = frameCount
	v.UpdatedAt = at
}

func (v *Video) MarkFailed(reason string, at time.Time) {
	v.Status = StatusFailed
	v.ErrorReason = reason
	v.UpdatedAt = at
}
