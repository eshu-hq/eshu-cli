// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

//go:build !unix

package graphinstall

import "fmt"

// tempVolumeFreeBytes is unavailable off unix (no statfs in stdlib), so the
// pre-expansion free-space leg fails closed there. Unreachable in practice:
// extractBinaryFromPackage admits .pkg sources only on darwin, which builds
// with the unix implementation.
var tempVolumeFreeBytes = func(dir string) (uint64, error) {
	return 0, fmt.Errorf("temp disk free-space check is unavailable on this platform (dir %q)", dir)
}
