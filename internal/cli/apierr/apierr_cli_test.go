// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package apierr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/eshu-hq/eshu-cli/internal/cli/apierr"
)

// Tests for the Tier-1 apierr copy pin the frozen source at pin
// 78c5b3d1a4 (docs/tier1-pin.md); this test is destination-authored, so
// it is excluded from the no-fork freeze and pinned here instead, per the
// coverage-ratchet standing ruling (eshu-hq/eshu-cli#6).
type statusError struct{ code int }

func (e *statusError) Error() string       { return fmt.Sprintf("api error %d", e.code) }
func (e *statusError) HTTPStatusCode() int { return e.code }

func TestStatusCodeFindsDirectCarrier(t *testing.T) {
	got, ok := apierr.StatusCode(&statusError{code: 404})
	if !ok || got != 404 {
		t.Fatalf("StatusCode(direct) = (%d, %v), want (404, true)", got, ok)
	}
}

func TestStatusCodeUnwrapsChain(t *testing.T) {
	err := fmt.Errorf("call failed: %w", fmt.Errorf("transport: %w", &statusError{code: 503}))
	got, ok := apierr.StatusCode(err)
	if !ok || got != 503 {
		t.Fatalf("StatusCode(wrapped) = (%d, %v), want (503, true)", got, ok)
	}
}

func TestStatusCodeReportsFalseWithoutCarrier(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"nil", nil},
		{"plain", errors.New("connection refused")},
		{"wrapped plain", fmt.Errorf("dial: %w", errors.New("connection refused"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, ok := apierr.StatusCode(tc.err); ok || got != 0 {
				t.Fatalf("StatusCode(%v) = (%d, %v), want (0, false)", tc.err, got, ok)
			}
		})
	}
}
