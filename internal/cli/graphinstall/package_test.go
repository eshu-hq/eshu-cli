// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package graphinstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stagePackageFile stages a fake .pkg artifact of exactly size bytes
// (sparse: no disk is consumed). Expansion never runs in these tests; only
// the artifact's stat size matters to the pre-expansion bound.
func stagePackageFile(t *testing.T, dir string, size int64) string {
	t.Helper()
	pkg := filepath.Join(dir, "NornicDB.pkg")
	f, err := os.Create(pkg)
	if err != nil {
		t.Fatalf("stage package: %v", err)
	}
	if err := f.Truncate(size); err != nil {
		t.Fatalf("size package: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close package: %v", err)
	}
	return pkg
}

func TestCheckPackageFileSizeRefusesOverCap(t *testing.T) {
	// A compressed artifact already over the cap cannot expand under it,
	// so it must be refused before pkgutil runs.
	dir := t.TempDir()
	pkg := stagePackageFile(t, dir, (512<<20)+1)
	fi, err := os.Stat(pkg)
	if err != nil {
		t.Fatalf("stat package: %v", err)
	}
	if err := checkPackageFileSize(fi, pkg); err == nil ||
		!strings.Contains(err.Error(), "pre-expansion") {
		t.Fatalf("got %v, want pre-expansion refusal", err)
	}
}

func TestCheckPackageFileSizeRefusesExactCap(t *testing.T) {
	// Meets-or-exceeds, matching the post-expansion boundary: an artifact
	// of exactly the cap size must be refused too.
	dir := t.TempDir()
	pkg := stagePackageFile(t, dir, 512<<20)
	fi, err := os.Stat(pkg)
	if err != nil {
		t.Fatalf("stat package: %v", err)
	}
	if err := checkPackageFileSize(fi, pkg); err == nil ||
		!strings.Contains(err.Error(), "pre-expansion") {
		t.Fatalf("got %v, want pre-expansion refusal", err)
	}
}

func TestCheckPackageFileSizeAllowsUnderCap(t *testing.T) {
	dir := t.TempDir()
	pkg := stagePackageFile(t, dir, 1024)
	fi, err := os.Stat(pkg)
	if err != nil {
		t.Fatalf("stat package: %v", err)
	}
	if err := checkPackageFileSize(fi, pkg); err != nil {
		t.Fatalf("under-cap package refused: %v", err)
	}
}

func TestCheckPackageTempFreeRefusesLowSpace(t *testing.T) {
	// A temp volume without cap-sized room must refuse with an actionable
	// error naming TMPDIR, before expansion fills the disk.
	err := checkPackageTempFree(1024, t.TempDir(), "NornicDB.pkg")
	if err == nil || !strings.Contains(err.Error(), "TMPDIR") {
		t.Fatalf("got %v, want actionable temp-disk refusal", err)
	}
}

func TestCheckPackageTempFreeAllowsCapRoom(t *testing.T) {
	if err := checkPackageTempFree(512<<20, t.TempDir(), "NornicDB.pkg"); err != nil {
		t.Fatalf("cap-sized room refused: %v", err)
	}
}

func TestExpandPackageCheckedRefusesBeforeExpand(t *testing.T) {
	// The #30 regression guard: when the temp volume lacks room, the
	// pkgutil subprocess must never run. A stubbed expandPackage that
	// records calls proves the refusal happens before expansion.
	oldExpand, oldFree := expandPackage, tempVolumeFreeBytes
	t.Cleanup(func() { expandPackage, tempVolumeFreeBytes = oldExpand, oldFree })
	var expanded []string
	expandPackage = func(pkgPath, targetDir string) error {
		expanded = append(expanded, pkgPath)
		return nil
	}
	tempVolumeFreeBytes = func(dir string) (uint64, error) { return 1024, nil }

	pkg := stagePackageFile(t, t.TempDir(), 1024)
	if err := expandPackageChecked(pkg, filepath.Join(t.TempDir(), "expanded")); err == nil ||
		!strings.Contains(err.Error(), "TMPDIR") {
		t.Fatalf("got %v, want temp-disk refusal", err)
	}
	if len(expanded) != 0 {
		t.Fatalf("pkgutil expansion ran %d times for a refused package", len(expanded))
	}
}

