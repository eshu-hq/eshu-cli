// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package graphinstall

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// resolveHomeDir resolves the per-user Eshu home root. It implements the
// managed-home side of the #8874 install contract (eshu#9484): ESHU_HOME
// wins verbatim with a leading ~ expanded and no "eshu" segment appended,
// and the platform default otherwise (macOS Library, Windows
// LOCALAPPDATA/USERPROFILE, Linux XDG_DATA_HOME then ~/.local/share, each
// with an "eshu" segment). getenv and userHomeDir stay parameters so tests
// pin every branch without touching the process environment.
func resolveHomeDir(getenv func(string) string, userHomeDir func() (string, error), goos string) (string, error) {
	if value := strings.TrimSpace(getenv("ESHU_HOME")); value != "" {
		return expandHome(value, userHomeDir)
	}

	switch goos {
	case "darwin":
		home, err := userHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home: %w", err)
		}
		return filepath.Join(home, "Library", "Application Support", "eshu"), nil
	case "windows":
		if value := strings.TrimSpace(getenv("LOCALAPPDATA")); value != "" {
			return filepath.Join(value, "eshu"), nil
		}
		home, err := userHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home: %w", err)
		}
		return filepath.Join(home, "AppData", "Local", "eshu"), nil
	default:
		if value := strings.TrimSpace(getenv("XDG_DATA_HOME")); value != "" {
			expanded, err := expandHome(value, userHomeDir)
			if err != nil {
				return "", err
			}
			return filepath.Join(expanded, "eshu"), nil
		}
		home, err := userHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home: %w", err)
		}
		return filepath.Join(home, ".local", "share", "eshu"), nil
	}
}

// expandHome expands a leading ~ against the user home directory. Only the
// leading-tilde form expands; every other path returns verbatim. This
// matches the core layout contract the install seam pins: "~" is a prefix
// strip joined onto the home directory, not shell-style tilde expansion.
func expandHome(path string, userHomeDir func() (string, error)) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	home, err := userHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~")), nil
}

// managedHome stubs the historical install target for tests that must not
// touch the operator's filesystem. Production paths resolve through
// resolveHomeDir above; os.Getenv and os.UserHomeDir are the live wiring.
func managedHome() (string, error) {
	return resolveHomeDir(os.Getenv, os.UserHomeDir, runtime.GOOS)
}
