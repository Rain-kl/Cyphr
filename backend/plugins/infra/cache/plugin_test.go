// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Rain-kl/Wavelet/core"
	"github.com/Rain-kl/Wavelet/core/contracts"
	"github.com/Rain-kl/Wavelet/plugins/infra/cache"
)

func TestCachePluginOperations(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer func() { _ = rdb.Close() }()

	p := cache.New(
		cache.WithRedis(rdb),
		cache.WithKeyPrefix("app:"),
		cache.WithRAMCapacity(500),
	)
	assert.Equal(t, "cache", p.Name())

	ctx := core.NewContext(context.Background())
	require.NoError(t, p.Apply(ctx))

	svc, err := core.Inject[contracts.CacheService](ctx)
	require.NoError(t, err)

	type Data struct {
		Value string `json:"value"`
	}

	testCtx := context.Background()

	// 1. ErrCacheMiss
	var out Data
	err = svc.Get(testCtx, "missing", &out)
	assert.ErrorIs(t, err, contracts.ErrCacheMiss)

	// 2. Set & Get
	in := Data{Value: "hello"}
	require.NoError(t, svc.Set(testCtx, "key1", in, 5*time.Minute))

	require.NoError(t, svc.Get(testCtx, "key1", &out))
	assert.Equal(t, "hello", out.Value)

	// 3. GetOrSet
	var target Data
	err = svc.GetOrSet(testCtx, "key1", &target, time.Minute, func() (any, error) {
		return Data{Value: "from_loader"}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, "hello", target.Value)

	err = svc.GetOrSet(testCtx, "key2", &target, time.Minute, func() (any, error) {
		return Data{Value: "from_loader"}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, "from_loader", target.Value)

	// 4. Invalidate / Delete
	require.NoError(t, svc.Invalidate(testCtx, "key1"))
	err = svc.Get(testCtx, "key1", &out)
	assert.ErrorIs(t, err, contracts.ErrCacheMiss)

	require.NoError(t, ctx.Dispose())
}
