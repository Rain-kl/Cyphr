// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newTestUserAccessStore(t *testing.T) *userAccessLogGormStore {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open("file:logstore-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(&UserAccessLog{}))
	return newUserAccessLogGormStore(gdb)
}

func TestGormUserAccessLogCountList(t *testing.T) {
	ua := newTestUserAccessStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, ua.BatchInsert(ctx, []UserAccessLog{
		{UserID: 10, Path: "/api/v1/users", Method: "GET", Status: 200, CreatedAt: now},
		{UserID: 20, Path: "/api/v1/admin", Method: "GET", Status: 200, CreatedAt: now},
		{UserID: 10, Path: "/api/v1/other", Method: "POST", Status: 201, CreatedAt: now},
	}))

	count, err := ua.Count(ctx, AccessLogFilter{UserIDs: []uint64{10}, Path: "users"})
	require.NoError(t, err)
	require.Equal(t, uint64(1), count)

	rows, total, err := ua.List(ctx, AccessLogFilter{UserIDs: []uint64{10}, Path: "users"}, 1, 10)
	require.NoError(t, err)
	require.Equal(t, uint64(1), total)
	require.Len(t, rows, 1)
	require.Equal(t, "/api/v1/users", rows[0].Path)
	require.NotZero(t, rows[0].ID)
}

func TestGormUserAccessLogFreeze(t *testing.T) {
	ua := newTestUserAccessStore(t)
	SetConfigReader(func(_ context.Context, key string) (string, error) {
		if key == logMigrationKey {
			return "migrating", nil
		}
		return "", nil
	})
	t.Cleanup(ResetForTest)

	err := ua.BatchInsert(context.Background(), []UserAccessLog{{UserID: 1, CreatedAt: time.Now()}})
	require.ErrorIs(t, err, ErrMigrating)
}
