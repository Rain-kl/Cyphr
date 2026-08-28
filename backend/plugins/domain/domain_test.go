// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"context"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Rain-kl/Wavelet/backend/core"
	"github.com/Rain-kl/Wavelet/backend/core/contracts"
	"github.com/Rain-kl/Wavelet/backend/plugins/domain/admin"
	"github.com/Rain-kl/Wavelet/backend/plugins/domain/auth"
	"github.com/Rain-kl/Wavelet/backend/plugins/domain/message_gateway"
	"github.com/Rain-kl/Wavelet/backend/plugins/domain/risk_control"
	"github.com/Rain-kl/Wavelet/backend/plugins/domain/user"
	"github.com/Rain-kl/Wavelet/backend/plugins/infra/cache"
	db "github.com/Rain-kl/Wavelet/backend/plugins/infra/database"
	"github.com/Rain-kl/Wavelet/backend/plugins/infra/logger"
	"github.com/Rain-kl/Wavelet/backend/plugins/infra/storage"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "domain_test.db")
	testDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, testDB.AutoMigrate(
		&user.User{},
		&user.AccessToken{},
		&auth.AuthSource{},
		&auth.ExternalAccount{},
		&message_gateway.MessageChannel{},
		&message_gateway.MessageBinding{},
		&message_gateway.MessagePairingCode{},
		&admin.SystemConfig{},
		&message_gateway.PushChannel{},
		&message_gateway.PushEvent{},
		&message_gateway.PushHistory{},
	))

	db.SetDB(testDB)
	return testDB
}

type mockOAuthProvider struct {
	name string
}

func (m *mockOAuthProvider) Name() string {
	return m.name
}

func (m *mockOAuthProvider) GetAuthURL(state string) string {
	return "https://auth.example.com/auth?state=" + state
}

func (m *mockOAuthProvider) ExchangeCode(ctx context.Context, code string) (*contracts.OAuthUserInfoDTO, error) {
	return &contracts.OAuthUserInfoDTO{
		ID:       1001,
		Username: "mock_user",
		Email:    "mock@example.com",
		Active:   true,
	}, nil
}

func TestAuthPlugin(t *testing.T) {
	ctx := core.NewContext(context.Background())
	testDB := setupTestDB(t)

	require.NoError(t, db.New(db.WithDB(testDB)).Apply(ctx))
	require.NoError(t, cache.New().Apply(ctx))
	require.NoError(t, logger.New().Apply(ctx))

	p := auth.New()
	assert.Equal(t, "auth", p.Name())
	assert.Equal(t, "auth", p.Manifest().Name)
	require.NoError(t, p.Apply(ctx))

	// 1. Verify migrations registered
	entry, ok := ctx.Migrations().Get("auth")
	require.True(t, ok)
	assert.Equal(t, "auth", entry.PluginID)
	entries, err := fs.ReadDir(entry.FS, entry.Dir)
	require.NoError(t, err)
	assert.NotEmpty(t, entries)

	// 2. Verify AuthService
	authSvc, err := core.Inject[contracts.AuthService](ctx)
	require.NoError(t, err)
	require.NotNil(t, authSvc)
	assert.NotNil(t, authSvc.RequireAuthMiddleware())
	assert.NotNil(t, authSvc.RequireAdminMiddleware())

	// 3. Verify AuthRegistry
	authReg, err := core.Inject[contracts.AuthRegistry](ctx)
	require.NoError(t, err)
	require.NotNil(t, authReg)

	mockProv := &mockOAuthProvider{name: "github"}
	authReg.RegisterOAuthProvider("github", mockProv)
	retrieved, ok := authReg.GetOAuthProvider("github")
	require.True(t, ok)
	assert.Equal(t, "github", retrieved.Name())
	assert.Contains(t, authReg.ListOAuthProviders(), "github")

	// 4. Verify Routes
	routes := ctx.Router().Routes()
	var hasSources, hasLogin, hasUserInfo bool
	for _, r := range routes {
		if r.Path == "/api/v1/oauth/sources" {
			hasSources = true
		}
		if r.Path == "/api/v1/oauth/login" {
			hasLogin = true
		}
		if r.Path == "/api/v1/user-info" {
			hasUserInfo = true
		}
	}
	assert.True(t, hasSources)
	assert.True(t, hasLogin)
	assert.True(t, hasUserInfo)

	// 5. Verify Settings
	schema, ok := ctx.Settings().Get("auth.session_age")
	require.True(t, ok)
	assert.Equal(t, 86400*7, schema.Default)
}

