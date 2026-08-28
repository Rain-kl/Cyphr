// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package extpoints

import (
	"slices"
	"sync"
)

func unregisterEntry[T any](mu *sync.RWMutex, lookup map[string]T, list *[]T, key string, matches func(T) bool) bool {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := lookup[key]; !exists {
		return false
	}

	delete(lookup, key)
	*list = slices.DeleteFunc(*list, matches)
	return true
}
