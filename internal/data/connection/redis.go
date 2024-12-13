package connection

import (
	"context"
	"fmt"
	"wameter/internal/data/config"

	"github.com/redis/go-redis/v9"
)

// newRedis creates new Redis client
func newRedis(cfg *config.Redis) (*redis.Client, error) {
	if cfg == nil || cfg.Addr == "" {
		return nil, fmt.Errorf("redis configuration is nil or empty")
	}

	rc := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.Db,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		DialTimeout:  cfg.DialTimeout,
		PoolSize:     10,
	})

	timeout, cancelFunc := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancelFunc()
	if err := rc.Ping(timeout).Err(); err != nil {
		return nil, fmt.Errorf("redis connect error: %w", err)
	}

	return rc, nil
}
