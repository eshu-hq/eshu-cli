// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package graphinstall

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// expandPackage shells out to the fixed macOS pkgutil to expand a .pkg
// install source into a scratch directory. It stays a variable so tests can
// stub the subprocess without touching the package manager.
var expandPackage = func(pkgPath, targetDir string) error {
	cmd := exec.Command("pkgutil", "--expand-full", pkgPath, targetDir) // #nosec G204 -- fixed binary "pkgutil" with arguments only; pkgPath is the explicitly selected package artifact and targetDir is an isolated temp directory
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("expand package %q: %w: %s", pkgPath, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func extractBinaryFromPackage(packagePath string) (string, string, func() error, error) {
	if runtime.GOOS != "darwin" {
		return "", "", nil, fmt.Errorf("nornicdb package sources are only supported on darwin today")
	}
	tempDir, err := os.MkdirTemp("", "eshu-nornicdb-package-*")
	if err != nil {
		return "", "", nil, fmt.Errorf("create package extraction directory: %w", err)
	}
	expandedDir := filepath.Join(tempDir, "expanded")
	if err := expandPackage(packagePath, expandedDir); err != nil {
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
