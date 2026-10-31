// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package graphinstall

import (
	"fmt"
	"os/exec"
	"strings"
)

// VersionReader reports the version string a NornicDB binary prints in
// response to `<binary> version`. graphinstall never executes a candidate
// binary except through this seam: the default reader shells out with a
// fixed "version" argument, and tests inject fakes that assert no
// subprocess runs at all.
type VersionReader func(binaryPath string) (string, error)

// DefaultVersionReader is the production VersionReader. The candidate
// binary is the operator-selected install source (or a verified
// downloaded/extracted artifact); probing it with the fixed "version"
// argument is the explicit install verification step.
func DefaultVersionReader(binaryPath string) (string, error) {
	output, err := exec.Command(binaryPath, "version").Output() // #nosec G204 -- binaryPath is the operator-selected or verified install artifact; "version" is a fixed literal argument
	if err != nil {
		return "", fmt.Errorf("read nornicdb version: %w", err)
	}
	return parseNornicDBVersionOutput(string(output))
}

// parseNornicDBVersionOutput extracts the version from a `<binary> version`
// line. The accepted shape is the version-probe protocol the install
// contract pins: the literal prefix "NornicDB " followed by a non-empty
// version. Anything else fails closed so a wrong binary never installs.
func parseNornicDBVersionOutput(output string) (string, error) {
	version := strings.TrimSpace(output)
	if version == "" {
		return "", fmt.Errorf("empty output")
	}
	const prefix = "NornicDB "
	if !strings.HasPrefix(version, prefix) {
		return "", fmt.Errorf("unexpected output %q", version)
	}
	version = strings.TrimSpace(strings.TrimPrefix(version, prefix))
	if version == "" {
		return "", fmt.Errorf("missing version in output %q", output)
	}
	return version, nil
}
