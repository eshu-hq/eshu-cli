// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package graphinstall

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// tarWriterForTest and tarHeaderForTest build uncompressed .tar fixtures,
// which writeTestArchive (gzip-only) cannot express.
func tarWriterForTest(t *testing.T, w io.Writer) *tar.Writer {
	t.Helper()
	return tar.NewWriter(w)
}

func tarHeaderForTest(name string, isDir bool, sizes ...int64) *tar.Header {
	header := &tar.Header{Name: name, Mode: 0o755, Typeflag: tar.TypeReg}
	if isDir {
		header.Typeflag = tar.TypeDir
		return header
	}
	if len(sizes) > 0 {
		header.Size = sizes[0]
	}
	return header
}

func TestInstallDownloadTimeoutDefaultsAndParses(t *testing.T) {
	t.Setenv(installTimeoutEnv, "5s")
	got, err := installDownloadTimeout()
	if err != nil {
		t.Fatalf("valid timeout: %v", err)
	}
	if got != 5*time.Second {
		t.Fatalf("got %v, want 5s", got)
	}
	if err := os.Unsetenv(installTimeoutEnv); err != nil {
		t.Fatalf("unset timeout env: %v", err)
	}
	got, err = installDownloadTimeout()
	if err != nil {
		t.Fatalf("default timeout: %v", err)
	}
	if got != 30*time.Second {
		t.Fatalf("got %v, want the 30s default", got)
	}
}

func downloadTestServer(t *testing.T, body string, contentLength string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if contentLength != "" {
			w.Header().Set("Content-Length", contentLength)
		}
		_, _ = w.Write([]byte(body))
	}))
}

func TestDownloadInstallSourceFailureSequence(t *testing.T) {
	ctx := t.Context()
	if _, err := downloadInstallSource(nil, downloadTestServer(t, "bytes", "").URL+"/n"); err != nil {
		t.Fatalf("nil context must default to Background: %v", err)
	}
	t.Setenv(installTimeoutEnv, "not-a-duration")
	if _, err := downloadInstallSource(ctx, "https://example.com/n"); err == nil ||
		!strings.Contains(err.Error(), "parse "+installTimeoutEnv) {
		t.Fatalf("got %v, want timeout error", err)
	}
	t.Setenv(installTimeoutEnv, "")
	if _, err := downloadInstallSource(ctx, "://bad-url"); err == nil ||
		!strings.Contains(err.Error(), "build nornicdb download request") {
		t.Fatalf("got %v, want request-build error", err)
	}
	// Nothing listens on port 1, so the dial fails closed.
	if _, err := downloadInstallSource(ctx, "http://127.0.0.1:1/nornicdb"); err == nil ||
		!strings.Contains(err.Error(), "download nornicdb source") {
		t.Fatalf("got %v, want download error", err)
	}
	badStatus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer badStatus.Close()
	if _, err := downloadInstallSource(ctx, badStatus.URL+"/n"); err == nil ||
		!strings.Contains(err.Error(), "unexpected status") {
		t.Fatalf("got %v, want status error", err)
	}
}

func TestDownloadInstallSourceStagesFileFailures(t *testing.T) {
	ctx := t.Context()
	realTMP := os.TempDir()
	// A TMPDIR naming a regular file breaks the staging MkdirTemp after a
	// successful 200 response.
	server := downloadTestServer(t, "payload", "")
	defer server.Close()
	fileTMP := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(fileTMP, []byte("x"), 0o644); err != nil {
		t.Fatalf("stage TMPDIR file: %v", err)
	}
	t.Setenv("TMPDIR", fileTMP)
	if _, err := downloadInstallSource(ctx, server.URL+"/n"); err == nil ||
		!strings.Contains(err.Error(), "create nornicdb source temp directory") {
		t.Fatalf("got %v, want temp-directory error", err)
	}
	t.Setenv("TMPDIR", realTMP)
	// A 300-byte URL basename exceeds the filesystem name limit, breaking
	// the staged-file create while the transfer itself succeeds.
	longName := strings.Repeat("a", 300)
	if _, err := downloadInstallSource(ctx, server.URL+"/"+longName); err == nil ||
		!strings.Contains(err.Error(), "create downloaded nornicdb source file") {
		t.Fatalf("got %v, want staged-file error", err)
	}
	// Declaring 100000 bytes but sending 5 breaks the body copy
	// mid-sequence: the staged tree is removed and no path leaks.
	truncated := downloadTestServer(t, "short", "100000")
	defer truncated.Close()
	if _, err := downloadInstallSource(ctx, truncated.URL+"/n"); err == nil ||
		!strings.Contains(err.Error(), "download nornicdb source body") {
		t.Fatalf("got %v, want body-copy error", err)
	}
}

func TestDownloadInstallSourceFallsBackToDefaultName(t *testing.T) {
	server := downloadTestServer(t, "payload", "")
	defer server.Close()
	got, err := downloadInstallSource(t.Context(), server.URL+"/")
	if err != nil {
		t.Fatalf("root-path download: %v", err)
	}
	defer func() { _ = os.RemoveAll(filepath.Dir(got)) }()
	if filepath.Base(got) != "nornicdb-source" {
		t.Fatalf("staged %q, want the nornicdb-source fallback name", got)
	}
}

