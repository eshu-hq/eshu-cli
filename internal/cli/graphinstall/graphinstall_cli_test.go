// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package graphinstall

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cliManifest builds a pinned manifest holding one release for cliVersion
// with a single asset, so the resolve tests pin matching without fixtures.
func cliManifest(cliVersion, assetOS, assetArch string, headless bool, url, sha string) pinnedReleaseManifest {
	return pinnedReleaseManifest{
		Version: 1,
		Backend: "nornicdb",
		Releases: []pinnedReleaseSpec{{
			CLIVersion: cliVersion,
			ReleaseTag: "test-tag",
			Assets:     []pinnedReleaseAsset{{OS: assetOS, Arch: assetArch, Headless: headless, URL: url, SHA256: sha}},
		}},
	}
}

func TestDecodeReleaseManifestRejectsBadShapes(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{"malformed-json", `{"version":`, "decode pinned NornicDB release manifest"},
		{"missing-version", `{"version":0,"backend":"nornicdb","releases":[]}`, "missing version"},
		{"wrong-backend", `{"version":1,"backend":"other","releases":[]}`, "unexpected backend"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := decodeReleaseManifest([]byte(tc.raw)); err == nil ||
				!strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("got %v, want error containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestResolvePinnedReleaseSourceMatchesAndSkips(t *testing.T) {
	goodSHA := strings.Repeat("ab", 32)
	manifest := pinnedReleaseManifest{
		Version: 1,
		Backend: "nornicdb",
		Releases: []pinnedReleaseSpec{
			{CLIVersion: "other", ReleaseTag: "old", Assets: []pinnedReleaseAsset{
				{OS: "linux", Arch: "arm64", Headless: true, URL: "https://example.com/old", SHA256: goodSHA},
			}},
			{CLIVersion: "9.9.9", ReleaseTag: "new", Assets: []pinnedReleaseAsset{
				{OS: "linux", Arch: "arm64", Headless: true, URL: "https://example.com/n-headless", SHA256: goodSHA},
				{OS: "linux", Arch: "arm64", Headless: false, URL: "https://example.com/n-full", SHA256: goodSHA},
			}},
		},
	}
	url, sha, err := resolvePinnedReleaseSource(manifest, "9.9.9", "linux", "arm64", true)
	if err != nil {
		t.Fatalf("headless resolve: %v", err)
	}
	if url != "https://example.com/n-headless" || sha != goodSHA {
		t.Fatalf("got %q %q, want the headless asset", url, sha)
	}
	url, _, err = resolvePinnedReleaseSource(manifest, "9.9.9", "linux", "arm64", false)
	if err != nil {
		t.Fatalf("full resolve: %v", err)
	}
	if url != "https://example.com/n-full" {
		t.Fatalf("got %q, want the full asset", url)
	}
}

func TestResolvePinnedReleaseSourceDefaultsEmptyCLIToDev(t *testing.T) {
	// An empty CLI version resolves as "dev" before the release scan.
	manifest := cliManifest("dev", "linux", "arm64", true, "https://example.com/d", strings.Repeat("cd", 32))
	url, _, err := resolvePinnedReleaseSource(manifest, "   ", "linux", "arm64", true)
	if err != nil {
		t.Fatalf("dev-default resolve: %v", err)
	}
	if url != "https://example.com/d" {
		t.Fatalf("got %q, want the dev asset", url)
	}
}

func TestResolvePinnedReleaseSourceRefusesIncompleteAsset(t *testing.T) {
	manifest := cliManifest("9.9.9", "linux", "arm64", true, "", "")
	if _, _, err := resolvePinnedReleaseSource(manifest, "9.9.9", "linux", "arm64", true); err == nil ||
		!strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("got %v, want incomplete-asset error", err)
	}
}

func TestResolvePinnedReleaseSourceMissingAssetNamesKind(t *testing.T) {
	manifest := cliManifest("9.9.9", "linux", "arm64", true, "https://example.com/n", strings.Repeat("ab", 32))
	if _, _, err := resolvePinnedReleaseSource(manifest, "9.9.9", "darwin", "arm64", true); err == nil ||
		!strings.Contains(err.Error(), "no embedded headless NornicDB release asset") {
		t.Fatalf("got %v, want headless-missing error", err)
	}
	if _, _, err := resolvePinnedReleaseSource(manifest, "9.9.9", "darwin", "arm64", false); err == nil ||
		!strings.Contains(err.Error(), "no embedded full NornicDB release asset") {
		t.Fatalf("got %v, want full-missing error", err)
	}
	empty := pinnedReleaseManifest{Version: 1, Backend: "nornicdb"}
	if _, _, err := resolvePinnedReleaseSource(empty, "9.9.9", "linux", "arm64", false); err == nil ||
		!strings.Contains(err.Error(), "no embedded full NornicDB release asset") {
		t.Fatalf("got %v, want full-missing error on the empty manifest", err)
	}
}

func TestMissingPinnedReleaseAssetErrorHintsFullSource(t *testing.T) {
	err := missingPinnedReleaseAssetError("full", "9.9.9", "linux", "arm64")
	if !strings.Contains(err.Error(), "<path-to-nornicdb>") {
		t.Fatalf("got %v, want the full-source hint", err)
	}
}

func TestResolveHomeDirPropagatesEveryHomeError(t *testing.T) {
	failing := func() (string, error) { return "", errors.New("no home") }
	if _, err := resolveHomeDir(homeEnv(), failing, "windows"); err == nil {
		t.Fatal("want error for the windows profile fallback, got nil")
	}
	if _, err := resolveHomeDir(homeEnv("XDG_DATA_HOME", "~/x"), failing, "linux"); err == nil {
		t.Fatal("want error for the XDG ~ expansion, got nil")
	}
	if _, err := resolveHomeDir(homeEnv(), failing, "linux"); err == nil {
		t.Fatal("want error for the linux home fallback, got nil")
	}
}

// TestParseMissingVersionArmIsUnreachable proves the "missing version"
// return is dead: TrimSpace runs before the prefix check, so every string
// carrying the "NornicDB " prefix still carries a non-space byte after it.
// Each near-miss below must report "unexpected output", never "missing
// version".
func TestParseMissingVersionArmIsUnreachable(t *testing.T) {
	for _, input := range []string{"NornicDB ", "NornicDB   ", "NornicDB\t", "NornicDB \t "} {
		_, err := parseNornicDBVersionOutput(input)
		if err == nil || !strings.Contains(err.Error(), "unexpected output") {
			t.Fatalf("parseNornicDBVersionOutput(%q) = %v, want unexpected-output error", input, err)
		}
		if strings.Contains(err.Error(), "missing version") {
			t.Fatalf("parseNornicDBVersionOutput(%q) reached the missing-version arm", input)
		}
	}
}

// TestDefaultReleaseManifestAlwaysDecodes proves the Install
// defaultReleaseManifest error arm cannot fire with the shipped manifest:
// the embedded bytes always decode, so the decode failures stay pinned on
// decodeReleaseManifest directly.
func TestDefaultReleaseManifestAlwaysDecodes(t *testing.T) {
	if _, err := defaultReleaseManifest(); err != nil {
		t.Fatalf("embedded manifest must always decode: %v", err)
	}
}

func TestInstallRejectsBadSourceChecksum(t *testing.T) {
	isolatedHome(t)
	bin := writeFakeBinary(t, t.TempDir(), "nornicdb-headless", "1.0.0")
	_, err := Install(Options{From: bin, SHA256: "zz", ReadVersion: fakeReader("1.0.0")})
	if err == nil || !strings.Contains(err.Error(), "invalid nornicdb source sha256") {
		t.Fatalf("got %v, want invalid-checksum error", err)
	}
}

func TestInstallFailsWhenHomeUnresolvable(t *testing.T) {
	// ESHU_HOME "~" expands against the user home; with HOME empty the
	// managed-home seam fails before any install work.
	t.Setenv("ESHU_HOME", "~")
	t.Setenv("HOME", "")
	bin := writeFakeBinary(t, t.TempDir(), "nornicdb-headless", "1.0.0")
	_, err := Install(Options{From: bin, ReadVersion: fakeReader("1.0.0")})
	if err == nil || !strings.Contains(err.Error(), "resolve user home") {
		t.Fatalf("got %v, want home-resolution error", err)
	}
}

func TestManagedPathsFailWhenHomeUnresolvable(t *testing.T) {
	t.Setenv("ESHU_HOME", "~")
	t.Setenv("HOME", "")
	if _, err := managedBinaryPath(); err == nil {
		t.Fatal("managedBinaryPath() = nil, want home error")
	}
	if _, err := installManifestPath(); err == nil {
		t.Fatal("installManifestPath() = nil, want home error")
	}
	if _, err := ManagedBinaryIfPresent(fakeReader("1.0")); err == nil {
		t.Fatal("ManagedBinaryIfPresent() = nil, want home error")
	}
}

func TestManagedBinaryIfPresentRejectsNilReader(t *testing.T) {
	isolatedHome(t)
	if _, err := ManagedBinaryIfPresent(nil); err == nil ||
		!strings.Contains(err.Error(), "non-nil VersionReader") {
		t.Fatalf("got %v, want nil-reader error", err)
	}
}

func TestInstallFailsWhenTargetStatIsNotNotExist(t *testing.T) {
	// ESHU_HOME naming a regular file makes the managed-binary stat fail
	// with ENOTDIR, which is neither success nor NotExist.
	homeFile := filepath.Join(t.TempDir(), "home-file")
	if err := os.WriteFile(homeFile, []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("stage home file: %v", err)
	}
	t.Setenv("ESHU_HOME", homeFile)
	bin := writeFakeBinary(t, t.TempDir(), "nornicdb-headless", "1.0.0")
	_, err := Install(Options{From: bin, ReadVersion: fakeReader("1.0.0")})
	if err == nil || !strings.Contains(err.Error(), "stat managed nornicdb binary") {
		t.Fatalf("got %v, want stat error", err)
	}
}

func TestInstallReportsCopyFailure(t *testing.T) {
	home := isolatedHome(t)
	if err := os.Chmod(home, 0o555); err != nil {
		t.Fatalf("chmod home: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(home, 0o755) })
	bin := writeFakeBinary(t, t.TempDir(), "nornicdb-headless", "1.0.0")
	_, err := Install(Options{From: bin, ReadVersion: fakeReader("1.0.0")})
	if err == nil || !strings.Contains(err.Error(), "create nornicdb binary directory") {
		t.Fatalf("got %v, want binary-directory error", err)
	}
}

func TestInstallReportsManifestWriteFailure(t *testing.T) {
	home := isolatedHome(t)
	bin := writeFakeBinary(t, t.TempDir(), "nornicdb-headless", "1.0.0")
	// A file where graph-backends must be breaks the manifest write while
	// the binary copy itself still succeeds.
	if err := os.WriteFile(filepath.Join(home, "graph-backends"), []byte("blocker"), 0o644); err != nil {
		t.Fatalf("stage blocker: %v", err)
	}
	_, err := Install(Options{From: bin, ReadVersion: fakeReader("1.0.0")})
	if err == nil || !strings.Contains(err.Error(), "create directory for") {
		t.Fatalf("got %v, want manifest-directory error", err)
	}
}

func TestInstallRefusesWhenExistingProbeFails(t *testing.T) {
	isolatedHome(t)
	srcDir := t.TempDir()
	bin := writeFakeBinary(t, srcDir, "nornicdb-headless", "1.0.0")
	managed := filepath.Join(os.Getenv("ESHU_HOME"), "bin", "nornicdb-headless")
	reader := VersionReader(func(path string) (string, error) {
		if path == managed {
			return "", errFakeVersion
		}
		return "1.0.0", nil
	})
	if _, err := Install(Options{From: bin, ReadVersion: reader}); err != nil {
		t.Fatalf("first install: %v", err)
	}
	// The existing binary no longer probes, so reuse is impossible and the
	// install must refuse without --force.
	if _, err := Install(Options{From: bin, ReadVersion: reader}); err == nil ||
		!strings.Contains(err.Error(), "already exists") {
		t.Fatalf("got %v, want exists error", err)
	}
}

func TestInstallRefusesWhenExistingChecksumFails(t *testing.T) {
	home := isolatedHome(t)
	srcDir := t.TempDir()
	bin := writeFakeBinary(t, srcDir, "nornicdb-headless", "1.0.0")
	if _, err := Install(Options{From: bin, ReadVersion: fakeReader("1.0.0")}); err != nil {
		t.Fatalf("first install: %v", err)
	}
	managed := filepath.Join(home, "bin", "nornicdb-headless")
	if err := os.Chmod(managed, 0); err != nil {
		t.Fatalf("chmod managed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(managed, 0o755) })
	if _, err := Install(Options{From: bin, ReadVersion: fakeReader("1.0.0")}); err == nil ||
		!strings.Contains(err.Error(), "already exists") {
		t.Fatalf("got %v, want exists error", err)
	}
}

// TestWriteInstallManifestMarshalIsInfallible proves the json.MarshalIndent
// error arm cannot fire: the manifest is a fixed string/bool struct, so
// marshaling never fails and the atomic-write failure stays the tested arm.
func TestWriteInstallManifestMarshalIsInfallible(t *testing.T) {
	source := preparedInstallSource{SourceKind: sourceLocalBinary, Version: "1.0.0", Headless: true}
	if _, err := writeInstallManifest(
		filepath.Join(t.TempDir(), "nornicdb-headless"),
		filepath.Join(t.TempDir(), "manifest.json"),
		source, false,
	); err != nil {
		t.Fatalf("writeInstallManifest() = %v, want nil (marshal cannot fail)", err)
	}
}

func TestWriteInstallManifestReportsAtomicWriteFailure(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("stage blocker: %v", err)
	}
	source := preparedInstallSource{SourceKind: sourceLocalBinary, Version: "1.0.0"}
	_, err := writeInstallManifest("target", filepath.Join(blocker, "manifest.json"), source, false)
	if err == nil || !strings.Contains(err.Error(), "create directory for") {
		t.Fatalf("got %v, want atomic-write error", err)
	}
}

func TestSHA256FileReportsOpenAndReadFailures(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent")
	if _, err := sha256File(missing); err == nil || !strings.Contains(err.Error(), "open") {
		t.Fatalf("got %v, want open error", err)
	}
	// A directory opens fine but fails the streaming read.
	if _, err := sha256File(t.TempDir()); err == nil || !strings.Contains(err.Error(), "hash") {
		t.Fatalf("got %v, want hash error", err)
	}
}

func TestCopyExecutableFileReportsEarlyFailures(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "absent")
	if err := copyExecutableFile(missing, filepath.Join(dir, "target")); err == nil ||
		!strings.Contains(err.Error(), "open nornicdb source binary") {
		t.Fatalf("got %v, want source-open error", err)
	}
	src := writeFakeBinary(t, dir, "src", "1.0.0")
	// A directory opens fine as a copy source but fails the streaming read,
	// pinning the mid-sequence copy arm.
	if err := copyExecutableFile(dir, filepath.Join(t.TempDir(), "target")); err == nil ||
		!strings.Contains(err.Error(), "copy nornicdb binary") {
		t.Fatalf("got %v, want copy error", err)
	}
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("stage blocker: %v", err)
	}
	if err := copyExecutableFile(src, filepath.Join(blocker, "target")); err == nil ||
		!strings.Contains(err.Error(), "create nornicdb binary directory") {
		t.Fatalf("got %v, want directory error", err)
	}
}

func TestCopyExecutableFileReportsTempAndRenameFailures(t *testing.T) {
	dir := t.TempDir()
	src := writeFakeBinary(t, dir, "src", "1.0.0")
	roDir := filepath.Join(dir, "ro")
	if err := os.Mkdir(roDir, 0o755); err != nil {
		t.Fatalf("stage ro dir: %v", err)
	}
	if err := os.Chmod(roDir, 0o555); err != nil {
		t.Fatalf("chmod ro dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(roDir, 0o755) })
	if err := copyExecutableFile(src, filepath.Join(roDir, "target")); err == nil ||
		!strings.Contains(err.Error(), "create nornicdb binary temp file") {
		t.Fatalf("got %v, want temp-file error", err)
	}
	// Renaming a staged file onto a non-empty directory fails, pinning the
	// replace arm without faulting the earlier sequence.
	occupied := filepath.Join(dir, "occupied")
	if err := os.Mkdir(occupied, 0o755); err != nil {
		t.Fatalf("stage occupied dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(occupied, "kept"), []byte("kept"), 0o644); err != nil {
		t.Fatalf("stage kept file: %v", err)
	}
	if err := copyExecutableFile(src, occupied); err == nil ||
		!strings.Contains(err.Error(), "replace managed nornicdb binary") {
		t.Fatalf("got %v, want replace error", err)
	}
}

func TestAtomicWriteFileReportsEarlyAndRenameFailures(t *testing.T) {
	dir := t.TempDir()
	content := []byte("{}\n")
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("stage blocker: %v", err)
	}
	if err := atomicWriteFile(filepath.Join(blocker, "m.json"), content, 0o600); err == nil ||
		!strings.Contains(err.Error(), "create directory for") {
		t.Fatalf("got %v, want directory error", err)
	}
	roDir := filepath.Join(dir, "ro")
	if err := os.Mkdir(roDir, 0o755); err != nil {
		t.Fatalf("stage ro dir: %v", err)
	}
	if err := os.Chmod(roDir, 0o555); err != nil {
		t.Fatalf("chmod ro dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(roDir, 0o755) })
	if err := atomicWriteFile(filepath.Join(roDir, "m.json"), content, 0o600); err == nil ||
		!strings.Contains(err.Error(), "create temp file for") {
		t.Fatalf("got %v, want temp-file error", err)
	}
	occupied := filepath.Join(dir, "occupied")
	if err := os.Mkdir(occupied, 0o755); err != nil {
		t.Fatalf("stage occupied dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(occupied, "kept"), []byte("kept"), 0o644); err != nil {
		t.Fatalf("stage kept file: %v", err)
	}
	if err := atomicWriteFile(occupied, content, 0o600); err == nil ||
		!strings.Contains(err.Error(), "replace") {
		t.Fatalf("got %v, want replace error", err)
	}
}

func TestSamePathFallsBackWhenAbsFails(t *testing.T) {
	// filepath.Abs fails only when the working directory is gone; deleting
	// the test's own cwd forces both Abs calls to fail, pinning the
	// verbatim fallback. t.Chdir restores the original directory.
	victim := filepath.Join(t.TempDir(), "gone")
	if err := os.Mkdir(victim, 0o755); err != nil {
		t.Fatalf("stage victim dir: %v", err)
	}
	t.Chdir(victim)
	if err := os.Remove(victim); err != nil {
		t.Fatalf("remove cwd: %v", err)
	}
	if !samePath("a", "a") {
		t.Fatal("samePath(a, a) with failing Abs = false, want verbatim true")
	}
	if samePath("a", "b") {
		t.Fatal("samePath(a, b) with failing Abs = true, want verbatim false")
	}
}

func TestPreparedSourceCloseAndCleanupJoin(t *testing.T) {
	if err := (preparedInstallSource{}).Close(); err != nil {
		t.Fatalf("Close(nil cleanup) = %v, want nil", err)
	}
	joinErr := errors.New("cleanup boom")
	joined := combineInstallCleanups(nil, func() error { return joinErr }, nil)
	if err := joined(); !errors.Is(err, joinErr) {
		t.Fatalf("joined cleanup = %v, want the cleanup error", err)
	}
	if err := combineInstallCleanups(nil, nil)(); err != nil {
		t.Fatalf("all-nil cleanup = %v, want nil", err)
	}
}
