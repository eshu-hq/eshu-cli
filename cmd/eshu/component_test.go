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

// newTestComponentCommand builds a fresh component subtree per test, so flag
// Changed state never leaks between tests. Registration on the shared tree
// is pinned separately by TestComponentCommandsRegistered.
func newTestComponentCommand(t *testing.T, args ...string) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	leaf, _, err := newComponentCommand().Find(args)
	if err != nil {
		t.Fatalf("find component %v: %v", args, err)
	}
	if leaf == nil {
		t.Fatalf("component %v not registered", args)
	}
	var stdout bytes.Buffer
	leaf.SetOut(&stdout)
	leaf.SetErr(&bytes.Buffer{})
	leaf.SetIn(strings.NewReader(""))
	return leaf, &stdout
}

// TestComponentInventoryRendersSummary pins the thin inventory flow: the CLI
// fetches the extension inventory through the configured API and renders the
// copied summary.
func TestComponentInventoryRendersSummary(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"count":1,"total_count":1,"limit":100,"truncated":false,"components":[{"id":"demo.ext","version":"0.1.0","states":["active"]}]},"truth":{"freshness":{"state":"fresh"}},"error":null}`))
	}))
	defer srv.Close()
	t.Setenv("ESHU_SERVICE_URL", srv.URL)

	cmd, stdout := newTestComponentCommand(t, "inventory")
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("inventory RunE error = %v", err)
	}
	if gotQuery != "limit=100" {
		t.Fatalf("inventory query = %q, want limit=100", gotQuery)
	}
	want := "Truth freshness: fresh\nComponent extensions: 1 of 1 (limit=100, truncated=false)\ndemo.ext@0.1.0 states=active\n"
	if stdout.String() != want {
		t.Fatalf("inventory stdout = %q, want %q", stdout.String(), want)
	}
}

// TestComponentInventoryMapsEnvelopeErrors pins the exit-code mapping: an
// in-envelope not_found becomes exit 2 through the shared trace table.
func TestComponentInventoryMapsEnvelopeErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":null,"truth":null,"error":{"code":"not_found","message":"no such extension"}}`))
	}))
	defer srv.Close()
	t.Setenv("ESHU_SERVICE_URL", srv.URL)

	cmd, stdout := newTestComponentCommand(t, "inventory")
	err := cmd.RunE(cmd, []string{})
	coded, ok := err.(commandExitError)
	if !ok || coded.ExitCode() != 2 {
		t.Fatalf("inventory envelope error exit = %v, want code 2", err)
	}
	if !strings.Contains(stdout.String(), "Component extension error (not_found): no such extension") {
		t.Fatalf("inventory stdout = %q, want error summary", stdout.String())
	}
}

// TestComponentDiagnosticsRendersRow pins the diagnostics drilldown: an
// envelope carrying a single component renders one row.
func TestComponentDiagnosticsRendersRow(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"component":{"id":"demo.ext","version":"0.2.0","states":["quarantined"],"diagnostics":{"policy_code":"deny","policy_reason":"revoked"}}},"truth":null,"error":null}`))
	}))
	defer srv.Close()
	t.Setenv("ESHU_SERVICE_URL", srv.URL)

	cmd, stdout := newTestComponentCommand(t, "diagnostics")
	if err := cmd.RunE(cmd, []string{"demo.ext"}); err != nil {
		t.Fatalf("diagnostics RunE error = %v", err)
	}
	if gotPath != "/api/v0/component-extensions/demo.ext/diagnostics" {
		t.Fatalf("diagnostics path = %q", gotPath)
	}
	want := "demo.ext@0.2.0 states=quarantined\n  policy=deny reason=revoked\n"
	if stdout.String() != want {
		t.Fatalf("diagnostics stdout = %q, want %q", stdout.String(), want)
	}
}

// TestComponentCommandsRegistered pins the Tier-2 wiring: the component
// parent, the thin inventory/diagnostics verbs, and the extension-host-bound
// local verbs with their core surface.
func TestComponentCommandsRegistered(t *testing.T) {
	for _, args := range [][]string{
		{"component"},
		{"component", "inventory"},
		{"component", "diagnostics"},
		{"component", "inspect"},
		{"component", "verify"},
		{"component", "install"},
		{"component", "list"},
		{"component", "enable"},
		{"component", "disable"},
		{"component", "uninstall"},
		{"component", "conform"},
		{"component", "init"},
		{"component", "index"},
		{"component", "extraction-readiness"},
		{"component", "schema-versions"},
		{"component", "grant"},
		{"component", "revoke-grant"},
		{"component", "grants"},
	} {
		t.Run(strings.Join(args, "-"), func(t *testing.T) {
			cmd, _, err := rootCmd.Find(args)
			if err != nil {
				t.Fatalf("find %v: %v", args, err)
			}
			if cmd == nil || cmd.Name() != args[len(args)-1] {
				t.Fatalf("command %v not registered", args)
			}
		})
	}
}
