// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type segmentEntry struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

func parseSegments(rawResp any) []segmentEntry {
	if rawResp == nil {
		return nil
	}
	var verbose struct {
		Segments []segmentEntry `json:"segments"`
	}
	b, err := json.Marshal(rawResp)
	if err != nil {
		return nil
	}
	if err := json.Unmarshal(b, &verbose); err != nil {
		return nil
	}
	return verbose.Segments
}

// formatSRTTime formats seconds into SRT timestamp format 00:00:00,000.
func formatSRTTime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	h := int(seconds) / secPerHour
	m := (int(seconds) % secPerHour) / secPerMin
	s := int(seconds) % secPerMin
	ms := int((seconds - float64(int(seconds))) * msPerSecond)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

// buildSRTContent constructs subtitle entries from OpenAI verbose_json segments or full text fallback.
func buildSRTContent(resultText string, rawResp any, totalDuration float64) string {
	segments := parseSegments(rawResp)
	if len(segments) > 0 {
		var sb strings.Builder
		idx := 1
		for _, seg := range segments {
			trimmed := strings.TrimSpace(seg.Text)
			if trimmed == "" {
				continue
			}
			fmt.Fprintf(&sb, "%d\n%s --> %s\n%s\n\n", idx, formatSRTTime(seg.Start), formatSRTTime(seg.End), trimmed)
			idx++
		}
		if sb.Len() > 0 {
			return strings.TrimRight(sb.String(), "\n") + "\n"
		}
	}

	trimmed := strings.TrimSpace(resultText)
	if trimmed == "" {
		return ""
	}
	endSec := totalDuration
	if endSec <= 0 {
		endSec = 5.0
	}
	return fmt.Sprintf("1\n00:00:00,000 --> %s\n%s\n", formatSRTTime(endSec), trimmed)
}

func saveJobResults(cmd *cobra.Command, baseName, outDir, resultText string, openAIResp any, duration float64) error {
	if outDir != "" {
		if err := os.MkdirAll(outDir, dirPerm); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
	}

	var txtPath, srtPath string
	if outDir != "" {
		txtPath = filepath.Join(outDir, baseName+".txt")
		srtPath = filepath.Join(outDir, baseName+".srt")
	} else {
		txtPath = baseName + ".txt"
		srtPath = baseName + ".srt"
	}

	if werr := os.WriteFile(txtPath, []byte(resultText), resultFilePerm); werr != nil {
		return fmt.Errorf("write txt result file: %w", werr)
	}
	cmd.Printf("Text result saved to %s\n", txtPath)

	srtContent := buildSRTContent(resultText, openAIResp, duration)
	if werr := os.WriteFile(srtPath, []byte(srtContent), resultFilePerm); werr != nil {
		return fmt.Errorf("write srt result file: %w", werr)
	}
	cmd.Printf("SRT result saved to %s\n", srtPath)
	return nil
}