func TestDownloadInstallSourceRoundTripVerifiesChecksum(t *testing.T) {
	payload := []byte("downloaded-nornicdb-binary")
	server := downloadTestServer(t, string(payload), "")
	defer server.Close()
	sum := sha256.Sum256(payload)
	prepared, err := prepareInstallSource(t.Context(), server.URL+"/nornicdb-headless", hex.EncodeToString(sum[:]), fakeReader("6.0.0"))
	if err != nil {
		t.Fatalf("prepare downloaded binary: %v", err)
	}
	_ = prepared.Close()
	if prepared.SourceSHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("source sha = %q, want the served digest", prepared.SourceSHA256)
	}
}

func writeRawTar(t *testing.T, dir, name string, files map[string]string, dirEntries []string) string {
	t.Helper()
	archivePath := filepath.Join(dir, name)
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	tw := tarWriterForTest(t, f)
	for _, entry := range dirEntries {
		if err := tw.WriteHeader(tarHeaderForTest(entry, true)); err != nil {
			t.Fatalf("write dir header: %v", err)
		}
	}
	for fname, body := range files {
		if err := tw.WriteHeader(tarHeaderForTest(fname, false, int64(len(body)))); err != nil {
			t.Fatalf("write file header: %v", err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatalf("write file body: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}
	return archivePath
}

func TestExtractBinaryFromArchiveFailureSequence(t *testing.T) {
	dir := t.TempDir()
	if _, _, _, err := extractBinaryFromArchive(filepath.Join(dir, "absent.tar.gz")); err == nil ||
		!strings.Contains(err.Error(), "open nornicdb archive") {
		t.Fatalf("got %v, want open error", err)
	}
	badGzip := filepath.Join(dir, "bad.tar.gz")
	if err := os.WriteFile(badGzip, []byte("not gzip at all"), 0o644); err != nil {
		t.Fatalf("stage bad gzip: %v", err)
	}
	if _, _, _, err := extractBinaryFromArchive(badGzip); err == nil ||
		!strings.Contains(err.Error(), "open gzip nornicdb archive") {
		t.Fatalf("got %v, want gzip error", err)
	}
	notArchive := filepath.Join(dir, "n.zip")
	if err := os.WriteFile(notArchive, []byte("zip"), 0o644); err != nil {
		t.Fatalf("stage zip: %v", err)
	}
	if _, _, _, err := extractBinaryFromArchive(notArchive); err == nil ||
		!strings.Contains(err.Error(), "not a supported archive") {
		t.Fatalf("got %v, want unsupported-suffix error", err)
	}
	corrupt := filepath.Join(dir, "corrupt.tar")
	if err := os.WriteFile(corrupt, []byte("this is not a tar stream, far too short"), 0o644); err != nil {
		t.Fatalf("stage corrupt tar: %v", err)
	}
	if _, _, _, err := extractBinaryFromArchive(corrupt); err == nil ||
		!strings.Contains(err.Error(), "read nornicdb archive") {
		t.Fatalf("got %v, want tar-read error", err)
	}
	// A header promising more bytes than the member holds breaks the
	// mid-sequence copy with an unexpected EOF.
	lying := filepath.Join(dir, "lying.tar")
	lfile, err := os.Create(lying)
	if err != nil {
		t.Fatalf("create lying archive: %v", err)
	}
	ltw := tarWriterForTest(t, lfile)
	if err := ltw.WriteHeader(tarHeaderForTest("nornicdb-headless", false, 1<<20)); err != nil {
		t.Fatalf("write lying header: %v", err)
	}
	if _, err := ltw.Write([]byte("short")); err != nil {
		t.Fatalf("write lying body: %v", err)
	}
	// Close must complain about the short member; the truncated stream on
	// disk is exactly the fixture under test.
	if err := ltw.Close(); err == nil {
		t.Fatal("lying tar closed cleanly, want a short-write error")
	}
	if err := lfile.Close(); err != nil {
		t.Fatalf("close lying file: %v", err)
	}
	if _, _, _, err := extractBinaryFromArchive(lying); err == nil ||
		!strings.Contains(err.Error(), "extract nornicdb binary from archive") {
		t.Fatalf("got %v, want extract-copy error", err)
	}
}

func TestExtractBinaryFromArchiveReadsPlainTarAndSkipsDirs(t *testing.T) {
	dir := t.TempDir()
	plain := writeRawTar(t, dir, "n.tar", map[string]string{
		"nornicdb-headless": "#!/bin/sh\necho headless\n",
	}, []string{"somedir"})
	path, name, cleanup, err := extractBinaryFromArchive(plain)
	if err != nil {
		t.Fatalf("extract plain tar: %v", err)
	}
	defer func() { _ = cleanup() }()
	if name != "nornicdb-headless" {
		t.Fatalf("name = %q, want the headless binary", name)
	}
	if filepath.Base(path) != "nornicdb-headless" {
		t.Fatalf("extracted %q, want the headless binary", path)
	}
}

func TestExtractBinaryFromArchiveTempDirFailure(t *testing.T) {
	fileTMP := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(fileTMP, []byte("x"), 0o644); err != nil {
		t.Fatalf("stage TMPDIR file: %v", err)
	}
	plain := writeRawTar(t, t.TempDir(), "n.tar", map[string]string{"nornicdb-headless": "bin"}, nil)
	t.Setenv("TMPDIR", fileTMP)
	if _, _, _, err := extractBinaryFromArchive(plain); err == nil ||
		!strings.Contains(err.Error(), "create archive extraction directory") {
		t.Fatalf("got %v, want extraction-directory error", err)
	}
}
