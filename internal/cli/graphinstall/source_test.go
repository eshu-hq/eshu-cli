// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package graphinstall

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeReader(version string) VersionReader {
	return func(binaryPath string) (string, error) { return version, nil }
}

func writeFakeBinary(t *testing.T, dir, name, version string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\necho \"NornicDB " + version + "\"\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	return path
}

func TestNormalizeInstallSHA256(t *testing.T) {
	good := "ab" + strings.Repeat("cd", 31)
	cases := []struct {
		name     string
		raw      string
		required bool
		want     string
		wantErr  string
	}{
		{"local-empty", "", false, "", ""},
		{"remote-empty-fails-closed", "", true, "", "remote nornicdb sources require --sha256"},
		{"valid", good, true, good, ""},
		{"uppercase-normalizes", strings.ToUpper(good), true, good, ""},
		{"short", "abcd", true, "", "invalid nornicdb source sha256"},
		{"non-hex", strings.Repeat("zz", 32), true, "", "invalid nornicdb source sha256"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeInstallSHA256(tc.raw, tc.required)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("normalize: %v", err)
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

func TestIsRemoteInstallSource(t *testing.T) {
	cases := []struct {
		ref  string
		want bool
	}{
		{"http://example.com/n.tar.gz", true},
		{"https://example.com/n.tar.gz", true},
		{"HTTP://example.com/n", true},
		{"/local/path/n", false},
		{"relative/n.tar.gz", false},
		{"file:///local/n.tar.gz", false},
		{"ftp://example.com/n", false},
	}
	for _, tc := range cases {
		if got := isRemoteInstallSource(tc.ref); got != tc.want {
			t.Errorf("isRemoteInstallSource(%q) = %v, want %v", tc.ref, got, tc.want)
		}
	}
}

func TestMaterializeRejectsUnsupportedScheme(t *testing.T) {
	_, _, _, err := materializeInstallSource(t.Context(), "ftp://example.com/n.tar.gz")
	if err == nil || !strings.Contains(err.Error(), "unsupported nornicdb source URL scheme") {
		t.Fatalf("got %v, want unsupported-scheme error", err)
	}
}

func TestMaterializeRejectsMissingAndDirectory(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")
	if _, _, _, err := materializeInstallSource(t.Context(), missing); err == nil {
		t.Fatal("want error for a missing source, got nil")
	}
	dir := t.TempDir()
	if _, _, _, err := materializeInstallSource(t.Context(), dir); err == nil || !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("got %v, want is-a-directory error", err)
	}
}

func TestPrepareLocalBinaryVerifiesVersion(t *testing.T) {
	dir := t.TempDir()
	bin := writeFakeBinary(t, dir, "nornicdb-headless", "1.0.0")
	prepared, err := prepareInstallSource(t.Context(), bin, "", fakeReader("9.9.9"))
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer func() { _ = prepared.Close() }()
	if prepared.SourceKind != sourceLocalBinary {
		t.Fatalf("kind = %q, want local-binary", prepared.SourceKind)
	}
	if prepared.Version != "9.9.9" {
		t.Fatalf("version = %q, want 9.9.9", prepared.Version)
	}
	if !prepared.Headless {
		t.Fatal("managed binary name must mark the source headless")
	}
	// The source checksum covers the file the reader verified.
	raw, err := os.ReadFile(bin)
	if err != nil {
		t.Fatalf("read fake: %v", err)
	}
	sum := sha256.Sum256(raw)
	if prepared.BinarySHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("binary sha = %q, want file digest", prepared.BinarySHA256)
	}
}

func TestPrepareRemoteRequiresSHA256(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("binary-bytes"))
	}))
	defer server.Close()
	if _, err := prepareInstallSource(t.Context(), server.URL+"/n", "", fakeReader("1.0.0")); err == nil ||
		!strings.Contains(err.Error(), "remote nornicdb sources require --sha256") {
		t.Fatalf("got %v, want fail-closed --sha256 error", err)
	}
}

func TestPrepareRemoteEnforcesSHA256(t *testing.T) {
	payload := []byte("downloaded-nornicdb")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	sum := sha256.Sum256(payload)
	good := hex.EncodeToString(sum[:])

	prepared, err := prepareInstallSource(t.Context(), server.URL+"/nornicdb-headless", good, fakeReader("2.0.0"))
	if err != nil {
		t.Fatalf("prepare with correct sha: %v", err)
	}
	_ = prepared.Close()
	if prepared.SourceKind != sourceDownloadedBinary {
		t.Fatalf("kind = %q, want downloaded-binary", prepared.SourceKind)
	}

	rotated := "00" + good[2:]
	if _, err := prepareInstallSource(t.Context(), server.URL+"/nornicdb-headless", rotated, fakeReader("2.0.0")); err == nil ||
		!strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("got %v, want sha256-mismatch error", err)
	}
}

func TestPrepareRemoteRejectsBadStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	sum := strings.Repeat("ab", 32)
	if _, err := prepareInstallSource(t.Context(), server.URL+"/n.tar.gz", sum, fakeReader("1.0")); err == nil ||
		!strings.Contains(err.Error(), "unexpected status") {
		t.Fatalf("got %v, want unexpected-status error", err)
	}
}

func writeTestArchive(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	archivePath := filepath.Join(dir, "nornicdb-headless.tar.gz")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatalf("write header: %v", err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatalf("write body: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}
	return archivePath
}

func TestPrepareArchiveExtractsAllowlistedBinary(t *testing.T) {
	dir := t.TempDir()
	archive := writeTestArchive(t, dir, map[string]string{
		"README.md":            "docs",
		"nested/nornicdb":      "#!/bin/sh\necho non-headless\n",
		"nornicdb-headless":    "#!/bin/sh\necho headless\n",
		"otherdir/evil-binary": "nope",
	})
	prepared, err := prepareInstallSource(t.Context(), archive, "", fakeReader("3.0.0"))
	if err != nil {
		t.Fatalf("prepare archive: %v", err)
	}
	defer func() { _ = prepared.Close() }()
	if prepared.SourceKind != sourceLocalArchive {
		t.Fatalf("kind = %q, want local-archive", prepared.SourceKind)
	}
	if filepath.Base(prepared.LocalBinaryPath) != "nornicdb-headless" {
		t.Fatalf("extracted %q, want the managed binary name", prepared.LocalBinaryPath)
	}
	if !prepared.Headless {
		t.Fatal("extracted managed binary must mark the source headless")
	}
}

func TestPrepareArchiveWithoutBinaryFails(t *testing.T) {
	dir := t.TempDir()
	archive := writeTestArchive(t, dir, map[string]string{"README.md": "docs"})
	if _, err := prepareInstallSource(t.Context(), archive, "", fakeReader("3.0.0")); err == nil ||
		!strings.Contains(err.Error(), "did not contain a usable NornicDB binary") {
		t.Fatalf("got %v, want missing-binary error", err)
	}
}

func TestPreparePackageIsDarwinOnly(t *testing.T) {
	// The source must exist: stat runs before the darwin gate, exactly as
	// in the core pipeline, so a missing path reports stat, not darwin.
	pkg := filepath.Join(t.TempDir(), "NornicDB.pkg")
	if err := os.WriteFile(pkg, []byte("pkg"), 0o644); err != nil {
		t.Fatalf("stage pkg: %v", err)
	}
	if _, err := prepareInstallSource(t.Context(), pkg, "", fakeReader("1.0")); err == nil ||
		!strings.Contains(err.Error(), "only supported on darwin") {
		t.Fatalf("got %v, want darwin-only error", err)
	}
}

func TestPrepareFileURLStripsScheme(t *testing.T) {
	dir := t.TempDir()
	bin := writeFakeBinary(t, dir, "nornicdb-headless", "1.0.0")
	prepared, err := prepareInstallSource(t.Context(), "file://"+bin, "", fakeReader("4.0.0"))
	if err != nil {
		t.Fatalf("prepare file:// source: %v", err)
	}
	_ = prepared.Close()
	if prepared.SourceKind != sourceLocalBinary {
		t.Fatalf("kind = %q, want local-binary", prepared.SourceKind)
	}
}

func TestPreparePropagatesVersionFailure(t *testing.T) {
	dir := t.TempDir()
	bin := writeFakeBinary(t, dir, "nornicdb-headless", "1.0.0")
	failing := VersionReader(func(binaryPath string) (string, error) { return "", errFakeVersion })
	if _, err := prepareInstallSource(t.Context(), bin, "", failing); err == nil ||
		!strings.Contains(err.Error(), "verify nornicdb source binary") {
		t.Fatalf("got %v, want verify error", err)
	}
}

var errFakeVersion = errSentinel("fake version failure")

type errSentinel string

func (e errSentinel) Error() string { return string(e) }
