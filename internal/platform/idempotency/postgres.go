// Package idempotency contains the Video-Service-specific implementation
// of the fiapx-events idempotency.Store interface, backed by a
// PostgreSQL table.
package idempotency

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore implements events/idempotency.Store using a single
// "INSERT ... ON CONFLICT DO NOTHING" round-trip — no race window.
type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// SeenOrRecord returns (true, nil) iff the (eventId, consumer) pair was
// already in the table. The INSERT is the side-effect that records it
// for the first time. RowsAffected == 0 means another concurrent
// delivery won the race.
func (s *PostgresStore) SeenOrRecord(ctx context.Context, eventID, consumer string) (bool, error) {
	if s == nil || s.pool == nil {
		return false, fmt.Errorf("postgres store not initialized")
	}
	tag, err := s.pool.Exec(ctx,
		`INSERT INTO processed_events (event_id, consumer)
		 VALUES ($1, $2)
		 ON CONFLICT (event_id, consumer) DO NOTHING`,
		eventID, consumer)
	if err != nil {
		return false, fmt.Errorf("insert processed_events: %w", err)
	}
	return tag.RowsAffected() == 0, nil
}
