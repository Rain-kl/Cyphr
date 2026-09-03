// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"Wavelet/plugins/domain/auth/pow"
)

// ChallengeResponse is a local type alias for the pow.ChallengeResponse struct
type ChallengeResponse = pow.ChallengeResponse

// challengeRequest is the CAPTCHA challenge request payload.
type challengeRequest struct {
	Scope string `json:"scope" form:"scope"`
}

// redeemRequest is the CAPTCHA redeem request payload.
type redeemRequest struct {
	Token     string `json:"token" binding:"required"`
	Solutions []int  `json:"solutions" binding:"required"`
	Scope     string `json:"scope" form:"scope"`
}

// RedeemResponse is returned to the client on redeem.
type RedeemResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	Expires int64  `json:"expires,omitempty"`
	Error   string `json:"error,omitempty"`
}

// capConfigRecord maps the columns selected from the system config table.
type capConfigRecord struct {
	Key   string `gorm:"column:key"`
	Value string `gorm:"column:value"`
}
