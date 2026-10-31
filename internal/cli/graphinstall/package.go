// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package graphinstall

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// expandPackage shells out to the fixed macOS pkgutil to expand a .pkg
// install source into a scratch directory, under the expansion-time tree
// bound from package_expand.go. It stays a variable so tests can stub the
// subprocess without touching the package manager.
var expandPackage = func(pkgPath, targetDir string) error {
	return runBoundedExpansion(newExpandCommand(pkgPath, targetDir), pkgPath, targetDir, nornicDBBinaryMaxBytes, expandWatchInterval)
}

func extractBinaryFromPackage(packagePath string) (string, string, func() error, error) {
	if packageGOOS != "darwin" {
		return "", "", nil, fmt.Errorf("nornicdb package sources are only supported on darwin today")
	}
	// Refuse before pkgutil expansion can fill temp disk: a compressed
	// artifact already at the cap cannot expand under it, and expansion
	// needs cap-sized scratch on the temp volume.
	if err := checkPackageExpansionBound(packagePath); err != nil {
		return "", "", nil, err
	}
	tempDir, err := os.MkdirTemp("", "eshu-nornicdb-package-*")
	if err != nil {
		return "", "", nil, fmt.Errorf("create package extraction directory: %w", err)
	}
	expandedDir := filepath.Join(tempDir, "expanded")
	// expandPackageChecked re-checks the bound so the subprocess has a
	// single chokepoint no caller can bypass.
	if err := expandPackageChecked(packagePath, expandedDir); err != nil {
		_ = os.RemoveAll(tempDir)
		return "", "", nil, err
	}

	chosen, err := selectPackageBinary(tempDir, expandedDir, packagePath)
	if err != nil {
		return "", "", nil, err
	}
	return chosen, filepath.Base(chosen), func() error { return os.RemoveAll(tempDir) }, nil
}

// selectPackageBinary chooses the managed binary from an expanded .pkg tree
// and enforces the shared extraction cap on it. It removes tempDir on every
// failure so a refused package leaves no residue. Extracted (rather than
// tested only through extractBinaryFromPackage) so the select/stat/check
// tail is pinned on Linux CI, where the darwin gate blocks the full path.
func selectPackageBinary(tempDir, expandedDir, packagePath string) (string, error) {
	candidates, err := filepath.Glob(filepath.Join(expandedDir, "*", "Payload", "usr", "local", "bin", "*"))
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", fmt.Errorf("scan expanded nornicdb package %q: %w", packagePath, err)
	}

	var chosen string
	for _, candidate := range candidates {
		name := filepath.Base(candidate)
		if name == managedBinaryName {
			chosen = candidate
			break
		}
		if name == "nornicdb" && chosen == "" {
			chosen = candidate
		}
	}
	if chosen == "" {
		_ = os.RemoveAll(tempDir)
		return "", fmt.Errorf("nornicdb package %q did not contain a usable NornicDB binary", packagePath)
	}
	info, err := os.Stat(chosen)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", fmt.Errorf("stat expanded nornicdb binary: %w", err)
	}
	if err := checkPackageBinarySize(info, packagePath); err != nil {
		_ = os.RemoveAll(tempDir)
		return "", err
	}
	return chosen, nil
}

// expandPackageChecked enforces the pre-expansion bound before the pkgutil
// subprocess runs, so a refused package never fills temp disk. Extracted
// (rather than inlined in extractBinaryFromPackage) so the check-then-expand
// ordering is pinned on Linux CI, where the darwin gate blocks the full path.
func expandPackageChecked(packagePath, expandedDir string) error {
	if err := checkPackageExpansionBound(packagePath); err != nil {
		return err
	}
	return expandPackage(packagePath, expandedDir)
}

// checkPackageExpansionBound refuses a .pkg source before pkgutil expansion:
// the compressed artifact itself must sit under the shared cap (expansion
// only grows it), and the temp volume must hold cap-sized scratch for the
// expansion tree. Both legs fail closed with an actionable error.
func checkPackageExpansionBound(packagePath string) error {
	info, err := os.Stat(packagePath)
	if err != nil {
		return fmt.Errorf("stat nornicdb package %q: %w", packagePath, err)
	}
	if err := checkPackageFileSize(info, packagePath); err != nil {
		return err
	}
	tempDir := os.TempDir()
	free, err := tempVolumeFreeBytes(tempDir)
	if err != nil {
		return fmt.Errorf("check temp disk space for nornicdb package %q: %w", packagePath, err)
	}
	return checkPackageTempFree(free, tempDir, packagePath)
}

// checkPackageFileSize refuses a package artifact that already meets or
// exceeds the shared extraction cap. The artifact is compressed, so its
// expansion can only grow past the cap the expanded binary would face.
func checkPackageFileSize(info os.FileInfo, packagePath string) error {
	if info.Size() >= nornicDBBinaryMaxBytes {
		return fmt.Errorf("nornicdb package %q is %d bytes, meeting or exceeding the 512 MiB pre-expansion bound; refusing before pkgutil expansion fills temp disk", packagePath, info.Size())
	}
	return nil
}

// checkPackageTempFree refuses expansion when the temp volume lacks room for
// a cap-sized expansion tree, naming the remedy (free space or TMPDIR) so
// the operator can act without reading source.
func checkPackageTempFree(freeBytes uint64, tempDir, packagePath string) error {
	if freeBytes < nornicDBBinaryMaxBytes {
		return fmt.Errorf("temp directory %q has only %d bytes free, below the 512 MiB needed to expand nornicdb package %q safely; free disk space or set TMPDIR to a volume with more room", tempDir, freeBytes, packagePath)
	}
	return nil
}

// packageGOOS is runtime.GOOS in production. Tests stub it to "darwin" to
// exercise the darwin install path on Linux (expandPackage is already a
// stub-able var for the same reason).
var packageGOOS = runtime.GOOS

// checkPackageBinarySize refuses an expanded Payload binary that meets or
// exceeds the shared extraction cap. pkgutil expansion is unbounded, so
// without this the darwin path would install an arbitrarily large binary
// that the archive path would have refused.
func checkPackageBinarySize(info os.FileInfo, packagePath string) error {
	if info.Size() >= nornicDBBinaryMaxBytes {
		return fmt.Errorf("nornicdb package %q binary %q meets or exceeds the 512 MiB extraction cap (%d bytes at the cap); refusing unbounded package install", packagePath, info.Name(), info.Size())
	}
	return nil
}
