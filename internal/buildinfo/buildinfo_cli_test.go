// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package buildinfo

import (
	"errors"
	"runtime/debug"
	"strings"
	"testing"
)

// Tests for the Tier-1 buildinfo copy pin the frozen source at pin
// 78c5b3d1a4 (docs/tier1-pin.md); this test is destination-authored, so
// it is excluded from the no-fork freeze and pinned here instead, per the
// coverage-ratchet standing ruling (eshu-hq/eshu-cli#6).
//
// The file lives in the internal test package (not buildinfo_test) so it
// can pin normalizeAppVersion directly: AppVersion reads the real test
// binary build info, whose main module version is always "(devel)", so the
// module-version arm is unreachable through the exported surface.
func TestAppVersionPrefersLinkerVersion(t *testing.T) {
	old := Version
	t.Cleanup(func() { Version = old })
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{"plain", "v1.2.3", "v1.2.3"},
		{"padded trims", "  v1.2.3  ", "v1.2.3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			Version = tc.raw
			if got := AppVersion(); got != tc.want {
				t.Fatalf("AppVersion() with Version=%q = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestAppVersionSourceBuildReportsDev(t *testing.T) {
	old := Version
	t.Cleanup(func() { Version = old })
	for _, raw := range []string{"dev", "", "  "} {
		Version = raw
		if got := AppVersion(); got != "dev" {
			t.Fatalf("AppVersion() with Version=%q = %q, want dev", raw, got)
		}
	}
}

// TestNormalizeAppVersionModuleVersion pins the precedence normalizeAppVersion
// applies once the linker version is absent: a real module version wins,
// while empty, blank, and "(devel)" builds fall back to dev. Empty raw input
// never consults the module, and a missing build info (nil or !ok) is dev.
func TestNormalizeAppVersionModuleVersion(t *testing.T) {
	for _, tc := range []struct {
		name    string
		raw     string
		main    string
		ok      bool
		nilInfo bool
		want    string
	}{
		{"linker wins over module", "v1.2.3", "v9.9.9", true, false, "v1.2.3"},
		{"dev takes module", "dev", "v0.9.0", true, false, "v0.9.0"},
		{"dev trims module", "dev", "  v0.9.0  ", true, false, "v0.9.0"},
		{"dev empty module stays dev", "dev", "", true, false, "dev"},
		{"dev blank module stays dev", "dev", "   ", true, false, "dev"},
		{"dev devel sentinel stays dev", "dev", "(devel)", true, false, "dev"},
		{"dev padded sentinel stays dev", "dev", "  (devel)  ", true, false, "dev"},
		{"dev nil info stays dev", "dev", "v0.9.0", true, true, "dev"},
		{"dev unreadable info stays dev", "dev", "v0.9.0", false, false, "dev"},
		{"empty raw ignores module", "", "v0.9.0", true, false, "dev"},
		{"blank raw ignores module", "   ", "v0.9.0", true, false, "dev"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var info *debug.BuildInfo
			if !tc.nilInfo {
				info = &debug.BuildInfo{Main: debug.Module{Version: tc.main}}
			}
			if got := normalizeAppVersion(tc.raw, info, tc.ok); got != tc.want {
				t.Fatalf("normalizeAppVersion(%q, Main.Version=%q, ok=%v) = %q, want %q",
					tc.raw, tc.main, tc.ok, got, tc.want)
			}
		})
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
			got, err := PrintVersionFlag(tc.args, &sb, "probe")
			if err != nil {
				t.Fatalf("PrintVersionFlag(%q) error = %v, want nil", tc.args, err)
			}
			if got != tc.want {
				t.Fatalf("PrintVersionFlag(%q) handled = %v, want %v", tc.args, got, tc.want)
			}
			if tc.want && sb.String() != "probe "+AppVersion()+"\n" {
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
		got, err := PrintVersionFlag([]string{"--version"}, &sb, name)
		if err != nil || !got {
			t.Fatalf("PrintVersionFlag(name=%q) = (%v, %v), want (true, nil)", name, got, err)
		}
		if sb.String() != "eshu "+AppVersion()+"\n" {
			t.Fatalf("PrintVersionFlag(name=%q) wrote %q", name, sb.String())
		}
	}
}

// failWriter is an io.Writer that always fails, exercising the Fprintf
// write-error arm a strings.Builder can never reach.
type failWriter struct{}

func (failWriter) Write(p []byte) (int, error) { return 0, errors.New("no space") }

// TestPrintVersionFlagWriteError proves the version probe surfaces a failing
// stdout instead of swallowing it, and that non-probe args never touch the
// writer.
func TestPrintVersionFlagWriteError(t *testing.T) {
	handled, err := PrintVersionFlag([]string{"--version"}, failWriter{}, "probe")
	if err == nil {
		t.Fatal("PrintVersionFlag(failWriter) = nil, want write error")
	}
	if !handled {
		t.Fatal("PrintVersionFlag(failWriter) handled = false, want true")
	}
	handled, err = PrintVersionFlag([]string{"--help"}, failWriter{}, "probe")
	if err != nil || handled {
		t.Fatalf("PrintVersionFlag(--help, failWriter) = (%v, %v), want (false, nil)", handled, err)
	}
}
