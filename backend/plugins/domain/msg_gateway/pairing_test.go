// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package msg_gateway

import (
	"strings"
	"testing"
)

func TestGenerateCode_AlphabetAndLength(t *testing.T) {
	code, err := GenerateCode()
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 8 {
		t.Fatalf("len=%d", len(code))
	}
	for _, r := range code {
		if !strings.ContainsRune(CodeAlphabet, r) {
			t.Fatalf("bad rune %q", r)
		}
	}
}

func TestNormalizeAndFormat(t *testing.T) {
	if got := NormalizeCode("ab-cd-ef-gh"); got != "ABCDEFGH" {
		t.Fatalf("got %q", got)
	}
	if got := FormatCode("ABCDEFGH"); got != "ABCD-EFGH" {
		t.Fatalf("got %q", got)
	}
}
