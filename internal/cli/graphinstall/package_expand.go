// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package graphinstall

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// expandWatchInterval samples the pkgutil expansion tree this often while
// the subprocess runs. A tighter interval shrinks the overshoot between the
// bound breach and the kill (a fast writer can add throughput-per-interval
// past the bound); the post-exit tree check below closes the remaining race
// for writers that finish between two ticks.
const expandWatchInterval = 25 * time.Millisecond

// newExpandCommand builds the fixed macOS pkgutil expansion subprocess. It
// stays a variable so tests can substitute a fake expander (the test binary
// itself) and pin the expansion-time bound on Linux, where pkgutil does not
// exist.
var newExpandCommand = func(pkgPath, targetDir string) *exec.Cmd {
	return exec.Command("pkgutil", "--expand-full", pkgPath, targetDir) // #nosec G204 -- fixed binary "pkgutil" with arguments only; pkgPath is the explicitly selected package artifact and targetDir is an isolated temp directory
}

// runBoundedExpansion runs an expansion subprocess under a total-tree disk
// bound: while the subprocess runs, a ticker measures the target directory,
// and the first sample at or past maxBytes kills the subprocess before it
// can fill temp disk. A subprocess that exits on its own still faces the
// same bound on its final tree, so a bomb that finishes between two ticks
// is refused rather than installed. Total tree size -- not any single file
// -- is the bounded quantity, because temp exhaustion is the threat: a
// per-file rlimit (RLIMIT_FSIZE) would let a many-medium-file payload
// through, and a filesystem quota needs privileges a CLI does not have.
//
// The combined subprocess output is read only after the process has been
// reaped, so the watchdog tick never races the exec copy goroutines.
func runBoundedExpansion(cmd *exec.Cmd, packagePath, targetDir string, maxBytes int64, pollInterval time.Duration) error {
	if maxBytes <= 0 {
		return fmt.Errorf("nornicdb package %q expansion bound must be positive, got %d", packagePath, maxBytes)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create package expansion directory: %w", err)
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start package expansion for %q: %w", packagePath, err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case waitErr := <-done:
			return checkFinishedExpansion(packagePath, targetDir, maxBytes, waitErr, strings.TrimSpace(output.String()))
		case <-ticker.C:
			size, sizeErr := expandedDirSize(targetDir)
			if sizeErr != nil {
				continue // transient: the expander owns the tree mid-run.
			}
			if size < maxBytes {
				continue
			}
			_ = cmd.Process.Kill()
			<-done
			return fmt.Errorf("nornicdb package %q expansion exceeded the %d-byte expansion bound (%d bytes observed); killed the expansion before it fills temp disk: the package may be highly compressible", packagePath, maxBytes, size)
		}
	}
}

// checkFinishedExpansion reports a reaped expansion: a nonzero exit keeps
// the old contract (subprocess output in the error), and a clean exit still
// faces the tree bound, closing the poll-interval race for fast bombs.
func checkFinishedExpansion(packagePath, targetDir string, maxBytes int64, waitErr error, trimmedOutput string) error {
	if waitErr != nil {
		return fmt.Errorf("expand package %q: %w: %s", packagePath, waitErr, trimmedOutput)
	}
	size, err := expandedDirSize(targetDir)
	if err != nil {
		return fmt.Errorf("size expanded nornicdb package %q: %w", packagePath, err)
	}
	if size >= maxBytes {
		return fmt.Errorf("nornicdb package %q expanded to %d bytes, meeting or exceeding the %d-byte expansion bound; refusing before install: the package may be highly compressible", packagePath, size, maxBytes)
	}
	return nil
}

// expandedDirSize totals the regular files under dir. Interior walk errors
// (a file vanishing under a running expander) are skipped and retried on
// the next tick; losing the root itself fails closed.
func expandedDirSize(dir string) (int64, error) {
	var total int64
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if path == dir {
				return err
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return total, nil
}
