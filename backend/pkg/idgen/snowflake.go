// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package idgen 提供分布式 ID 生成器
package idgen

import (
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/bwmarrin/snowflake"
)

// 2025-12-01 00:00:00 UTC 的毫秒时间戳
const epoch int64 = 1764547200000

const maxNegativeIDRetries = 3

var (
	mu   sync.RWMutex
	node *snowflake.Node
)

// Init initializes the snowflake ID generator with the given node ID.
func Init(nodeID int64) error {
	snowflake.Epoch = epoch

	n, err := snowflake.NewNode(nodeID)
	if err != nil {
		return fmt.Errorf("idgen: init node %d failed: %w", nodeID, err)
	}

	mu.Lock()
	node = n
	mu.Unlock()

	log.Printf("[Snowflake] initialized with node ID: %d, epoch: 2025-12-01\n", nodeID)
	return nil
}

// ErrNotInitialized indicates NextUint64ID was called before Init.
var ErrNotInitialized = errors.New("idgen: Init must be called before generating IDs")

// NextUint64ID 生成下一个分布式唯一 ID。
// 理论上不应出现负值；若出现则最多重试 maxNegativeIDRetries 次，仍失败则 panic。
func NextUint64ID() uint64 {
	mu.RLock()
	n := node
	mu.RUnlock()

	if n == nil {
		panic(ErrNotInitialized)
	}

	for attempt := 1; attempt <= maxNegativeIDRetries; attempt++ {
		id := n.Generate().Int64()
		if id >= 0 {
			return uint64(id)
		}
		log.Printf("[Snowflake] generated negative ID: %d (attempt %d/%d)", id, attempt, maxNegativeIDRetries)
	}
	panic(fmt.Sprintf("[Snowflake] generated negative ID after %d attempts", maxNegativeIDRetries))
}
