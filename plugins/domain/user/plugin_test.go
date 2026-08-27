// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Rain-kl/Wavelet/core"
	"github.com/Rain-kl/Wavelet/core/contracts"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/plugins/domain/user"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "user_test.db")
	testDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, testDB.AutoMigrate(
		&model.User{},
		&model.AccessToken{},
	))

	db.SetDB(testDB)
	return testDB
}

func TestUserPluginUnit(t *testing.T) {
	ctx := core.NewContext(context.Background())
	_ = setupTestDB(t)

	p := user.New()
	assert.Equal(t, "user", p.Name())
	assert.Equal(t, "1.0.0", p.Manifest().Version)
	require.NoError(t, p.Apply(ctx))

	userSvc, err := core.Inject[contracts.UserService](ctx)
	require.NoError(t, err)
	require.NotNil(t, userSvc)

	testCtx := context.Background()

	// 1. Create User
	u, err := userSvc.CreateUser(testCtx, contracts.CreateUserRequest{
		Username: "charlie",
		Password: "Password789!",
		Email:    "charlie@example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, "charlie", u.Username)

	// 2. Empty username error
	_, err = userSvc.CreateUser(testCtx, contracts.CreateUserRequest{})
	assert.Error(t, err)

	// 3. Verify Password
	assert.True(t, userSvc.VerifyPassword(testCtx, u.ID, "Password789!"))
	assert.False(t, userSvc.VerifyPassword(testCtx, u.ID, "Wrong"))

	// 4. Update Password with wrong old password
	err = userSvc.UpdatePassword(testCtx, u.ID, "WrongOld", "NewPass999!")
	assert.Error(t, err)

	// Update Password success
	err = userSvc.UpdatePassword(testCtx, u.ID, "Password789!", "NewPass999!")
	require.NoError(t, err)
	assert.True(t, userSvc.VerifyPassword(testCtx, u.ID, "NewPass999!"))

	// 5. Update Profile
	nickname := "Charlie Brown"
	email := "charlie.new@example.com"
	gender := "male"
	website := "https://charlie.me"
	loc := "SF"
	updated, err := userSvc.UpdateProfile(testCtx, u.ID, contracts.UpdateUserProfileRequest{
		Nickname: &nickname,
		Email:    &email,
		Gender:   &gender,
		Website:  &website,
		Location: &loc,
	})
	require.NoError(t, err)
	assert.Equal(t, "Charlie Brown", updated.Nickname)
	assert.Equal(t, "charlie.new@example.com", updated.Email)
	assert.Equal(t, "male", updated.Gender)
	assert.Equal(t, "https://charlie.me", updated.Website)
	assert.Equal(t, "SF", updated.Location)

	// 6. List and Status
	require.NoError(t, userSvc.SetUserAdmin(testCtx, u.ID, true))
	require.NoError(t, userSvc.SetUserActive(testCtx, u.ID, true))

	list, total, err := userSvc.ListUsers(testCtx, 1, 10, "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	assert.NotEmpty(t, list)
}
