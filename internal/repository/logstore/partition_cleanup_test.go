// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"testing"
	"time"

	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/stretchr/testify/require"
)

func TestPartitionNameMonth(t *testing.T) {
	cases := []struct {
		table string
		name  string
		want  string
	}{
		{"w_user_access_logs", "w_user_access_logs_202612", "2026-12"},
		{"w_user_access_logs", "w_user_access_logs_202608", "2026-08"},
		{"w_user_access_logs", "of_node_access_logs_202608", ""},
		{"w_user_access_logs", "w_user_access_logs_20268", ""},
		{"w_user_access_logs", "w_user_access_logs_202613", ""},
		{"w_user_access_logs", "w_user_access_logs_default", ""},
	}
	for _, c := range cases {
		got, ok := partitionNameMonth(c.table, c.name)
		if c.want == "" {
			if ok {
				t.Fatalf("partitionNameMonth(%q, %q) ok = true, want false", c.table, c.name)
			}
			continue
		}
		if !ok || got.Format("2006-01") != c.want {
			t.Fatalf("partitionNameMonth(%q, %q) = %v, want %s", c.table, c.name, got, c.want)
		}
	}
}

func TestDropEligiblePartitionNames(t *testing.T) {
	before := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	names := []string{
		"w_user_access_logs_202608",
		"w_user_access_logs_202609",
		"w_user_access_logs_202610",
		"w_user_access_logs_202611",
		"w_user_access_logs_default",
	}
	got := dropEligiblePartitionNames(userAccessLogTable, names, before)
	want := []string{"w_user_access_logs_202608", "w_user_access_logs_202609"}
	require.Equal(t, want, got)

	first := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	require.Empty(t, dropEligiblePartitionNames(userAccessLogTable, []string{"w_user_access_logs_202610"}, first))
}

func TestDropPartitionHelpersSQLiteNoop(t *testing.T) {
	ua := newTestUserAccessStore(t)
	ctx := context.Background()
	require.NoError(t, ua.BatchInsert(ctx, []analyticsmodel.UserAccessLog{
		{UserID: 1, Path: "/x", CreatedAt: time.Now().UTC()},
	}))
	require.NoError(t, ua.DropExpiredPartitions(ctx, time.Now().AddDate(0, 0, -90)))
	require.NoError(t, ua.DropEmptyPartitions(ctx, time.Now()))
	count, err := ua.Count(ctx, AccessLogFilter{})
	require.NoError(t, err)
	require.Equal(t, uint64(1), count)
}
