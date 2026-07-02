package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Postgres wraps a pgxpool connection pool.
type Postgres struct {
	Pool *pgxpool.Pool
}

// NewPostgres creates a new PostgreSQL connection pool.
func NewPostgres(databaseURL string) (*Postgres, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing database URL: %w", err)
	}

	cfg.MaxConns = 25
	cfg.MinConns = 5
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return &Postgres{Pool: pool}, nil
}

// Close closes the connection pool.
func (p *Postgres) Close() {
	if p.Pool != nil {
		p.Pool.Close()
	}
}

// SetTenantContext sets the tenant ID on the database connection for RLS.
// This MUST be called at the start of every request that accesses tenant data.
func (p *Postgres) SetTenantContext(ctx context.Context, tenantID string) error {
	_, err := p.Pool.Exec(ctx,
		"SELECT set_config('app.current_tenant_id', $1, true)",
		tenantID,
	)
	if err != nil {
		return fmt.Errorf("setting tenant context: %w", err)
	}
	return nil
}

// Redis wraps a go-redis client.
type Redis struct {
	Client *redis.Client
	prefix string
}

// NewRedis creates a new Redis client.
func NewRedis(redisURL string) (*Redis, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parsing Redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("pinging Redis: %w", err)
	}

	return &Redis{
		Client: client,
		prefix: "clientroll:",
	}, nil
}

// Close closes the Redis client.
func (r *Redis) Close() error {
	if r.Client != nil {
		return r.Client.Close()
	}
	return nil
}

// Key returns a namespaced Redis key to avoid collisions with other apps.
func (r *Redis) Key(parts ...string) string {
	key := r.prefix
	for i, part := range parts {
		if i > 0 {
			key += ":"
		}
		key += part
	}
	return key
}

// Set stores a value with a TTL.
func (r *Redis) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return r.Client.Set(ctx, r.Key(key), value, ttl).Err()
}

// Get retrieves a value by key.
func (r *Redis) Get(ctx context.Context, key string) (string, error) {
	return r.Client.Get(ctx, r.Key(key)).Result()
}

// Del deletes a key.
func (r *Redis) Del(ctx context.Context, keys ...string) error {
	namespacedKeys := make([]string, len(keys))
	for i, k := range keys {
		namespacedKeys[i] = r.Key(k)
	}
	return r.Client.Del(ctx, namespacedKeys...).Err()
}
