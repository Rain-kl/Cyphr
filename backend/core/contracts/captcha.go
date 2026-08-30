// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package contracts

// CaptchaService defines the contract for CAPTCHA challenge issuance,
// redemption, and scoped verification middleware.
type CaptchaService interface {
	VerifyMiddleware(scope string) any
	ChallengeHandler() any
	RedeemHandler() any
}
