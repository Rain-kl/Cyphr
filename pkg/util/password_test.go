// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package util

import "testing"

func TestDummyCheckPasswordDoesNotPanic(t *testing.T) {
	DummyCheckPassword("any-password")
}

func TestCheckPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("secret-pass")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPasswordHash(hash, "secret-pass") {
		t.Fatal("expected matching password to succeed")
	}
	if CheckPasswordHash(hash, "other-pass") {
		t.Fatal("expected mismatched password to fail")
	}
}
