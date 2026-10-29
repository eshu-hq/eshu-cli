// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package config

import (
	"os"
	"testing"
)

// TestCredentialProfileRoundTrip pins the credential contract the CLI moves
// with (population-scope verdict, decision 7): a profile write lands in the
// frozen ~/.eshu/.env location at mode 0600, and ESHU_API_KEY resolves
// through the profile suffix so the client sends it as a Bearer token.
func TestCredentialProfileRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv(homeEnvVar, home)

	if err := SetValue("ESHU_API_KEY", "base-token"); err != nil {
		t.Fatalf("SetValue() error = %v, want nil", err)
	}
	if err := SetValue("ESHU_API_KEY_STAGING", "staging-token"); err != nil {
		t.Fatalf("SetValue() error = %v, want nil", err)
	}

	info, err := os.Stat(EnvFilePath())
	if err != nil {
		t.Fatalf("Stat(.env) error = %v, want the profile file", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf(".env mode = %o, want 600", perm)
	}

	if got := ResolveValue("ESHU_API_KEY", "staging"); got != "staging-token" {
		t.Fatalf("ResolveValue(profile=staging) = %q, want the profile token", got)
	}
	if got := ResolveValue("ESHU_API_KEY", ""); got != "base-token" {
		t.Fatalf("ResolveValue(profile=\"\") = %q, want the base token", got)
	}
}
