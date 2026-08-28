// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package util

import "testing"

func TestEscapeLike(t *testing.T) {
	cases := map[string]string{
		"":            "",
		"/my_page":    `/my\_page`,
		"100%":        `100\%`,
		`a\b`:         `a\\b`,
		`%_\`:         `\%\_\\`,
		"normal/path": "normal/path",
	}
	for input, want := range cases {
		if got := EscapeLike(input); got != want {
			t.Errorf("EscapeLike(%q) = %q, want %q", input, got, want)
		}
	}
}
