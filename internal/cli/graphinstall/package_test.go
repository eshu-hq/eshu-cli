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
