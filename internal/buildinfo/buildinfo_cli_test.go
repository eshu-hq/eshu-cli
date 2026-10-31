// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package buildinfo_test

import (
	"strings"
	"testing"

	"github.com/eshu-hq/eshu-cli/internal/buildinfo"
)

// Tests for the Tier-1 buildinfo copy pin the frozen source at pin
// 78c5b3d1a4 (docs/tier1-pin.md); this test is destination-authored, so
// it is excluded from the no-fork freeze and pinned here instead, per the
// coverage-ratchet standing ruling (eshu-hq/eshu-cli#6).
func TestAppVersionPrefersLinkerVersion(t *testing.T) {
	old := buildinfo.Version
	t.Cleanup(func() { buildinfo.Version = old })
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{"plain", "v1.2.3", "v1.2.3"},
		{"padded trims", "  v1.2.3  ", "v1.2.3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			buildinfo.Version = tc.raw
			if got := buildinfo.AppVersion(); got != tc.want {
				t.Fatalf("AppVersion() with Version=%q = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestAppVersionSourceBuildReportsDev(t *testing.T) {
	old := buildinfo.Version
	t.Cleanup(func() { buildinfo.Version = old })
	for _, raw := range []string{"dev", "", "  "} {
		buildinfo.Version = raw
		if got := buildinfo.AppVersion(); got != "dev" {
			t.Fatalf("AppVersion() with Version=%q = %q, want dev", raw, got)
		}
	}
}

func TestPrintVersionFlagHandlesOnlyVersionProbe(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want bool
	}{
		{"none", nil, false},
		{"empty", []string{}, false},
		{"two args", []string{"--version", "extra"}, false},
		{"other flag", []string{"--help"}, false},
		{"long", []string{"--version"}, true},
		{"short", []string{"-v"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var sb strings.Builder
			got, err := buildinfo.PrintVersionFlag(tc.args, &sb, "probe")
			if err != nil {
				t.Fatalf("PrintVersionFlag(%q) error = %v, want nil", tc.args, err)
			}
			if got != tc.want {
				t.Fatalf("PrintVersionFlag(%q) handled = %v, want %v", tc.args, got, tc.want)
			}
			if tc.want && sb.String() != "probe "+buildinfo.AppVersion()+"\n" {
				t.Fatalf("PrintVersionFlag(%q) wrote %q", tc.args, sb.String())
			}
			if !tc.want && sb.String() != "" {
				t.Fatalf("PrintVersionFlag(%q) wrote %q, want nothing", tc.args, sb.String())
			}
		})
	}
}

func TestPrintVersionFlagDefaultsEmptyName(t *testing.T) {
	for _, name := range []string{"", "   "} {
		var sb strings.Builder
		got, err := buildinfo.PrintVersionFlag([]string{"--version"}, &sb, name)
		if err != nil || !got {
			t.Fatalf("PrintVersionFlag(name=%q) = (%v, %v), want (true, nil)", name, got, err)
		}
		if sb.String() != "eshu "+buildinfo.AppVersion()+"\n" {
			t.Fatalf("PrintVersionFlag(name=%q) wrote %q", name, sb.String())
		}
	}
}