func TestExpandPackageCheckedRefusesOversizedArtifactBeforeExpand(t *testing.T) {
	// Same ordering guard for the artifact-size leg: an over-cap .pkg
	// must not reach pkgutil however roomy the temp volume is.
	oldExpand, oldFree := expandPackage, tempVolumeFreeBytes
	t.Cleanup(func() { expandPackage, tempVolumeFreeBytes = oldExpand, oldFree })
	var expanded []string
	expandPackage = func(pkgPath, targetDir string) error {
		expanded = append(expanded, pkgPath)
		return nil
	}
	tempVolumeFreeBytes = func(dir string) (uint64, error) { return 1 << 40, nil }

	pkg := stagePackageFile(t, t.TempDir(), (512<<20)+1)
	if err := expandPackageChecked(pkg, filepath.Join(t.TempDir(), "expanded")); err == nil ||
		!strings.Contains(err.Error(), "pre-expansion") {
		t.Fatalf("got %v, want pre-expansion refusal", err)
	}
	if len(expanded) != 0 {
		t.Fatalf("pkgutil expansion ran %d times for a refused package", len(expanded))
	}
}

func TestExpandPackageCheckedExpandsWhenBoundPasses(t *testing.T) {
	// The bound must not break the normal path: a small artifact on a
	// roomy volume still reaches pkgutil exactly once.
	oldExpand, oldFree := expandPackage, tempVolumeFreeBytes
	t.Cleanup(func() { expandPackage, tempVolumeFreeBytes = oldExpand, oldFree })
	var expanded []string
	expandPackage = func(pkgPath, targetDir string) error {
		expanded = append(expanded, pkgPath)
		return nil
	}
	tempVolumeFreeBytes = func(dir string) (uint64, error) { return 1 << 40, nil }

	pkg := stagePackageFile(t, t.TempDir(), 1024)
	if err := expandPackageChecked(pkg, filepath.Join(t.TempDir(), "expanded")); err != nil {
		t.Fatalf("bound-passing package refused: %v", err)
	}
	if len(expanded) != 1 || expanded[0] != pkg {
		t.Fatalf("got expansions %q, want exactly [%q]", expanded, pkg)
	}
}

func TestCheckPackageExpansionBoundStatsMissingPackage(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.pkg")
	if err := checkPackageExpansionBound(missing); err == nil ||
		!strings.Contains(err.Error(), "stat nornicdb package") {
		t.Fatalf("got %v, want stat error", err)
	}
}

func TestPackageBinarySizeRefusesOverCap(t *testing.T) {
	// Fail-closed darwin path: the expanded Payload binary must face the
	// same 512 MiB cap as archive members (issue #29).
	dir := t.TempDir()
	big := filepath.Join(dir, "nornicdb")
	f, err := os.Create(big)
	if err != nil {
		t.Fatalf("stage binary: %v", err)
	}
	if err := f.Truncate((512 << 20) + 1); err != nil {
		t.Fatalf("size binary: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close binary: %v", err)
	}
	fi, err := os.Stat(big)
	if err != nil {
		t.Fatalf("stat binary: %v", err)
	}
	if err := checkPackageBinarySize(fi, "NornicDB.pkg"); err == nil ||
		!strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("got %v, want over-cap refusal", err)
	}
}

func TestPackageBinarySizeRefusesExactCap(t *testing.T) {
	// The meets-or-exceeds boundary: an expanded binary of exactly the cap
	// size is indistinguishable from a truncated longer one, so it must be
	// refused too. A `>` instead of `>=` in the check would allow it and
	// fail this test.
	dir := t.TempDir()
	exact := filepath.Join(dir, "nornicdb")
	f, err := os.Create(exact)
	if err != nil {
		t.Fatalf("stage binary: %v", err)
	}
	if err := f.Truncate(512 << 20); err != nil {
		t.Fatalf("size binary: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close binary: %v", err)
	}
	fi, err := os.Stat(exact)
	if err != nil {
		t.Fatalf("stat binary: %v", err)
	}
	if err := checkPackageBinarySize(fi, "NornicDB.pkg"); err == nil ||
		!strings.Contains(err.Error(), "meets or exceeds") {
		t.Fatalf("got %v, want exact-cap refusal", err)
	}
}

func TestPackageBinarySizeAllowsUnderCap(t *testing.T) {
	dir := t.TempDir()
	small := filepath.Join(dir, "nornicdb")
	if err := os.WriteFile(small, []byte("fake"), 0o755); err != nil {
		t.Fatalf("stage binary: %v", err)
	}
	fi, err := os.Stat(small)
	if err != nil {
		t.Fatalf("stat binary: %v", err)
	}
	if err := checkPackageBinarySize(fi, "NornicDB.pkg"); err != nil {
		t.Fatalf("under-cap binary refused: %v", err)
	}
}

// stageExpandedPackage builds a fake pkgutil expansion tree holding one
// binary of exactly size bytes (sparse: no disk is consumed). treeRoot is
// the directory expandPackage would have populated (the "expanded" dir).
func stageExpandedPackage(t *testing.T, treeRoot, comp, name string, size int64) string {
	t.Helper()
	binDir := filepath.Join(treeRoot, comp, "Payload", "usr", "local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("stage expanded tree: %v", err)
	}
	bin := filepath.Join(binDir, name)
	f, err := os.Create(bin)
	if err != nil {
		t.Fatalf("stage binary: %v", err)
	}
	if err := f.Truncate(size); err != nil {
		t.Fatalf("size binary: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close binary: %v", err)
	}
	return bin
}

