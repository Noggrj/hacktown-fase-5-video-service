// Package gateway has the Postgres-backed implementation of
// domain.VideoRepository and the Redis-backed implementation of
// usecase.Cache.
package gateway

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/noggrj/hacktown-fase-5-video-service/internal/video/domain"
)

type PostgresVideoRepository struct{ pool *pgxpool.Pool }

func NewPostgresVideoRepository(pool *pgxpool.Pool) *PostgresVideoRepository {
	return &PostgresVideoRepository{pool: pool}
}

const videoColumns = `id, user_id, filename, status, s3_raw_key, s3_zip_key,
	frame_count, error_reason, created_at, updated_at`

func (r *PostgresVideoRepository) Create(ctx context.Context, v *domain.Video) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO videos (`+videoColumns+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		v.ID, v.UserID, v.Filename, v.Status, v.S3RawKey, nullStr(v.S3ZipKey),
		v.FrameCount, nullStr(v.ErrorReason), v.CreatedAt, v.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert video: %w", err)
	}
	return nil
}

func (r *PostgresVideoRepository) Update(ctx context.Context, v *domain.Video) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE videos SET status=$2, s3_zip_key=$3, frame_count=$4, error_reason=$5, updated_at=$6
		 WHERE id=$1`,
		v.ID, v.Status, nullStr(v.S3ZipKey), v.FrameCount, nullStr(v.ErrorReason), v.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update video: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *PostgresVideoRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Video, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+videoColumns+` FROM videos WHERE id=$1`, id)
	return scanVideo(row)
}

func (r *PostgresVideoRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*domain.Video, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+videoColumns+` FROM videos WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("query videos: %w", err)
	}
	defer rows.Close()

	var out []*domain.Video
	for rows.Next() {
		v, err := scanVideoRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func scanVideo(row pgx.Row) (*domain.Video, error) {
	v := &domain.Video{}
	var zipKey, errorReason *string
	if err := row.Scan(
		&v.ID, &v.UserID, &v.Filename, &v.Status, &v.S3RawKey, &zipKey,
		&v.FrameCount, &errorReason, &v.CreatedAt, &v.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("scan video: %w", err)
	}
	if zipKey != nil {
		v.S3ZipKey = *zipKey
	}
	if errorReason != nil {
		v.ErrorReason = *errorReason
	}
	return v, nil
}

// rowScanner is satisfied by both pgx.Row (QueryRow) and pgx.Rows (Query).
type rowScanner interface {
	Scan(dest ...any) error
}

func scanVideoRow(row rowScanner) (*domain.Video, error) {
	return scanVideo(row)
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
