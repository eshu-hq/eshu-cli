// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package graphinstall

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// chmodDenyReader probes the version, then revokes all access to the exact
// path it just probed. The install pipeline hashes the binary after the
// probe with no hook in between, so this is the only way to force the
// post-probe sha256 failure arms.
func chmodDenyReader(t *testing.T) VersionReader {
	t.Helper()
	return func(binaryPath string) (string, error) {
		if err := os.Chmod(binaryPath, 0); err != nil {
			return "", err
		}
		t.Cleanup(func() { _ = os.Chmod(binaryPath, 0o755) })
		return "1.0.0", nil
	}
}

func TestIsRemoteInstallSourceRejectsUnparseable(t *testing.T) {
	if isRemoteInstallSource("http://example.com/\x7f") {
		t.Fatal("isRemoteInstallSource(control char) = true, want false (parse error)")
	}
}

func TestMaterializeClassifiesDownloadedKinds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("payload"))
	}))
	defer server.Close()
	for _, tc := range []struct {
		suffix string
		want   installSourceKind
	}{
		{"/n.tar.gz", sourceDownloadedArchive},
		{"/n.tgz", sourceDownloadedArchive},
		{"/NornicDB.pkg", sourceDownloadedPackage},
		{"/nornicdb-headless", sourceDownloadedBinary},
	} {
		_, kind, cleanup, err := materializeInstallSource(t.Context(), server.URL+tc.suffix)
		if err != nil {
			t.Fatalf("materialize %q: %v", tc.suffix, err)
		}
		_ = cleanup()
		if kind != tc.want {
			t.Fatalf("kind(%q) = %q, want %q", tc.suffix, kind, tc.want)
		}
	}
}

func TestMaterializeAbsFailureIsUnreachableExceptWithoutCwd(t *testing.T) {
	// filepath.Abs fails only when the working directory is gone, so the
	// materialize resolve arm needs the same deleted-cwd forcing as
	// samePath. Any other relative source resolves.
	victim := filepath.Join(t.TempDir(), "gone")
	if err := os.Mkdir(victim, 0o755); err != nil {
		t.Fatalf("stage victim dir: %v", err)
	}
	t.Chdir(victim)
	if err := os.Remove(victim); err != nil {
		t.Fatalf("remove cwd: %v", err)
	}
	if _, _, _, err := materializeInstallSource(t.Context(), "relative/path"); err == nil ||
		!strings.Contains(err.Error(), "resolve nornicdb source path") {
		t.Fatalf("got %v, want resolve error without a cwd", err)
	}
}

