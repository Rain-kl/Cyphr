// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package user provides the user profile, credential management, role management, and access token domain plugin for Cordis.
package user

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/core/extpoints"
	"Wavelet/pkg/ginutil"
	"context"
	"embed"
	"reflect"

	"github.com/gin-gonic/gin"
)

//go:embed migrations/*/*.sql
var userMigrations embed.FS

// Option configures the user plugin.
type Option func(*Plugin)

// WithUserService sets a custom UserService implementation.
func WithUserService(svc contracts.UserService) Option {
	return func(p *Plugin) {
		p.userSvc = svc
	}
}

// Plugin implements core.Plugin to provide user account and credential domain services.
type Plugin struct {
	userSvc contracts.UserService
}

// New creates a new user domain plugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p
}

// PluginName 用户插件唯一名称标识
const PluginName = "user"

// Name returns the unique identifier for the user domain plugin.
func (p *Plugin) Name() string {
	return PluginName
}

// Inject declares required dependencies for the user domain plugin.
func (p *Plugin) Inject() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[contracts.DBService](),
		// Apply resolves AuthService to build the route auth middleware. The
		// kernel only gates Apply on declared deps, so leaving this out lets
		// user mount before auth and fall back to a pass-through middleware.
		reflect.TypeFor[contracts.AuthService](),
	}
}

// Manifest returns the plugin metadata.
func (p *Plugin) Manifest() core.Manifest {
	return core.Manifest{
		Name:        PluginName,
		Version:     "1.0.0",
		Description: "User profiles, credentials, role management, and access token domain plugin",
		Author:      "Wavelet Team",
	}
}

// Apply registers user migrations, services, routes, tasks, schedules, and settings into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	// 0. Bind DBService from Context
	if db, err := core.Inject[contracts.DBService](ctx); err == nil && db != nil {
		SetDBService(db)
	} else {
		core.When[contracts.DBService](ctx, func(db contracts.DBService) {
			SetDBService(db)
		})
	}
	ctx.OnDispose(func() error {
		SetDBService(nil)
		return nil
	})

	// 0.1 Resolve auth service for middleware and cache invalidation (via IoC, not direct import)
	denyAuth := ginutil.AuthUnavailable()
	loginMW := denyAuth
	noTokenMW := denyAuth
	if authSvc, err := core.Inject[contracts.AuthService](ctx); err == nil && authSvc != nil {
		SetAuthService(authSvc)
		if mw, ok := authSvc.RequireAuthMiddleware().(gin.HandlerFunc); ok {
			loginMW = mw
		}
		if mw, ok := authSvc.DisallowTokenAuthMiddleware().(gin.HandlerFunc); ok {
			noTokenMW = mw
		}
	} else {
		core.When[contracts.AuthService](ctx, func(svc contracts.AuthService) {
			SetAuthService(svc)
		})
	}
	ctx.OnDispose(func() error {
		SetAuthService(nil)
		return nil
	})

	// 1. Register migrations
	ctx.Migrations().Register("user", userMigrations)

	// 2. Initialize and provide UserService
	if p.userSvc == nil {
		p.userSvc = newUserService(ctx.Events())
	}
	core.Provide[contracts.UserService](ctx, p.userSvc)

	// 3. Register HTTP Routes
	userGroup := ctx.Router().Group("/api/v1/user")
	{
		userGroup.POST("/login", Login)
		userGroup.POST("/register", Register)
		userGroup.GET("/logout", Logout)
		userGroup.POST("/send-email-code", SendEmailCode)
		userGroup.POST("/change-password", loginMW, ChangePassword)
		userGroup.PUT("/profile", loginMW, UpdateProfile)

		// Access Tokens
		tokensGroup := userGroup.Group("/access-tokens", loginMW, noTokenMW)
		{
			tokensGroup.GET("", ListAccessTokens)
			tokensGroup.POST("", CreateAccessToken)
			tokensGroup.DELETE("/:id", DeleteAccessToken)
			tokensGroup.POST("/:id/rotate", RotateAccessToken)
		}
	}

	const (
		defaultUserTaskRetry = 3
		paramTypeString      = "string"
		paramNameEmail       = "email"
	)

	// 4. Register background tasks
	ctx.Task().Register("user:send_email_code", func(_ context.Context, _ []byte) error {
		return nil
	},
		extpoints.WithTaskType("send_email_code"),
		extpoints.WithTaskName("发送邮箱验证码"),
		extpoints.WithTaskDescription("异步发送用户注册与验证邮箱验证码"),
		extpoints.WithTaskCategory("user"),
		extpoints.WithTaskRetry(defaultUserTaskRetry),
		extpoints.WithTaskQueue("default"),
		extpoints.WithTaskRetryable(true),
		extpoints.WithTaskParams(
			contracts.TaskParamDTO{
				Name:        paramNameEmail,
				Label:       "目标邮箱",
				Type:        paramTypeString,
				Required:    true,
				Placeholder: "user@example.com",
				Description: "接收验证码的目标邮箱",
			},
			contracts.TaskParamDTO{
				Name:        "code",
				Label:       "验证码",
				Type:        paramTypeString,
				Required:    true,
				Placeholder: "123456",
				Description: "6 位数字验证码",
			},
		),
	)

	ctx.Task().Register("mail:send", func(_ context.Context, _ []byte) error {
		return nil
	},
		extpoints.WithTaskType("send_email"),
		extpoints.WithTaskName("发送邮件"),
		extpoints.WithTaskDescription("异步发送系统邮件"),
		extpoints.WithTaskCategory("mail"),
		extpoints.WithTaskRetry(defaultUserTaskRetry),
		extpoints.WithTaskQueue("default"),
		extpoints.WithTaskRetryable(true),
		extpoints.WithTaskParams(
			contracts.TaskParamDTO{
				Name:        "to",
				Label:       "接收邮箱 (To)",
				Type:        paramTypeString,
				Required:    true,
				Placeholder: "receiver@example.com",
				Description: "接收邮件的目标邮箱地址",
			},
			contracts.TaskParamDTO{
				Name:        "subject",
				Label:       "邮件主题 (Subject)",
				Type:        paramTypeString,
				Required:    true,
				Placeholder: "请输入邮件主题",
				Description: "发送邮件的主题标题",
			},
			contracts.TaskParamDTO{
				Name:        "body",
				Label:       "邮件内容 (Body)",
				Type:        "text",
				Required:    true,
				Placeholder: "请输入邮件内容（支持 HTML格式）",
				Description: "发送邮件的内容主体",
			},
		),
	)

	ctx.Task().Register("user:cleanup_inactive", func(_ context.Context, _ []byte) error {
		return nil
	},
		extpoints.WithTaskType("cleanup_inactive_users"),
		extpoints.WithTaskName("清理未激活用户"),
		extpoints.WithTaskDescription("清理长期未激活的注册用户与临时凭据"),
		extpoints.WithTaskCategory("user"),
		extpoints.WithTaskQueue("default"),
	)

	// 5. Register Settings Schemas
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "user.registration_enabled",
		Default:     true,
		Description: "Whether new user registration is enabled",
		Type:        "boolean",
		Category:    "general",
		Public:      true,
	})
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "user.password_login_enabled",
		Default:     true,
		Description: "Whether password login is enabled",
		Type:        "boolean",
		Category:    "general",
		Public:      true,
	})
	ctx.Settings().Register(extpoints.SettingSchema{
		Key:         "user.min_password_length",
		Default:     8,
		Description: "Minimum password length required for user accounts",
		Type:        "integer",
		Category:    "security",
	})

	return nil
}
