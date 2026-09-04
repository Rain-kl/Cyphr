// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"path/filepath"
	"strings"
)

const (
	widthZero           = 0
	widthHalf           = 1
	widthFull           = 2
	minEllipsisWidth    = 3
	maxPreservedExtLen  = 6
	minFileColTruncBase = 4
	ellipsisStr         = "..."
)

type runeRange struct {
	start rune
	end   rune
}

var zeroWidthRanges = [...]runeRange{
	{start: 0x0300, end: 0x036F}, // Combining Diacritical Marks
	{start: 0x1DC0, end: 0x1DFF}, // Combining Diacritical Marks Supplement
	{start: 0x200B, end: 0x200F}, // Zero width spaces, LTR/RTL marks
	{start: 0x20D0, end: 0x20FF}, // Combining Marks for Symbols
	{start: 0xFE00, end: 0xFE0F}, // Variation Selectors
	{start: 0xFE20, end: 0xFE2F}, // Combining Half Marks
}

var wideRanges = [...]runeRange{
	{start: 0x1100, end: 0x115F},   // Hangul Jamo
	{start: 0x2E80, end: 0x303E},   // CJK Radicals, Kangxi, CJK Symbols
	{start: 0x3040, end: 0x309F},   // Hiragana
	{start: 0x30A0, end: 0x30FF},   // Katakana
	{start: 0x3100, end: 0x312F},   // Bopomofo
	{start: 0x3130, end: 0x318F},   // Hangul Compatibility Jamo
	{start: 0x3190, end: 0x31E3},   // Kanbun, Bopomofo Extended, CJK Strokes
	{start: 0x3200, end: 0x4DBF},   // Enclosed CJK, CJK Compatibility, CJK Unified Extension A
	{start: 0x4E00, end: 0x9FFF},   // CJK Unified Ideographs
	{start: 0xAC00, end: 0xD7A3},   // Hangul Syllables
	{start: 0xF900, end: 0xFAFF},   // CJK Compatibility Ideographs
	{start: 0xFE10, end: 0xFE19},   // Vertical Forms
	{start: 0xFE30, end: 0xFE4F},   // CJK Compatibility Forms
	{start: 0xFF01, end: 0xFF60},   // Fullwidth Forms
	{start: 0xFFE0, end: 0xFFE6},   // Fullwidth Signs
	{start: 0x1F300, end: 0x1F64F}, // Miscellaneous Symbols and Pictographs
	{start: 0x1F680, end: 0x1F6FF}, // Transport and Map Symbols
	{start: 0x20000, end: 0x2FA1F}, // CJK Unified Extension B-F
}

func inRanges(r rune, ranges []runeRange) bool {
	for i := range ranges {
		if r >= ranges[i].start && r <= ranges[i].end {
			return true
		}
	}
	return false
}

// RuneWidth returns the display width of a rune in terminal columns (0, 1, or 2).
// It accurately accounts for East Asian Wide / Fullwidth characters (CJK).
func RuneWidth(r rune) int {
	if r < 32 || (r >= 0x7f && r < 0xa0) {
		return widthZero
	}
	if inRanges(r, zeroWidthRanges[:]) {
		return widthZero
	}
	if inRanges(r, wideRanges[:]) {
		return widthFull
	}
	return widthHalf
}

// StringWidth returns the visual display width of a string in terminal cells.
func StringWidth(s string) int {
	w := 0
	for _, r := range s {
		w += RuneWidth(r)
	}
	return w
}

// PadRight pads s on the right with spaces until its terminal visual width equals targetWidth.
func PadRight(s string, targetWidth int) string {
	sw := StringWidth(s)
	if sw >= targetWidth {
		return s
	}
	return s + strings.Repeat(" ", targetWidth-sw)
}

func truncateRunes(s string, maxW int) string {
	var b strings.Builder
	cur := 0
	for _, r := range s {
		rw := RuneWidth(r)
		if cur+rw > maxW {
			break
		}
		b.WriteRune(r)
		cur += rw
	}
	return b.String()
}

// TruncateVisual truncates string s so its terminal visual width does not exceed maxWidth.
// If s has a common media file extension, it preserves the extension (e.g. "my_long_audio...mp3").
func TruncateVisual(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if StringWidth(s) <= maxWidth {
		return s
	}
	if maxWidth <= minEllipsisWidth {
		return strings.Repeat(".", maxWidth)
	}

	ext := filepath.Ext(s)
	extWidth := StringWidth(ext)
	if ext != "" && extWidth > 0 && extWidth <= maxPreservedExtLen && maxWidth > extWidth+minFileColTruncBase {
		base := strings.TrimSuffix(s, ext)
		availableForBase := maxWidth - extWidth - minEllipsisWidth
		return truncateRunes(base, availableForBase) + ellipsisStr + ext
	}

	available := maxWidth - minEllipsisWidth
	return truncateRunes(s, available) + ellipsisStr
}

