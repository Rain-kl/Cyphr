// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cap

import (
	"context"
	"testing"

	"Wavelet/core"
	"Wavelet/core/contracts"
)

func TestApplyProvidesCaptchaService(t *testing.T) {
	ctx := core.NewContext(context.Background())
	if err := New().Apply(ctx); err != nil {
		t.Fatal(err)
	}
	svc, err := core.Inject[contracts.CaptchaService](ctx)
	if err != nil || svc == nil {
		t.Fatalf("Inject CaptchaService: svc=%v err=%v", svc, err)
	}
	if svc.ChallengeHandler() == nil || svc.RedeemHandler() == nil {
		t.Fatal("handlers must be non-nil")
	}
	if svc.VerifyMiddleware("login") == nil {
		t.Fatal("VerifyMiddleware(login) must be non-nil")
	}
}

func TestApplyRegistersUnversionedCapRoutes(t *testing.T) {
	ctx := core.NewContext(context.Background())
	if err := New().Apply(ctx); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"POST /api/v1/cap/challenge": false,
		"POST /api/cap/challenge":    false,
		"POST /api/cap/redeem":       false,
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
