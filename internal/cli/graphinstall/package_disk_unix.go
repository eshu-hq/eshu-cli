// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

//go:build unix

package graphinstall

import (
	"fmt"
	"syscall"
)

// tempVolumeFreeBytes reports the free bytes available to the caller on the
// volume holding dir, mirroring `df` available space (unprivileged blocks
// times block size). It stays a variable so tests can stub disk-full
// conditions without touching the filesystem.
var tempVolumeFreeBytes = func(dir string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err != nil {
		return 0, fmt.Errorf("statfs temp directory %q: %w", dir, err)
	}
	return uint64(stat.Bavail) * uint64(stat.Bsize), nil
}
