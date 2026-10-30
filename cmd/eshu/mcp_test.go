// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newTestMCPSetupCommand(t *testing.T) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	cmd := newMCPSetupCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetIn(strings.NewReader(""))
	return cmd, &stdout
}

// hostedStubServer serves the hosted --verify probes: the RFC 9728 discovery
// route 404s (token posture), /health is public, the query route requires a
// bearer, and /mcp/message answers tools/list.
func hostedStubServer(t *testing.T, queryAuth *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/.well-known/oauth-protected-resource"):
			w.WriteHeader(http.StatusNotFound)
		case r.URL.Path == "/health":
			_, _ = w.Write([]byte(`ok`))
		case strings.HasPrefix(r.URL.Path, "/api/v0/index-status"):
			if r.Header.Get("Authorization") == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			*queryAuth = r.Header.Get("Authorization")
			_, _ = w.Write([]byte(`{"status":"ready"}`))
		case r.URL.Path == "/mcp/message":
			raw, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(raw), `"method":"tools/list"`) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if r.Header.Get("Authorization") == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			*queryAuth = r.Header.Get("Authorization")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"find_tool","description":"find","inputSchema":{"type":"object"}},{"name":"call_tool","description":"call","inputSchema":{"type":"object"}}]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestMCPSetupPrintsLocalSnippet pins the offline print path: the generic
// local snippet is fully deterministic and carries no secret.
func TestMCPSetupPrintsLocalSnippet(t *testing.T) {
	cmd, stdout := newTestMCPSetupCommand(t)
	setCmdFlags(t, cmd, map[string]string{"platform": "generic"})
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("setup RunE error = %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, `"command": "eshu"`) || !strings.Contains(out, `"mcpServers"`) {
		t.Fatalf("setup snippet = %q, want local stdio entry", out)
	}
	if strings.Contains(strings.ToLower(out), "bearer") && strings.Contains(out, "eshu_") {
		t.Fatalf("setup snippet leaks a secret: %q", out)
	}
}

// TestMCPSetupVerifyHosted pins the hosted --verify flow against the stub
// deployment: health, remote tools/list (decoded via wirecontract), and the
// token-posture first query all run, and the report is all-OK.
func TestMCPSetupVerifyHosted(t *testing.T) {
	var queryAuth string
	srv := hostedStubServer(t, &queryAuth)
	defer srv.Close()
	t.Setenv("ESHU_MCP_TOKEN", "personal-pat-token")

	cmd, stdout := newTestMCPSetupCommand(t)
	setCmdFlags(t, cmd, map[string]string{
		"platform":    "generic",
		"hosted":      "true",
		"verify":      "true",
		"service-url": srv.URL,
		"auth":        "token",
	})
	// Resolve the client against the stub: --service-url feeds BaseURL.
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("setup --verify RunE error = %v, report:\n%s", err, stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Auth posture: token (per-user ESHU_MCP_TOKEN)") {
		t.Fatalf("verify report missing posture line:\n%s", out)
	}
	if !strings.Contains(out, "2 tools advertised by the deployment") {
		t.Fatalf("verify report missing remote tools stage:\n%s", out)
	}
	if queryAuth != "Bearer personal-pat-token" {
		t.Fatalf("query auth = %q, want the personal token", queryAuth)
	}
}

// TestMCPSetupVerifyLocalFailsToolsStage pins the thin-CLI boundary: with no
// embedded tool surface and no endpoint, the tools stage fails closed instead
// of passing on a list the binary does not carry.
func TestMCPSetupVerifyLocalFailsToolsStage(t *testing.T) {
	cmd, stdout := newTestMCPSetupCommand(t)
	setCmdFlags(t, cmd, map[string]string{"verify": "true"})
	err := cmd.RunE(cmd, []string{})
	if err == nil {
		t.Fatal("local --verify returned nil, want tools-stage failure")
	}
	if !strings.Contains(stdout.String(), "tools visible: no tool surface available") {
		t.Fatalf("verify report = %q, want tools fail-closed", stdout.String())
	}
}

// TestMCPCommandsRegistered pins the Tier-2 wiring: the mcp parent, the thin
// setup verb and its m alias, the static tools verb, and the RUNTIME start
// stub.
func TestMCPCommandsRegistered(t *testing.T) {
	for _, args := range [][]string{
		{"mcp"}, {"m"}, {"mcp", "setup"}, {"mcp", "tools"}, {"mcp", "start"},
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
