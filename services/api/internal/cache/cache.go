package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Cache is a generic key-value cache backed by the cache_entries table
// (ARCHITECTURE 5.4) -- self-hosted in Postgres rather than a separate
// service like Redis, and equally usable from any environment that can
// reach the database, unlike a local filesystem cache.
type Cache struct {
	pool *pgxpool.Pool
}

// New returns a Cache backed by pool.
func New(pool *pgxpool.Pool) *Cache {
	return &Cache{pool: pool}
}

// Get retrieves the value stored under key into dest, returning ok=false if
// the key is missing or has expired -- not an error, just a cache miss.
func (c *Cache) Get(ctx context.Context, key string, dest any) (ok bool, err error) {
	var raw []byte
	err = c.pool.QueryRow(ctx,
		"SELECT cache_value FROM cache_entries WHERE cache_key = $1 AND expires_at > now()",
		key,
	).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("cache: get %q: %w", key, err)
	}

	if err := json.Unmarshal(raw, dest); err != nil {
		return false, fmt.Errorf("cache: unmarshal %q: %w", key, err)
	}
	return true, nil
}

// Set stores value under key with the given ttl, overwriting any existing
// entry for that key.
func (c *Cache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache: marshal %q: %w", key, err)
	}

	_, err = c.pool.Exec(ctx,
		`INSERT INTO cache_entries (cache_key, cache_value, expires_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (cache_key) DO UPDATE SET cache_value = EXCLUDED.cache_value, expires_at = EXCLUDED.expires_at`,
		key, raw, time.Now().Add(ttl),
	)
	if err != nil {
		return fmt.Errorf("cache: set %q: %w", key, err)
	}
	return nil
}