func TestSelectPackageBinaryRefusesOverCap(t *testing.T) {
	// Pins selectPackageBinary (not the extractBinaryFromPackage entrypoint):
	// neutralizing the select/stat/check tail inside selectPackageBinary
	// must fail this test. It runs on Linux with a fake expanded tree
	// (no pkgutil, no darwin gate). The entrypoint delegation is pinned by
	// TestExtractBinaryFromPackageRefusesOverCapEndToEnd.
	tempDir := t.TempDir()
	expanded := filepath.Join(tempDir, "expanded")
	stageExpandedPackage(t, filepath.Join(tempDir, "expanded"), "comp", "nornicdb", (512<<20)+1)
	if _, err := selectPackageBinary(tempDir, expanded, "NornicDB.pkg"); err == nil ||
		!strings.Contains(err.Error(), "meets or exceeds") {
		t.Fatalf("got %v, want over-cap refusal", err)
	}
	if _, statErr := os.Stat(tempDir); !os.IsNotExist(statErr) {
		t.Fatalf("temp dir survives a refused package: %v", statErr)
	}
}

func TestSelectPackageBinaryRefusesExactCap(t *testing.T) {
	tempDir := t.TempDir()
	expanded := filepath.Join(tempDir, "expanded")
	stageExpandedPackage(t, filepath.Join(tempDir, "expanded"), "comp", "nornicdb", 512<<20)
	if _, err := selectPackageBinary(tempDir, expanded, "NornicDB.pkg"); err == nil ||
		!strings.Contains(err.Error(), "meets or exceeds") {
		t.Fatalf("got %v, want exact-cap refusal", err)
	}
	if _, statErr := os.Stat(tempDir); !os.IsNotExist(statErr) {
		t.Fatalf("temp dir survives a refused package: %v", statErr)
	}
}

func TestSelectPackageBinarySelectsUnderCap(t *testing.T) {
	tempDir := t.TempDir()
	expanded := filepath.Join(tempDir, "expanded")
	want := stageExpandedPackage(t, filepath.Join(tempDir, "expanded"), "comp", "nornicdb", 1024)
	got, err := selectPackageBinary(tempDir, expanded, "NornicDB.pkg")
	if err != nil {
		t.Fatalf("under-cap binary refused: %v", err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if _, statErr := os.Stat(tempDir); statErr != nil {
		t.Fatalf("temp dir removed for an accepted package: %v", statErr)
	}
}

func TestExtractBinaryFromPackageRefusesOverCapEndToEnd(t *testing.T) {
	// Pins the delegation the unit tests cannot see: extractBinaryFromPackage
	// must refuse an over-cap expanded binary through its real entrypoint.
	// Neutralizing its selectPackageBinary call fails this test.
	originalGOOS, originalExpand := packageGOOS, expandPackage
	packageGOOS = "darwin"
	expandPackage = func(pkgPath, targetDir string) error {
		stageExpandedPackage(t, targetDir, "comp", "nornicdb", (512<<20)+1)
		return nil
	}
	t.Cleanup(func() { packageGOOS, expandPackage = originalGOOS, originalExpand })

	pkg := filepath.Join(t.TempDir(), "NornicDB.pkg")
	if err := os.WriteFile(pkg, []byte("pkg"), 0o644); err != nil {
		t.Fatalf("stage pkg: %v", err)
	}
	if _, _, cleanup, err := extractBinaryFromPackage(pkg); err == nil ||
		!strings.Contains(err.Error(), "meets or exceeds") {
		t.Fatalf("got %v, want over-cap refusal", err)
	} else if cleanup != nil {
		_ = cleanup()
	}
}

func TestSelectPackageBinaryWithoutBinaryFails(t *testing.T) {
	tempDir := t.TempDir()
	expanded := filepath.Join(tempDir, "expanded")
	if err := os.MkdirAll(expanded, 0o755); err != nil {
		t.Fatalf("stage expanded tree: %v", err)
	}
	if _, err := selectPackageBinary(tempDir, expanded, "NornicDB.pkg"); err == nil ||
		!strings.Contains(err.Error(), "did not contain a usable NornicDB binary") {
		t.Fatalf("got %v, want missing-binary error", err)
	}
	if _, statErr := os.Stat(tempDir); !os.IsNotExist(statErr) {
		t.Fatalf("temp dir survives a refused package: %v", statErr)
	}
}