func TestUserPlugin(t *testing.T) {
	ctx := core.NewContext(context.Background())
	testDB := setupTestDB(t)

	require.NoError(t, db.New(db.WithDB(testDB)).Apply(ctx))
	require.NoError(t, cache.New().Apply(ctx))
	require.NoError(t, logger.New().Apply(ctx))

	p := user.New()
	assert.Equal(t, "user", p.Name())
	assert.Equal(t, "user", p.Manifest().Name)
	require.NoError(t, p.Apply(ctx))

	// 1. Verify migrations
	entry, ok := ctx.Migrations().Get("user")
	require.True(t, ok)
	assert.Equal(t, "user", entry.PluginID)

	// 2. Verify UserService
	userSvc, err := core.Inject[contracts.UserService](ctx)
	require.NoError(t, err)
	require.NotNil(t, userSvc)

	testCtx := context.Background()

	// 3. Create user
	created, err := userSvc.CreateUser(testCtx, contracts.CreateUserRequest{
		Username: "bob",
		Password: "SecurePassword123!",
		Nickname: "Bob Builder",
		Email:    "bob@example.com",
		IsAdmin:  false,
	})
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "bob", created.Username)
	assert.Equal(t, "Bob Builder", created.Nickname)
	assert.Equal(t, "bob@example.com", created.Email)
	assert.False(t, created.IsAdmin)

	// 4. Query user
	byID, err := userSvc.GetUserByID(testCtx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "bob", byID.Username)

	byUsername, err := userSvc.GetUserByUsername(testCtx, "bob")
	require.NoError(t, err)
	assert.Equal(t, created.ID, byUsername.ID)

	byEmail, err := userSvc.GetUserByEmail(testCtx, "bob@example.com")
	require.NoError(t, err)
	assert.Equal(t, created.ID, byEmail.ID)

	// 5. Password verification and update
	assert.True(t, userSvc.VerifyPassword(testCtx, created.ID, "SecurePassword123!"))
	assert.False(t, userSvc.VerifyPassword(testCtx, created.ID, "WrongPass"))

	require.NoError(t, userSvc.UpdatePassword(testCtx, created.ID, "SecurePassword123!", "NewSecurePassword456!"))
	assert.True(t, userSvc.VerifyPassword(testCtx, created.ID, "NewSecurePassword456!"))

	// 6. Update Profile
	newBio := "I build things"
	newPhone := "13800138000"
	updated, err := userSvc.UpdateProfile(testCtx, created.ID, contracts.UpdateUserProfileRequest{
		Bio:   &newBio,
		Phone: &newPhone,
	})
	require.NoError(t, err)
	assert.Equal(t, newBio, updated.Bio)
	assert.Equal(t, newPhone, updated.Phone)

	// 7. Update Last Login
	require.NoError(t, userSvc.UpdateLastLogin(testCtx, created.ID, "127.0.0.1"))

	// 8. Admin operations: SetUserActive, SetUserAdmin, ListUsers
	require.NoError(t, userSvc.SetUserAdmin(testCtx, created.ID, true))
	reloaded, err := userSvc.GetUserByID(testCtx, created.ID)
	require.NoError(t, err)
	assert.True(t, reloaded.IsAdmin)

	require.NoError(t, userSvc.SetUserActive(testCtx, created.ID, false))
	reloadedBanned, err := userSvc.GetUserByID(testCtx, created.ID)
	require.NoError(t, err)
	assert.False(t, reloadedBanned.IsActive)

	list, total, err := userSvc.ListUsers(testCtx, 1, 10, "bob")
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, list, 1)
	assert.Equal(t, "bob", list[0].Username)

	// 9. Tasks & Schedules
	taskDef, ok := ctx.Tasks().Get("user:send_email_code")
	require.True(t, ok)
	assert.Equal(t, 3, taskDef.Retry)

	schedDef, ok := ctx.Schedules().Get("user:daily_audit")
	require.True(t, ok)
	assert.Equal(t, "0 3 * * *", schedDef.Spec)

	// 10. Settings
	sReg, ok := ctx.Settings().Get("user.registration_enabled")
	require.True(t, ok)
	assert.Equal(t, true, sReg.Default)
}

