// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package response

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAbortNotFoundIfMissing(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		assert.False(t, AbortNotFoundIfMissing(c, nil, "gone"))
		assert.False(t, c.IsAborted())
		assert.Empty(t, c.Errors)
	})

	t.Run("record not found", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		assert.True(t, AbortNotFoundIfMissing(c, gorm.ErrRecordNotFound, "记录不存在"))
		assert.True(t, c.IsAborted())
		var apiErr *APIError
		require.True(t, errors.As(c.Errors.Last().Err, &apiErr))
		assert.Equal(t, http.StatusNotFound, apiErr.Code)
		assert.Equal(t, "记录不存在", apiErr.Msg)
	})

	t.Run("other error", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		assert.True(t, AbortNotFoundIfMissing(c, errors.New("boom"), "记录不存在"))
		assert.True(t, c.IsAborted())
		var apiErr *APIError
		require.True(t, errors.As(c.Errors.Last().Err, &apiErr))
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
		assert.Equal(t, "boom", apiErr.Msg)
	})
}

func TestAbortBadRequestOnError(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		assert.False(t, AbortBadRequestOnError(c, nil))
		assert.False(t, c.IsAborted())
	})

	t.Run("error", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		assert.True(t, AbortBadRequestOnError(c, errors.New("bad")))
		assert.True(t, c.IsAborted())
		var apiErr *APIError
		require.True(t, errors.As(c.Errors.Last().Err, &apiErr))
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
		assert.Equal(t, "bad", apiErr.Msg)
	})
}
