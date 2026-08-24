-- fiapx-video-service: videos table
CREATE TABLE IF NOT EXISTS videos (
    id            UUID PRIMARY KEY,
    user_id       UUID NOT NULL,
    filename      VARCHAR(255) NOT NULL,
    status        VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    s3_raw_key    VARCHAR(512) NOT NULL,
    s3_zip_key    VARCHAR(512),
    frame_count   INTEGER NOT NULL DEFAULT 0,
    error_reason  TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_videos_user_id ON videos (user_id, created_at DESC);
