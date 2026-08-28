// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package message_gateway_test

import (
	"context"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/pkg/testhelper"
	"github.com/Rain-kl/Wavelet/plugins/domain/message_gateway"
)

func TestUpsertPairingCode_ReusesUnexpired(t *testing.T) {
	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()
	first, err := message_gateway.UpsertPairingCode(ctx, 1, "tg-1", "ABCD1234", time.Now().Add(15*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	second, err := message_gateway.UpsertPairingCode(ctx, 1, "tg-1", "ZZZZ9999", time.Now().Add(15*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if first.Code != second.Code || first.Code != "ABCD1234" {
		t.Fatalf("reuse failed: %+v %+v", first, second)
	}
}