func TestMessageGatewayPlugin(t *testing.T) {
	ctx := core.NewContext(context.Background())
	testDB := setupTestDB(t)

	require.NoError(t, db.New(db.WithDB(testDB)).Apply(ctx))
	require.NoError(t, cache.New().Apply(ctx))
	require.NoError(t, logger.New().Apply(ctx))

	p := message_gateway.New()
	assert.Equal(t, "message_gateway", p.Name())
	assert.Equal(t, "message_gateway", p.Manifest().Name)
	require.NoError(t, p.Apply(ctx))

	// 1. Migrations
	entry, ok := ctx.Migrations().Get("message_gateway")
	require.True(t, ok)
	assert.Equal(t, "message_gateway", entry.PluginID)

	// 2. Routes
	routes := ctx.Router().Routes()
	var hasChannels, hasBindings bool
	for _, r := range routes {
		if r.Path == "/api/v1/message-gateway/channels" {
			hasChannels = true
		}
		if r.Path == "/api/v1/message-gateway/bindings" {
			hasBindings = true
		}
	}
	assert.True(t, hasChannels)
	assert.True(t, hasBindings)

	// 3. Tasks & Schedules
	taskDef, ok := ctx.Tasks().Get("message_gateway:push_notification")
	require.True(t, ok)
	assert.Equal(t, 3, taskDef.Retry)

	schedDef, ok := ctx.Schedules().Get("message_gateway:cleanup_pairing_codes")
	require.True(t, ok)
	assert.Equal(t, "*/10 * * * *", schedDef.Spec)

	// 4. EventBus Trigger
	var receivedEvent message_gateway.PushNotificationEvent
	var eventFired bool
	ctx.Events().On("notification:push", func(c context.Context, e message_gateway.PushNotificationEvent) error {
		eventFired = true
		receivedEvent = e
		return nil
	})

	err := ctx.Events().Emit(context.Background(), "notification:push", message_gateway.PushNotificationEvent{
		UserID:  99,
		Channel: "telegram",
		Title:   "System Alert",
		Content: "Disk 85% full",
	})
	require.NoError(t, err)
	assert.True(t, eventFired)
	assert.Equal(t, uint64(99), receivedEvent.UserID)
	assert.Equal(t, "telegram", receivedEvent.Channel)
	assert.Equal(t, "System Alert", receivedEvent.Title)

	// 5. Settings
	schema, ok := ctx.Settings().Get("message_gateway.pairing_code_expiry_minutes")
	require.True(t, ok)
	assert.Equal(t, 15, schema.Default)
}

func TestRiskControlPlugin(t *testing.T) {
	ctx := core.NewContext(context.Background())
	p := risk_control.New()
	assert.Equal(t, "risk_control", p.Name())
	assert.Equal(t, "risk_control", p.Manifest().Name)
	require.NoError(t, p.Apply(ctx))

	// 1. Middleware registered on Router
	mws := ctx.Router().Middlewares()
	assert.NotEmpty(t, mws)

	// 2. Settings
	schema, ok := ctx.Settings().Get("risk_control.ip_rate_limit_per_minute")
	require.True(t, ok)
	assert.Equal(t, 60, schema.Default)

	// 3. Disposal cleanup
	require.NoError(t, ctx.Dispose())
}

