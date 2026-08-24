package gateway

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache implements usecase.Cache. Every method swallows Redis
// errors (logged upstream would be nicer, but the usecase already
// treats a miss and an error identically) so an outage degrades to
// "always fall back to Postgres" instead of failing requests.
type RedisCache struct{ client *redis.Client }

func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

func (c *RedisCache) GetList(ctx context.Context, userID string) ([]byte, bool) {
	val, err := c.client.Get(ctx, cacheKey(userID)).Bytes()
	if err != nil {
		return nil, false
	}
	return val, true
}

func (c *RedisCache) SetList(ctx context.Context, userID string, data []byte, ttl time.Duration) {
	_ = c.client.Set(ctx, cacheKey(userID), data, ttl).Err()
}

func (c *RedisCache) Invalidate(ctx context.Context, userID string) {
	_ = c.client.Del(ctx, cacheKey(userID)).Err()
}

func cacheKey(userID string) string { return "video-status:" + userID }
