// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package message_gateway

import (
	"crypto/rand"
	"strings"
	"unicode"
)

// CodeAlphabet excludes easily confused runes 0/O/1/I.
const CodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// CodeLength is the raw pairing code size.
const CodeLength = 8

// GenerateCode returns an 8-character pairing code.
func GenerateCode() (string, error) {
	buf := make([]byte, CodeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, CodeLength)
	for i, b := range buf {
		out[i] = CodeAlphabet[int(b)%len(CodeAlphabet)]
	}
	return string(out), nil
}

// NormalizeCode strips separators and uppercases.
func NormalizeCode(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '-' || unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(unicode.ToUpper(r))
	}
	return b.String()
}

// FormatCode renders ABCD-EFGH.
func FormatCode(s string) string {
	s = NormalizeCode(s)
	if len(s) != CodeLength {
		return s
	}
	return s[:4] + "-" + s[4:]
}