func TestPrepareFailsWhenSourceUnreadableAfterStat(t *testing.T) {
	dir := t.TempDir()
	bin := writeFakeBinary(t, dir, "nornicdb-headless", "1.0.0")
	if err := os.Chmod(bin, 0); err != nil {
		t.Fatalf("chmod source: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(bin, 0o755) })
	// Stat succeeds (directory search is intact) but the content hash
	// cannot open the file.
	if _, err := prepareInstallSource(t.Context(), bin, "", fakeReader("1.0.0")); err == nil ||
		!strings.Contains(err.Error(), "for sha256") {
		t.Fatalf("got %v, want sha256-open error", err)
	}
}

func TestPrepareLocalBinaryFailsPostProbeHash(t *testing.T) {
	dir := t.TempDir()
	bin := writeFakeBinary(t, dir, "nornicdb-headless", "1.0.0")
	if _, err := prepareInstallSource(t.Context(), bin, "", chmodDenyReader(t)); err == nil ||
		!strings.Contains(err.Error(), "for sha256") {
		t.Fatalf("got %v, want post-probe hash error", err)
	}
}

func TestPrepareArchiveFailsVersionAndPostExtractHash(t *testing.T) {
	dir := t.TempDir()
	archive := writeTestArchive(t, dir, map[string]string{"nornicdb-headless": "#!/bin/sh\necho headless\n"})
	failing := VersionReader(func(binaryPath string) (string, error) { return "", errFakeVersion })
	if _, err := prepareInstallSource(t.Context(), archive, "", failing); err == nil ||
		!strings.Contains(err.Error(), "verify nornicdb source binary") {
		t.Fatalf("got %v, want verify error", err)
	}
	if _, err := prepareInstallSource(t.Context(), archive, "", chmodDenyReader(t)); err == nil ||
		!strings.Contains(err.Error(), "for sha256") {
		t.Fatalf("got %v, want post-extract hash error", err)
	}
}

// stubDarwinExpansion replaces the pkgutil seam with a fake that stages an
// expanded tree holding one small binary of the given name, and forces the
// darwin gate on Linux. The pre-expansion bound still runs against the real
// artifact and the real temp volume.
func stubDarwinExpansion(t *testing.T, name string) {
	t.Helper()
	oldGOOS, oldExpand := packageGOOS, expandPackage
	packageGOOS = "darwin"
	expandPackage = func(pkgPath, targetDir string) error {
		binDir := filepath.Join(targetDir, "comp", "Payload", "usr", "local", "bin")
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(binDir, name), []byte("fake"), 0o755)
	}
	t.Cleanup(func() { packageGOOS, expandPackage = oldGOOS, oldExpand })
}

func stageSmallPackage(t *testing.T) string {
	t.Helper()
	pkg := filepath.Join(t.TempDir(), "NornicDB.pkg")
	if err := os.WriteFile(pkg, []byte("pkg"), 0o644); err != nil {
		t.Fatalf("stage pkg: %v", err)
	}
	return pkg
}

func TestPreparePackageDarwinSuccessAndFailures(t *testing.T) {
	stubDarwinExpansion(t, managedBinaryName)
	prepared, err := prepareInstallSource(t.Context(), stageSmallPackage(t), "", fakeReader("4.0.0"))
	if err != nil {
		t.Fatalf("prepare package: %v", err)
	}
	_ = prepared.Close()
	if prepared.SourceKind != sourceLocalPackage {
		t.Fatalf("kind = %q, want local-package", prepared.SourceKind)
	}
	if !prepared.Headless {
		t.Fatal("managed binary name must mark the package source headless")
	}
	failing := VersionReader(func(binaryPath string) (string, error) { return "", errFakeVersion })
	if _, err := prepareInstallSource(t.Context(), stageSmallPackage(t), "", failing); err == nil ||
		!strings.Contains(err.Error(), "verify nornicdb source binary") {
		t.Fatalf("got %v, want verify error", err)
	}
	if _, err := prepareInstallSource(t.Context(), stageSmallPackage(t), "", chmodDenyReader(t)); err == nil ||
		!strings.Contains(err.Error(), "for sha256") {
		t.Fatalf("got %v, want post-extract hash error", err)
	}
}

func TestExtractBinaryFromPackageReportsExpandAndTempFailures(t *testing.T) {
	oldGOOS, oldExpand := packageGOOS, expandPackage
	packageGOOS = "darwin"
	expandPackage = func(pkgPath, targetDir string) error { return errFakeVersion }
	t.Cleanup(func() { packageGOOS, expandPackage = oldGOOS, oldExpand })
	// The pre-expansion stat runs before the stubbed subprocess: a missing
	// artifact reports stat, never expansion.
	if _, _, _, err := extractBinaryFromPackage(filepath.Join(t.TempDir(), "absent.pkg")); err == nil ||
		!strings.Contains(err.Error(), "stat nornicdb package") {
		t.Fatalf("got %v, want stat error", err)
	}
	pkg := stageSmallPackage(t)
	if _, _, cleanup, err := extractBinaryFromPackage(pkg); err == nil ||
		!strings.Contains(err.Error(), "fake version failure") {
		t.Fatalf("got %v, want expansion error", err)
	} else if cleanup != nil {
		t.Fatal("failed expansion must not return a cleanup")
	}
	if _, err := os.Stat(pkg + ".extracted"); !os.IsNotExist(err) {
		t.Fatal("failed expansion left residue beside the package")
	}
	// A TMPDIR naming a regular file breaks the extraction MkdirTemp while
	// the pre-expansion bound (real stat, real free space) still passes.
	fileTMP := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(fileTMP, []byte("x"), 0o644); err != nil {
		t.Fatalf("stage TMPDIR file: %v", err)
	}
	t.Setenv("TMPDIR", fileTMP)
	if _, _, _, err := extractBinaryFromPackage(pkg); err == nil ||
		!strings.Contains(err.Error(), "create package extraction directory") {
		t.Fatalf("got %v, want extraction-directory error", err)
	}
}

func TestExtractBinaryFromPackageSucceedsOnDarwinStub(t *testing.T) {
	stubDarwinExpansion(t, managedBinaryName)
	pkg := stageSmallPackage(t)
	chosen, name, cleanup, err := extractBinaryFromPackage(pkg)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if name != managedBinaryName {
		t.Fatalf("name = %q, want %q", name, managedBinaryName)
	}
	if err := cleanup(); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if _, statErr := os.Stat(filepath.Dir(filepath.Dir(chosen))); !os.IsNotExist(statErr) {
		t.Fatal("cleanup left the extraction tree behind")
	}
}

func TestSelectPackageBinaryPrefersHeadlessAndStatsVanished(t *testing.T) {
	tempDir := t.TempDir()
	expanded := filepath.Join(tempDir, "expanded")
	stageExpandedPackage(t, expanded, "comp", "nornicdb", 1024)
	want := stageExpandedPackage(t, expanded, "comp", managedBinaryName, 1024)
	got, err := selectPackageBinary(tempDir, expanded, "NornicDB.pkg")
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if got != want {
		t.Fatalf("got %q, want headless %q", got, want)
	}
	// A dangling symlink matches the glob but fails the follow-up stat.
	goneDir := t.TempDir()
	goneExpanded := filepath.Join(goneDir, "expanded")
	binDir := filepath.Join(goneExpanded, "comp", "Payload", "usr", "local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("stage tree: %v", err)
	}
	if err := os.Symlink(filepath.Join(binDir, "vanished"), filepath.Join(binDir, "nornicdb")); err != nil {
		t.Fatalf("stage dangling link: %v", err)
	}
	if _, err := selectPackageBinary(goneDir, goneExpanded, "NornicDB.pkg"); err == nil ||
		!strings.Contains(err.Error(), "stat expanded nornicdb binary") {
		t.Fatalf("got %v, want stat error", err)
	}
	if _, statErr := os.Stat(goneDir); !os.IsNotExist(statErr) {
		t.Fatal("temp dir survives a refused package")
	}
}

// TestSelectPackageBinaryGlobCannotFail proves the scan-error arm is dead:
// the glob pattern is a fixed construction with no malformed meta, so
// filepath.Glob only ever returns ErrBadPattern for patterns this shape
// cannot take. An empty tree returns empty with a nil error.
func TestSelectPackageBinaryGlobCannotFail(t *testing.T) {
	pattern := filepath.Join(t.TempDir(), "*", "Payload", "usr", "local", "bin", "*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("Glob(fixed pattern) = %v, want nil (only a bad pattern errors)", err)
	}
	if len(matches) != 0 {
		t.Fatalf("Glob(empty tree) = %q, want empty", matches)
	}
}

func TestCheckPackageExpansionBoundReportsTempFailure(t *testing.T) {
	oldFree := tempVolumeFreeBytes
	tempVolumeFreeBytes = func(dir string) (uint64, error) { return 0, errFakeVersion }
	t.Cleanup(func() { tempVolumeFreeBytes = oldFree })
	if err := checkPackageExpansionBound(stageSmallPackage(t)); err == nil ||
		!strings.Contains(err.Error(), "check temp disk space") {
		t.Fatalf("got %v, want temp-space error", err)
	}
}

func TestTempVolumeFreeBytesReportsStatfsFailure(t *testing.T) {
	if _, err := tempVolumeFreeBytes(filepath.Join(t.TempDir(), "absent", "child")); err == nil ||
		!strings.Contains(err.Error(), "statfs temp directory") {
		t.Fatalf("got %v, want statfs error", err)
	}
	if _, err := tempVolumeFreeBytes(t.TempDir()); err != nil {
		t.Fatalf("live temp volume: %v", err)
	}
}

func TestNewExpandCommandPinsPkgutilWiring(t *testing.T) {
	cmd := newExpandCommand("NornicDB.pkg", "target")
	if !strings.HasSuffix(cmd.Path, "pkgutil") {
		t.Fatalf("command path = %q, want pkgutil", cmd.Path)
	}
	joined := strings.Join(cmd.Args, " ")
	if !strings.Contains(joined, "--expand-full") {
		t.Fatalf("command args = %q, want --expand-full", joined)
	}
}

func TestRunBoundedExpansionReportsSetupFailures(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("stage blocker: %v", err)
	}
	if err := runBoundedExpansion(nil, "NornicDB.pkg", filepath.Join(blocker, "expanded"), 1<<20, time.Millisecond); err == nil ||
		!strings.Contains(err.Error(), "create package expansion directory") {
		t.Fatalf("got %v, want mkdir error", err)
	}
	cmd := exec.Command("/nonexistent-eshu-expansion-probe")
	startErr := runBoundedExpansion(cmd, "NornicDB.pkg", filepath.Join(t.TempDir(), "expanded"), 1<<20, time.Millisecond)
	if startErr == nil || !strings.Contains(startErr.Error(), "start package expansion") {
		t.Fatalf("got %v, want start error", startErr)
	}
}

