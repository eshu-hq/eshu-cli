// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newTestDoctorCommand builds a fresh doctor command for a test run.
// Fresh instances (not the shared rootCmd tree) keep flag state isolated
// between tests; registration on the shared tree is pinned separately by
// TestTier3CommandsRegistered.
func newTestDoctorCommand() (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	cmd := newDoctorCommand()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader(""))
	return cmd, &stdout, &stderr
}

// TestDoctorIsNotAStub pins the Tier-3 doctor population: the command is
// registered with the core help bytes and its RunE writes the diagnostics
// report instead of the removed-command error the E3 stub returned.
func TestDoctorIsNotAStub(t *testing.T) {
	cmd, stdout, _ := newTestDoctorCommand()
	t.Setenv("ESHU_HOME", t.TempDir())
	t.Setenv("NEO4J_URI", "")
	t.Setenv("ESHU_POSTGRES_DSN", "")
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("doctor RunE: %v", err)
	}
	if strings.Contains(strings.ToLower(stdout.String()), "removed") {
		t.Fatalf("doctor returned stub guidance: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Eshu Diagnostics") {
		t.Fatalf("doctor output missing report header: %q", stdout.String())
	}
}
