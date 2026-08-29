// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"testing"
)

func TestParsePositiveInt(t *testing.T) {
	const untouched = 77

	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "empty means zero", input: "", want: 0},
		{name: "zero accepted", input: "0", want: 0},
		{name: "positive accepted", input: "42", want: 42},
		{name: "negative rejected", input: "-5", want: untouched, wantErr: true},
		{name: "oversized rejected", input: "99999999999999999999", want: untouched, wantErr: true},
		{name: "non numeric rejected", input: "abc", want: untouched, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := untouched
			err := parsePositiveInt(tt.input, &got)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parsePositiveInt(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("parsePositiveInt(%q) left result %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
