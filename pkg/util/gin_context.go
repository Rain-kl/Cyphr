// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package util

import "github.com/gin-gonic/gin"

// GetFromContext retrieves a typed value from Gin context.
func GetFromContext[T any](c *gin.Context, key string) (T, bool) {
	value, exists := c.Get(key)
	if !exists {
		var zero T
		return zero, false
	}
	typed, ok := value.(T)
	return typed, ok
}

// SetToContext sets a typed value into Gin context.
func SetToContext[T any](c *gin.Context, key string, value T) {
	c.Set(key, value)
}