func TestRunBoundedExpansionToleratesTransientSizeError(t *testing.T) {
	// The watchdog tick must skip a tree that vanishes mid-run and retry on
	// the next tick: the directory disappears after the process starts and
	// returns before it exits, so the install still succeeds.
	target := filepath.Join(t.TempDir(), "expanded")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("stage target: %v", err)
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.RemoveAll(target)
		time.Sleep(150 * time.Millisecond)
		_ = os.MkdirAll(target, 0o755)
	}()
	if err := runBoundedExpansion(exec.Command("sleep", "0.4"), "NornicDB.pkg", target, 1<<20, 10*time.Millisecond); err != nil {
		t.Fatalf("transiently missing tree refused: %v", err)
	}
}

func TestCheckFinishedExpansionReportsSizeFailure(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "absent")
	if err := checkFinishedExpansion("NornicDB.pkg", absent, 1<<20, nil, ""); err == nil ||
		!strings.Contains(err.Error(), "size expanded nornicdb package") {
		t.Fatalf("got %v, want size error", err)
	}
}

func TestExpandedDirSizeSkipsAndFailsClosed(t *testing.T) {
	dir := t.TempDir()
	if _, err := expandedDirSize(filepath.Join(dir, "absent")); err == nil {
		t.Fatal("missing root = nil, want fail-closed error")
	}
	total, err := expandedDirSize(dir)
	if err != nil {
		t.Fatalf("size empty dir: %v", err)
	}
	if total != 0 {
		t.Fatalf("total = %d, want 0", total)
	}
}

func TestExpandedDirSizeSkipsUnreadableSubtree(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission bits do not bind root; the skip is unobservable")
	}
	dir := t.TempDir()
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatalf("stage locked dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(locked, "secret"), []byte("0123456789"), 0o644); err != nil {
		t.Fatalf("stage secret: %v", err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatalf("chmod locked: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	total, err := expandedDirSize(dir)
	if err != nil {
		t.Fatalf("size: %v", err)
	}
	if total != 0 {
		t.Fatalf("total = %d, want 0 (locked subtree skipped)", total)
	}
}