func TestAdminPlugin(t *testing.T) {
	ctx := core.NewContext(context.Background())
	testDB := setupTestDB(t)

	require.NoError(t, db.New(db.WithDB(testDB)).Apply(ctx))
	require.NoError(t, cache.New().Apply(ctx))
	require.NoError(t, logger.New().Apply(ctx))

	p := admin.New()
	assert.Equal(t, "admin", p.Name())
	assert.Equal(t, "admin", p.Manifest().Name)
	require.NoError(t, p.Apply(ctx))

	// 1. Admin Routes
	routes := ctx.Router().Routes()
	var hasStatus, hasDBOverview, hasUsers, hasTasks, hasConfigs bool
	for _, r := range routes {
		if r.Path == "/api/v1/admin/status" {
			hasStatus = true
		}
		if r.Path == "/api/v1/admin/db-manage/overview" {
			hasDBOverview = true
		}
		if r.Path == "/api/v1/admin/users" {
			hasUsers = true
		}
		if r.Path == "/api/v1/admin/tasks/types" {
			hasTasks = true
		}
		if r.Path == "/api/v1/admin/system-configs" {
			hasConfigs = true
		}
	}
	assert.True(t, hasStatus)
	assert.True(t, hasDBOverview)
	assert.True(t, hasUsers)
	assert.True(t, hasTasks)
	assert.True(t, hasConfigs)

	// 2. Task & Schedule
	_, ok := ctx.Tasks().Get("admin:system_cleanup")
	require.True(t, ok)
	sched, ok := ctx.Schedules().Get("admin:system_cleanup")
	require.True(t, ok)
	assert.Equal(t, "0 4 * * *", sched.Spec)

	// 3. Settings
	schema, ok := ctx.Settings().Get("admin.system_cleanup_cron")
	require.True(t, ok)
	assert.Equal(t, "0 4 * * *", schema.Default)
}

func TestAllDomainPluginsCombined(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := core.NewContext(context.Background())
	testDB := setupTestDB(t)

	// Apply Infra plugins
	require.NoError(t, db.New(db.WithDB(testDB)).Apply(ctx))
	require.NoError(t, cache.New(cache.WithRedis(rdb)).Apply(ctx))
	require.NoError(t, logger.New().Apply(ctx))
	require.NoError(t, storage.New().Apply(ctx))

	// Apply Domain plugins
	require.NoError(t, auth.New().Apply(ctx))
	require.NoError(t, user.New().Apply(ctx))
	require.NoError(t, message_gateway.New().Apply(ctx))
	require.NoError(t, risk_control.New().Apply(ctx))
	require.NoError(t, admin.New().Apply(ctx))

	// Verify cross-plugin service injection via Using3
	var resolved bool
	err = core.Using3(ctx, func(authSvc contracts.AuthService, userSvc contracts.UserService, authReg contracts.AuthRegistry) {
		resolved = true
		assert.NotNil(t, authSvc)
		assert.NotNil(t, userSvc)
		assert.NotNil(t, authReg)
	})
	require.NoError(t, err)
	assert.True(t, resolved)

	// Verify all migration entries
	allMigrations := ctx.Migrations().Entries()
	assert.GreaterOrEqual(t, len(allMigrations), 3)

	// Verify total routes registered
	allRoutes := ctx.Router().Routes()
	assert.GreaterOrEqual(t, len(allRoutes), 20)

	// Verify total tasks registered
	allTasks := ctx.Tasks().Tasks()
	assert.GreaterOrEqual(t, len(allTasks), 4)

	// Verify total schedules registered
	allSchedules := ctx.Schedules().Schedules()
	assert.GreaterOrEqual(t, len(allSchedules), 3)

	// Verify total settings schemas registered
	allSettings := ctx.Settings().Schemas()
	assert.GreaterOrEqual(t, len(allSettings), 7)

	// Clean shutdown
	require.NoError(t, ctx.Dispose())
}
