package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool wraps a pgxpool.Pool exposing a Healthy() check for /ready. The
// pool is created lazily-tolerant: failure on startup logs and keeps the
// app alive, /ready stays degraded until the database is reachable.
type Pool struct {
	*pgxpool.Pool
}

func NewPool(ctx context.Context, dbURL string) (*Pool, error) {
	if dbURL == "" {
		return nil, fmt.Errorf("DB_URL is empty")
	}
	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("parse pg config: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Pool{Pool: pool}, nil
}

func (p *Pool) Healthy(ctx context.Context) error {
	if p == nil || p.Pool == nil {
		return fmt.Errorf("pool not initialized")
	}
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return p.Ping(c)
}
