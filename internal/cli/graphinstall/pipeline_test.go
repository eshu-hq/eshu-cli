// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package graphinstall

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func isolatedHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("ESHU_HOME", home)
	return home
}

func TestInstallRejectsNilVersionReader(t *testing.T) {
	isolatedHome(t)
	_, err := Install(Options{From: "/nonexistent"})
	if err == nil || !strings.Contains(err.Error(), "Options.ReadVersion is required") {
		t.Fatalf("got %v, want nil-reader error", err)
	}
}

func TestInstallLocalBinaryEndToEnd(t *testing.T) {
	home := isolatedHome(t)
	srcDir := t.TempDir()
	bin := writeFakeBinary(t, srcDir, "nornicdb-headless", "7.7.7")

	result, err := Install(Options{From: bin, ReadVersion: fakeReader("7.7.7")})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !result.Installed || result.Reused {
		t.Fatalf("result = %+v, want fresh install", result)
	}
	if result.Backend != "nornicdb" || result.Version != "7.7.7" {
		t.Fatalf("result = %+v, want backend/version pins", result)
	}
	wantBin := filepath.Join(home, "bin", "nornicdb-headless")
	if result.BinaryPath != wantBin {
		t.Fatalf("binary path = %q, want %q", result.BinaryPath, wantBin)
	}
	wantManifest := filepath.Join(home, "graph-backends", "nornicdb", "manifest.json")
	if result.ManifestPath != wantManifest {
		t.Fatalf("manifest path = %q, want %q", result.ManifestPath, wantManifest)
	}
	info, err := os.Stat(wantBin)
	if err != nil {
		t.Fatalf("stat managed binary: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatal("managed binary must be executable")
	}
	manifestInfo, err := os.Stat(wantManifest)
	if err != nil {
		t.Fatalf("stat manifest: %v", err)
	}
	if manifestInfo.Mode().Perm() != 0o600 {
		t.Fatalf("manifest mode = %o, want 600", manifestInfo.Mode().Perm())
	}

	// The manifest is the stable on-disk contract: backend, paths, version,
	// checksums, source kind, and install mode survive a re-read.
	raw, err := os.ReadFile(wantManifest)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	for key, want := range map[string]any{
		"backend":      "nornicdb",
		"binary_path":  wantBin,
		"version":      "7.7.7",
		"source_kind":  "local-binary",
		"install_mode": "local-binary",
		"headless":     true,
	} {
		if decoded[key] != want {
			t.Fatalf("manifest[%q] = %v, want %v", key, decoded[key], want)
		}
	}
	if decoded["installed_at"] == "" || decoded["sha256"] == "" {
		t.Fatalf("manifest misses installed_at/sha256: %v", decoded)
	}
}

func TestInstallReusesMatchingBinary(t *testing.T) {
	isolatedHome(t)
	srcDir := t.TempDir()
	bin := writeFakeBinary(t, srcDir, "nornicdb-headless", "7.7.7")

	first, err := Install(Options{From: bin, ReadVersion: fakeReader("7.7.7")})
	if err != nil {
		t.Fatalf("first install: %v", err)
	}
	before, err := os.Stat(first.BinaryPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	second, err := Install(Options{From: bin, ReadVersion: fakeReader("7.7.7")})
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if !second.Reused {
		t.Fatalf("second result = %+v, want reuse", second)
	}
	after, err := os.Stat(first.BinaryPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("reuse must not rewrite the managed binary")
	}
}

func TestInstallRefusesDifferentBinaryWithoutForce(t *testing.T) {
	isolatedHome(t)
	srcDir := t.TempDir()
	first := writeFakeBinary(t, srcDir, "first", "1.0.0")
	if _, err := Install(Options{From: first, ReadVersion: fakeReader("1.0.0")}); err != nil {
		t.Fatalf("first install: %v", err)
	}
	otherDir := t.TempDir()
	other := writeFakeBinary(t, otherDir, "other", "2.0.0")
	_, err := Install(Options{From: other, ReadVersion: fakeReader("2.0.0")})
	if err == nil || !strings.Contains(err.Error(), "already exists") || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("got %v, want exists/--force error", err)
	}
	forced, err := Install(Options{From: other, Force: true, ReadVersion: fakeReader("2.0.0")})
	if err != nil {
		t.Fatalf("forced install: %v", err)
	}
	if forced.Reused || forced.Version != "2.0.0" {
		t.Fatalf("forced result = %+v, want fresh 2.0.0 install", forced)
	}
}

func TestInstallSamePathSourceReuses(t *testing.T) {
	home := isolatedHome(t)
	srcDir := t.TempDir()
	bin := writeFakeBinary(t, srcDir, "nornicdb-headless", "5.0.0")
	if _, err := Install(Options{From: bin, ReadVersion: fakeReader("5.0.0")}); err != nil {
		t.Fatalf("install: %v", err)
	}
	managed := filepath.Join(home, "bin", "nornicdb-headless")
	result, err := Install(Options{From: managed, ReadVersion: fakeReader("5.0.0")})
	if err != nil {
		t.Fatalf("same-path install: %v", err)
	}
	if !result.Reused {
		t.Fatalf("same-path result = %+v, want reuse", result)
	}
}

func TestInstallBlankFromFallsToManifestGuidance(t *testing.T) {
	// Core trims --from before the empty check: whitespace-only input
	// prints the manifest guidance, never a directory error.
	isolatedHome(t)
	_, err := Install(Options{From: "   ", ReadVersion: fakeReader("1.0")})
	if err == nil || !strings.Contains(err.Error(), "no embedded headless NornicDB release asset") {
		t.Fatalf("got %v, want manifest-guidance error", err)
	}
}

func TestInstallEmptyFromFallsToManifestGuidance(t *testing.T) {
	isolatedHome(t)
	_, err := Install(Options{ReadVersion: fakeReader("1.0")})
	if err == nil || !strings.Contains(err.Error(), "no embedded headless NornicDB release asset") {
		t.Fatalf("got %v, want manifest-guidance error", err)
	}
}

func TestManagedBinaryIfPresent(t *testing.T) {
	home := isolatedHome(t)
	managed := filepath.Join(home, "bin", "nornicdb-headless")

	if _, err := ManagedBinaryIfPresent(fakeReader("1.0")); !os.IsNotExist(err) {
		t.Fatalf("got %v, want os.IsNotExist when nothing is installed", err)
	}

	srcDir := t.TempDir()
	bin := writeFakeBinary(t, srcDir, "nornicdb-headless", "8.0.0")
	if _, err := Install(Options{From: bin, ReadVersion: fakeReader("8.0.0")}); err != nil {
		t.Fatalf("install: %v", err)
	}
	got, err := ManagedBinaryIfPresent(fakeReader("8.0.0"))
	if err != nil {
		t.Fatalf("present: %v", err)
	}
	if got != managed {
		t.Fatalf("got %q, want %q", got, managed)
	}
	failing := VersionReader(func(binaryPath string) (string, error) { return "", errFakeVersion })
	if _, err := ManagedBinaryIfPresent(failing); err == nil {
		t.Fatal("want error when the managed binary fails verification, got nil")
	}
}

func TestInstallDownloadTimeoutValidation(t *testing.T) {
	t.Setenv("ESHU_NORNICDB_INSTALL_TIMEOUT", "not-a-duration")
	if _, err := installDownloadTimeout(); err == nil {
		t.Fatal("want error for an unparsable timeout, got nil")
	}
	t.Setenv("ESHU_NORNICDB_INSTALL_TIMEOUT", "-5s")
	if _, err := installDownloadTimeout(); err == nil || !strings.Contains(err.Error(), "greater than zero") {
		t.Fatalf("got %v, want positive-timeout error", err)
	}
}
