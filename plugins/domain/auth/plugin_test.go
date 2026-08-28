// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Rain-kl/Wavelet/core"
	"github.com/Rain-kl/Wavelet/core/contracts"
	"github.com/Rain-kl/Wavelet/plugins/domain/auth"
	db "github.com/Rain-kl/Wavelet/plugins/infra/database"
)

type testUser struct {
	ID          uint64 `gorm:"primaryKey"`
	Username    string
	IsActive    bool
	LastLoginAt time.Time
}

func (testUser) TableName() string { return "w_users" }

type testAccessToken struct {
	ID        uint64 `gorm:"primaryKey"`
	UserID    uint64
	TokenHash string
	Name      string
	IsAdmin   bool
}

func (testAccessToken) TableName() string { return "w_access_tokens" }

func hashToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "auth_test.db")
	testDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, testDB.AutoMigrate(
		&testUser{},
		&testAccessToken{},
		&auth.AuthSource{},
		&auth.ExternalAccount{},
	))

	db.SetDB(testDB)
	return testDB
}

type mockProvider struct{}

func (m *mockProvider) Name() string { return "custom" }
func (m *mockProvider) GetAuthURL(state string) string {
	return "https://custom.com/auth?state=" + state
}
func (m *mockProvider) ExchangeCode(ctx context.Context, code string) (*contracts.OAuthUserInfoDTO, error) {
	return &contracts.OAuthUserInfoDTO{
		ID:       555,
		Username: "custom_user",
		Email:    "custom@example.com",
	}, nil
}

func TestAuthPluginUnit(t *testing.T) {
	ctx := core.NewContext(context.Background())
	testDB := setupTestDB(t)

	p := auth.New()
	assert.Equal(t, "auth", p.Name())
	assert.Equal(t, "1.0.0", p.Manifest().Version)
	require.NoError(t, p.Apply(ctx))

	// Test AuthService injection
	authSvc, err := core.Inject[contracts.AuthService](ctx)
	require.NoError(t, err)
	assert.NotNil(t, authSvc.RequireAuthMiddleware())
	assert.NotNil(t, authSvc.RequireAdminMiddleware())

	// Test AuthRegistry injection
	authReg, err := core.Inject[contracts.AuthRegistry](ctx)
	require.NoError(t, err)
	authReg.RegisterOAuthProvider("custom", &mockProvider{})
	prov, ok := authReg.GetOAuthProvider("custom")
	require.True(t, ok)
	assert.Equal(t, "custom", prov.Name())

	// Test User Token Verification with dummy token
	user := testUser{
		ID:       101,
		Username: "token_user",
		IsActive: true,
	}
	require.NoError(t, testDB.Create(&user).Error)

	tokenStr := "test-secret-token-123456"
	tokenHash := hashToken(tokenStr)
	tokenRecord := testAccessToken{
		ID:        201,
		UserID:    user.ID,
		TokenHash: tokenHash,
		Name:      "test-token",
		IsAdmin:   false,
	}
	require.NoError(t, testDB.Create(&tokenRecord).Error)

	userDTO, err := authSvc.VerifyToken(context.Background(), tokenStr)
	require.NoError(t, err)
	assert.Equal(t, user.ID, userDTO.ID)
	assert.Equal(t, "token_user", userDTO.Username)

	// Empty token fails
	_, err = authSvc.VerifyToken(context.Background(), "")
	assert.Error(t, err)

	// Revoke sessions
	require.NoError(t, authSvc.RevokeUserSessions(context.Background(), user.ID))

	// GetCurrentUser from context
	userCtx := context.WithValue(context.Background(), contracts.AuthUserObjKey, userDTO)
	current, err := authSvc.GetCurrentUser(userCtx)
	require.NoError(t, err)
	assert.Equal(t, user.ID, current.ID)
}
