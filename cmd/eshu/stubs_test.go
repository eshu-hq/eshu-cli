// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"strings"
	"testing"
)

// TestE3StubsPointAtCoreBinary pins the E3 parity exception: every
// Tier-2/Tier-3 family ships in the first population as a STUB via the
// existing removedCommandError pattern (exit 1 through the uncoded-error
// path in main.go, guidance pointing at the core binary).
func TestE3StubsPointAtCoreBinary(t *testing.T) {
	stubRoots := []string{
		"report", "investigation", "mcp", "m", "config", "component",
		"scan", "vuln-scan", "service-report", "docs", "demo",
		"demo-benchmark", "first-run", "first-run-benchmark",
		"hosted-setup", "hosted-onboard", "assistant",
		"answer-quality-scorecard", "competitive-parity", "evidence",
		"evidence-packet-dogfood", "workspace", "graph", "watch", "w",
		"admin", "doctor", "install",
	}
	for _, name := range stubRoots {
		t.Run(name, func(t *testing.T) {
			cmd, _, err := rootCmd.Find([]string{name})
			if err != nil {
				t.Fatalf("find %q: %v", name, err)
			}
			if cmd == nil || cmd.Name() != name {
				t.Fatalf("command %q not registered", name)
			}
			runErr := cmd.RunE(cmd, []string{})
			if runErr == nil {
				t.Fatalf("stub %q returned nil error, want removed-command error", name)
			}
			if !strings.Contains(strings.ToLower(runErr.Error()), "removed") {
				t.Fatalf("stub %q error %q does not name the removal contract", name, runErr.Error())
			}
		})
	}
}

// TestTier1CommandsRegistered pins the Tier-1 population scope: every
// pure-remote/mechanical family from the population-scope verdict is a
// real command in the destination tree.
func TestTier1CommandsRegistered(t *testing.T) {
	// Note: cobra's default "completion" command is added lazily at
	// Execute() time in both binaries, so it is not asserted here.
	tier1 := []string{
		"analyze", "find", "query", "list", "ls", "stats",
		"index-status", "index", "i", "change", "map", "trace",
		"freshness", "playbooks", "ecosystem", "neo4j", "n",
		"version", "help",
	}
	for _, name := range tier1 {
		t.Run(name, func(t *testing.T) {
			cmd, _, err := rootCmd.Find([]string{name})
			if err != nil {
				t.Fatalf("find %q: %v", name, err)
			}
			if cmd == nil {
				t.Fatalf("Tier-1 command %q not registered", name)
			}
		})
	}
}

// TestRuntimeLaunchersStayOut pins the inventory boundary: RUNTIME
// launchers follow their runtime repos and are not registered here.
func TestRuntimeLaunchersStayOut(t *testing.T) {
	for _, name := range []string{"api", "serve", "local-host"} {
		t.Run(name, func(t *testing.T) {
			cmd, _, err := rootCmd.Find([]string{name})
			if err == nil && cmd != nil && cmd.Name() == name {
				t.Fatalf("RUNTIME launcher %q must not be registered", name)
			}
		})
	}
}
