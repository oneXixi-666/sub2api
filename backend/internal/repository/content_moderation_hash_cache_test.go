package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestContentModerationHashCacheAcquireEventDedupe(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := &contentModerationHashCache{rdb: client}

	acquired, err := cache.AcquireEventDedupe(context.Background(), "fingerprint", time.Minute)
	require.NoError(t, err)
	require.True(t, acquired)

	acquired, err = cache.AcquireEventDedupe(context.Background(), "fingerprint", time.Minute)
	require.NoError(t, err)
	require.False(t, acquired)

	server.FastForward(time.Minute)
	acquired, err = cache.AcquireEventDedupe(context.Background(), "fingerprint", time.Minute)
	require.NoError(t, err)
	require.True(t, acquired)
}
