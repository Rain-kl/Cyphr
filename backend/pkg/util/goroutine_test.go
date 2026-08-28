// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"sync"
	"testing"
)

func TestGoRecoversPanic(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	// Go should swallow the panic without crashing the test process
	Go(func() {
		defer wg.Done()
		panic("boom")
	})

	wg.Wait()
}

func TestGoRunsNormally(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	ran := false

	Go(func() {
		defer wg.Done()
		ran = true
	})

	wg.Wait()
	if !ran {
		t.Fatal("expected fn to run")
	}
}
