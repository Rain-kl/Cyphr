// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"

	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/plugins/domain/user"
)

func TestApplyWithoutCaptchaServiceKeepsAuthRoutes(t *testing.T) {
	ctx := core.NewContext(context.Background())
	if err := user.New().Apply(ctx); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"POST /api/v1/user/login":           false,
		"POST /api/v1/user/register":        false,
		"POST /api/v1/user/send-email-code": false,
	}
	for _, rd := range ctx.Router().Routes() {
		key := rd.Method + " " + rd.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for key, ok := range want {
		if !ok {
			t.Errorf("missing route %s", key)
		}
	}
}

func TestInjectDoesNotRequireCaptchaService(t *testing.T) {
	for _, dep := range user.New().Inject() {
		if dep == reflect.TypeFor[contracts.CaptchaService]() {
			t.Fatal("CaptchaService must not be a hard Inject() dependency")
		}
	}
}

type fakeCaptchaService struct{}

func (fakeCaptchaService) VerifyMiddleware(scope string) any {
	return gin.HandlerFunc(func(c *gin.Context) { c.Next() })
}

func (fakeCaptchaService) ChallengeHandler() any { return gin.HandlerFunc(func(c *gin.Context) {}) }

func (fakeCaptchaService) RedeemHandler() any { return gin.HandlerFunc(func(c *gin.Context) {}) }

func TestApplyWithCaptchaServiceWrapsLogin(t *testing.T) {
	ctx := core.NewContext(context.Background())
	core.Provide[contracts.CaptchaService](ctx, fakeCaptchaService{})
	if err := user.New().Apply(ctx); err != nil {
		t.Fatal(err)
	}
	for _, rd := range ctx.Router().Routes() {
		if rd.Method == "POST" && rd.Path == "/api/v1/user/login" {
			if len(rd.Handlers) <= 1 {
				t.Fatalf("login handler chain length = %d, want > 1", len(rd.Handlers))
			}
			return
		}
	}
	t.Fatal("missing POST /api/v1/user/login")
}
