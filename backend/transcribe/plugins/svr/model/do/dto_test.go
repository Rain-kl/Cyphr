// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package do_test

import (
	"Wavelet/transcribe/plugins/svr/model/do"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJobDTO_JSONStringSerialization(t *testing.T) {
	var nodeID uint64 = 100192547116158976
	dto := do.JobDTO{
		ID:     100196806192795648,
		UserID: 100196806192795000,
		NodeID: &nodeID,
		Model:  "mock-whisper-base",
		Status: "completed",
	}

	data, err := json.Marshal(dto)
	require.NoError(t, err)

	jsonStr := string(data)
	// ID must be string serialized to prevent JS float64 precision truncation
	require.True(t, strings.Contains(jsonStr, `"id":"100196806192795648"`), "expected string serialized id, got: %s", jsonStr)
	require.True(t, strings.Contains(jsonStr, `"user_id":"100196806192795000"`), "expected string serialized user_id, got: %s", jsonStr)
	require.True(t, strings.Contains(jsonStr, `"node_id":"100192547116158976"`), "expected string serialized node_id, got: %s", jsonStr)

	// Verify unmarshaling back from string
	var decoded do.JobDTO
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, uint64(100196806192795648), decoded.ID)
	require.Equal(t, uint64(100196806192795000), decoded.UserID)
	require.NotNil(t, decoded.NodeID)
	require.Equal(t, uint64(100192547116158976), *decoded.NodeID)
}
