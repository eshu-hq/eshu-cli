// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package resultshape_test

import (
	"context"
	"strings"
	"testing"

	"github.com/eshu-hq/eshu-cli/internal/cli/resultshape"
)

// Tests for the Tier-2 resultshape copy pin the frozen source at pin
// 78c5b3d1a4 (docs/tier2-pin.md); this test is destination-authored, so
// it is excluded from the no-fork freeze and pinned here instead, per the
// coverage-ratchet standing ruling (eshu-hq/eshu-cli#6).
func TestParseSelectsDualByDefault(t *testing.T) {
	for _, raw := range []string{"", "dual", "Dual", "DUAL", "dUaL"} {
		got, err := resultshape.Parse(raw)
		if err != nil {
			t.Fatalf("Parse(%q) error = %v, want nil", raw, err)
		}
		if got != resultshape.Dual {
			t.Fatalf("Parse(%q) = %q, want %q", raw, got, resultshape.Dual)
		}
	}
}

func TestParseSelectsStructuredCaseInsensitively(t *testing.T) {
	for _, raw := range []string{"structured", "Structured", "STRUCTURED"} {
		got, err := resultshape.Parse(raw)
		if err != nil {
			t.Fatalf("Parse(%q) error = %v, want nil", raw, err)
		}
		if got != resultshape.Structured {
			t.Fatalf("Parse(%q) = %q, want %q", raw, got, resultshape.Structured)
		}
	}
}

func TestParseRejectsUnknownAndPaddedValues(t *testing.T) {
	for _, raw := range []string{"json", " dual", "dual ", " structur ed ", "xml"} {
		if got, err := resultshape.Parse(raw); err == nil {
			t.Fatalf("Parse(%q) = %q, want error", raw, got)
		} else if !strings.Contains(err.Error(), "dual") || !strings.Contains(err.Error(), "structured") {
			t.Fatalf("Parse(%q) error %q does not name the allowed values", raw, err)
		}
	}
}

func TestOrDualFoldsToTwoValues(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   resultshape.Mode
		want resultshape.Mode
	}{
		{"structured stays", resultshape.Structured, resultshape.Structured},
		{"dual stays", resultshape.Dual, resultshape.Dual},
		{"zero value renders dual", "", resultshape.Dual},
		{"unknown folds to dual", "yaml", resultshape.Dual},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := resultshape.OrDual(tc.in); got != tc.want {
				t.Fatalf("OrDual(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestOverrideRoundTripThroughContext(t *testing.T) {
	ctx := resultshape.WithOverride(context.Background(), resultshape.Structured)
	got, ok := resultshape.FromContext(ctx)
	if !ok || got != resultshape.Structured {
		t.Fatalf("FromContext after WithOverride = (%q, %v), want (structured, true)", got, ok)
	}
}

func TestFromContextWithoutOverrideReportsFalse(t *testing.T) {
	if got, ok := resultshape.FromContext(context.Background()); ok || got != "" {
		t.Fatalf("FromContext(empty ctx) = (%q, %v), want (\"\", false)", got, ok)
	}
}

func TestFromContextWithStoredEmptyModeReportsFalse(t *testing.T) {
	ctx := resultshape.WithOverride(context.Background(), "")
	if got, ok := resultshape.FromContext(ctx); ok || got != "" {
		t.Fatalf("FromContext(stored empty) = (%q, %v), want (\"\", false)", got, ok)
	}
}
