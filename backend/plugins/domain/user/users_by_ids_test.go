// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user_test

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/plugins/domain/user"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	database "Wavelet/plugins/infra/database"
)

// TestGetUsersByIDsUsesSingleQuery 回归：批量取用户必须只发一条 SQL，
// 否则调用方（如访问日志按用户补全）会按 ID 逐条打库。
func TestGetUsersByIDsUsesSingleQuery(t *testing.T) {
	ctx := core.NewContext(context.Background())
	testDB := setupTestDB(t)
	require.NoError(t, database.New(database.WithDB(testDB)).Apply(ctx))
	require.NoError(t, user.New().Apply(ctx))

	userSvc, err := core.Inject[contracts.UserService](ctx)
	require.NoError(t, err)

	bg := context.Background()
	ids := make([]uint64, 0, 3)
	for _, name := range []string{"batch_a", "batch_b", "batch_c"} {
		u, err := userSvc.CreateUser(bg, contracts.CreateUserRequest{
			Username: name,
			Password: "Password789!",
			Email:    name + "@example.com",
		})
		require.NoError(t, err)
		ids = append(ids, u.ID)
	}

	queries := 0
	require.NoError(t, testDB.Callback().Query().Register("count_queries", func(_ *gorm.DB) {
		queries++
	}))

	got, err := userSvc.GetUsersByIDs(bg, ids)
	require.NoError(t, err)
	assert.Len(t, got, 3)
	assert.Equal(t, 1, queries, "batch lookup must issue exactly one query")

	queries = 0
	for _, id := range ids {
		_, err := userSvc.GetUserByID(bg, id)
		require.NoError(t, err)
	}
	assert.Equal(t, 3, queries, "per-id lookups cost one query each")

	queries = 0
	empty, err := userSvc.GetUsersByIDs(bg, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
	assert.Equal(t, 0, queries, "empty id list must not touch storage")
}
