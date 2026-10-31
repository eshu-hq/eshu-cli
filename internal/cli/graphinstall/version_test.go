// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package graphinstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseNornicDBVersionOutput(t *testing.T) {
	cases := []struct {
		name    string
		output  string
		want    string
		wantErr string
	}{
		{"plain", "NornicDB 1.2.3", "1.2.3", ""},
		{"trailing-newline", "NornicDB 1.2.3\n", "1.2.3", ""},
		{"headless-suffix", "NornicDB 0.9.0-headless", "0.9.0-headless", ""},
		{"empty", "", "", "empty output"},
		{"blank", "  \n", "", "empty output"},
		{"wrong-binary", "postgres 15.2", "", "unexpected output"},
		// A bare "NornicDB " trims to "NornicDB" before the prefix check,
		// so it reports "unexpected output" exactly like the core probe:
		// the trim-then-prefix order is part of the pinned protocol.
		{"bare-prefix", "NornicDB ", "", "unexpected output"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseNornicDBVersionOutput(tc.output)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("parse: %v", err)
				}
				if got != tc.want {
					t.Fatalf("got %q, want %q", got, tc.want)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("got %v, want error containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestDefaultVersionReaderProbesBinaryVersion(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "nornicdb-headless")
	script := "#!/bin/sh\nif [ \"$1\" = version ]; then echo \"NornicDB 9.9.9-test\"; exit 0; fi\necho \"unexpected args: $@\" >&2\nexit 1\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	got, err := DefaultVersionReader(fake)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if got != "9.9.9-test" {
		t.Fatalf("got %q, want %q", got, "9.9.9-test")
	}
}

func TestDefaultVersionReaderRejectsBadProbe(t *testing.T) {
	if _, err := DefaultVersionReader(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("want error for a missing candidate binary, got nil")
	}
	dir := t.TempDir()
	wrong := filepath.Join(dir, "wrong")
	if err := os.WriteFile(wrong, []byte("#!/bin/sh\necho \"other 1.0\"\n"), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	if _, err := DefaultVersionReader(wrong); err == nil {
		t.Fatal("want error for a non-NornicDB version line, got nil")
	}
}

func TestResolvePinnedReleaseSourceIsFailClosed(t *testing.T) {
	manifest, err := defaultReleaseManifest()
	if err != nil {
		t.Fatalf("decode embedded manifest: %v", err)
	}
	_, _, err = resolvePinnedReleaseSource(manifest, "dev", "linux", "arm64", true)
	if err == nil {
		t.Fatal("want guidance error for the empty release manifest, got nil")
	}
	msg := err.Error()
	for _, fragment := range []string{"no embedded headless NornicDB release asset", "Eshu dev on linux/arm64", "--from"} {
		if !strings.Contains(msg, fragment) {
			t.Fatalf("error %q misses fragment %q", msg, fragment)
		}
	}
}
