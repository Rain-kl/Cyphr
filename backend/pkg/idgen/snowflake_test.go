// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package idgen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNextUint64ID(t *testing.T) {
	require.NoError(t, Init(1))
	id := NextUint64ID()
	assert.NotZero(t, id)
}

func TestNextUint64ID_PanicsWhenNotInitialized(t *testing.T) {
	mu.Lock()
	node = nil
	mu.Unlock()

	assert.PanicsWithError(t, ErrNotInitialized.Error(), func() {
		NextUint64ID()
	})

	// Restore initialization for other tests
	require.NoError(t, Init(1))
}
