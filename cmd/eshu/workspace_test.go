// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newTestWorkspaceCommand builds a fresh workspace command for a test run.
// Fresh instances (not the shared rootCmd tree) keep flag state isolated
// between tests; registration on the shared tree is pinned separately by
// TestTier3CommandsRegistered.
func newTestWorkspaceCommand() (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	cmd := newWorkspaceCommand()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader(""))
	return cmd, &stdout, &stderr
}

func findWorkspaceSub(t *testing.T, cmd *cobra.Command, name string) *cobra.Command {
	t.Helper()
	for _, sub := range cmd.Commands() {
		if sub.Name() == name {
			sub.SetOut(cmd.OutOrStdout())
			sub.SetErr(cmd.ErrOrStderr())
			return sub
		}
	}
	t.Fatalf("workspace subcommand %q not registered", name)
	return nil
}

// TestWorkspaceStatusGetsPipeline pins the Tier-3 workspace status thin
// client: it GETs /api/v0/status/pipeline and prints the decoded body.
func TestWorkspaceStatusGetsPipeline(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"pipeline":"ok"},"truth":{},"error":null}`))
	}))
	defer srv.Close()

	cmd, _, _ := newTestWorkspaceCommand()
	status := findWorkspaceSub(t, cmd, "status")
	// The command resolves ambiently like its core twin (addRemoteFlags
	// stays for help parity; runWorkspaceStatus reads apiClient()).
	t.Setenv("ESHU_SERVICE_URL", srv.URL)
	if err := status.RunE(status, []string{}); err != nil {
		t.Fatalf("workspace status RunE: %v", err)
	}
	if gotPath != "/api/v0/status/pipeline" {
		t.Fatalf("workspace status requested %q, want /api/v0/status/pipeline", gotPath)
	}
	// printJSON renders to process stdout (same as core); the byte shape
	// is pinned by the core-vs-dest parity goldens, not here.
}

// TestWorkspaceRemovedSubcommandsStayRemoved pins the Tier-3 workspace
// boundary: plan, sync, and index keep the removed-command error contract,
// and watch keeps retained-lifecycle guidance instead of running a local
// watcher the destination does not own.
func TestWorkspaceRemovedSubcommandsStayRemoved(t *testing.T) {
	for _, name := range []string{"plan", "sync", "index", "watch"} {
		t.Run(name, func(t *testing.T) {
			cmd, _, _ := newTestWorkspaceCommand()
			sub := findWorkspaceSub(t, cmd, name)
			var args []string
			if name != "status" {
				args = []string{"some-path"}
			}
			err := sub.RunE(sub, args)
			if err == nil {
				t.Fatalf("workspace %s returned nil error, want removed/retained guidance", name)
			}
			if !strings.Contains(strings.ToLower(err.Error()), "removed") &&
				!strings.Contains(strings.ToLower(err.Error()), "core eshu binary") {
				t.Fatalf("workspace %s error %q names neither the removal contract nor the core binary", name, err.Error())
			}
		})
	}
}
