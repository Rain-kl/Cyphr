// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"Wavelet/transcribe/plugins/cli/client"
)

func TestStringWidthAndRuneWidth(t *testing.T) {
	// ASCII characters have width 1
	assert.Equal(t, 5, StringWidth("1.mp3"))
	assert.Equal(t, 6, StringWidth("JOB ID"))

	// Chinese characters have width 2
	assert.Equal(t, 4, StringWidth("切片"))
	// "01 切片1" -> '0'(1), '1'(1), ' '(1), '切'(2), '片'(2), '1'(1) = 8
	assert.Equal(t, 8, StringWidth("01 切片1"))

	// Full-width punctuation has width 2
	assert.Equal(t, 2, StringWidth("，"))
	assert.Equal(t, 2, StringWidth("。"))
}

func TestPadRight(t *testing.T) {
	// ASCII string padded to 10
	s1 := PadRight("abc", 10)
	assert.Equal(t, 10, StringWidth(s1))
	assert.Equal(t, "abc       ", s1)

	// Chinese string padded to 10
	s2 := PadRight("切片", 10)
	assert.Equal(t, 10, StringWidth(s2))
	assert.Equal(t, "切片      ", s2)

	// Mixed string padded to 20
	s3 := PadRight("01 切片1.mp3", 20)
	assert.Equal(t, 20, StringWidth(s3))
}

func TestTruncateVisual(t *testing.T) {
	// Shorter than max
	assert.Equal(t, "short.mp3", TruncateVisual("short.mp3", 20))

	// Exactly max
	assert.Equal(t, "hello.wav", TruncateVisual("hello.wav", 9))

	// Truncate preserving extension: "01 切片1 二维随机变量的概念及其分布.mp3"
	longName := "01 切片1 二维随机变量的概念及其分布.mp3"
	truncated := TruncateVisual(longName, 30)
	assert.LessOrEqual(t, StringWidth(truncated), 30)
	assert.True(t, strings.HasSuffix(truncated, ".mp3"))
	assert.Contains(t, truncated, "...")

	// Truncate without extension
	noExt := "这是一个非常非常非常非常非常长的标题没有任何扩展名"
	truncatedNoExt := TruncateVisual(noExt, 20)
	assert.LessOrEqual(t, StringWidth(truncatedNoExt), 20)
	assert.True(t, strings.HasSuffix(truncatedNoExt, "..."))
}

func TestRenderJobsTableAlignment(t *testing.T) {
	items := []client.JobInfo{
		{
			ID:               100516759517270016,
			OriginalFileName: "01 切片1 二维随机变量的概念及其分布.mp3",
			Model:            "qwen3-asr-0.6b",
			Status:           "completed",
			Progress:         100,
			Duration:         71.73,
			CreatedAt:        time.Date(2026, 9, 4, 16, 57, 42, 0, time.UTC),
		},
		{
			ID:               100516701195472896,
			OriginalFileName: "06 切片6 一维随机变量函数的分布.mp3",
			Model:            "qwen3-asr-0.6b",
			Status:           "completed",
			Progress:         100,
			Duration:         54.53,
			CreatedAt:        time.Date(2026, 9, 4, 16, 57, 29, 0, time.UTC),
		},
		{
			ID:               100514754417659904,
			OriginalFileName: "1.mp3",
			Model:            "qwen3-asr-0.6b",
			Status:           "completed",
			Progress:         100,
			Duration:         15.82,
			CreatedAt:        time.Date(2026, 9, 4, 16, 49, 44, 0, time.UTC),
		},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	renderJobsTable(cmd, items, false)
	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Len(t, lines, 4) // header + 3 rows

	// Verify header and each line has MODEL starting at the exact same visual position
	headerModelIdx := strings.Index(lines[0], "MODEL")
	assert.Greater(t, headerModelIdx, 0)

	for i := 1; i < len(lines); i++ {
		line := lines[i]
		// The model name "qwen3-asr-0.6b" must appear at the exact visual offset
		// Verify by checking substring up to MODEL
		idx := strings.Index(line, "qwen3-asr-0.6b")
		assert.Greater(t, idx, 0)
		visualWidthBeforeModel := StringWidth(line[:idx])
		headerVisualWidth := StringWidth(lines[0][:headerModelIdx])
		assert.Equal(t, headerVisualWidth, visualWidthBeforeModel, "Row %d MODEL column must align with header", i)
	}
}
