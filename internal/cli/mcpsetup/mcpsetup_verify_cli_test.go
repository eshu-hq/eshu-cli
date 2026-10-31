// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package mcpsetup

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eshu-hq/eshu-sdk/wirecontract"
)

type (
	healthOK  struct{}
	healthErr struct{ err error }
	queryOK   struct{}
	queryErr  struct{ err error }
)

func (healthOK) Reachable() error    { return nil }
func (h healthErr) Reachable() error { return h.err }
func (queryOK) Smoke() error         { return nil }
func (q queryErr) Smoke() error      { return q.err }

func toolsOf(names ...string) func() []wirecontract.ToolDefinition {
	defs := make([]wirecontract.ToolDefinition, 0, len(names))
	for _, n := range names {
		defs = append(defs, wirecontract.ToolDefinition{Name: n})
	}
	return func() []wirecontract.ToolDefinition { return defs }
}

func TestVerificationAllOK(t *testing.T) {
	report := RunVerification(`{"mcpServers":{}}`, toolsOf("a", "b"), healthOK{}, queryOK{}, "")
	if !report.AllOK() {
		t.Fatalf("AllOK = false for %+v", report)
	}
	if len(report.Stages) != 4 {
		t.Fatalf("stages = %d, want 4", len(report.Stages))
	}
	rendered := RenderVerifyReport(report)
	for _, want := range []string{"MCP setup verification", "[ok] config generated: platform snippet produced", "2 tools advertised", "[ok] first query successful: bounded read succeeded"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("report missing %q in:\n%s", want, rendered)
		}
	}
}

func TestVerificationFailuresAndSkips(t *testing.T) {
	boom := errors.New("connection refused")
	report := RunVerification("   ", nil, healthErr{boom}, queryErr{boom}, "")
	if report.AllOK() {
		t.Fatal("AllOK = true with failures")
	}
	rendered := RenderVerifyReport(report)
	for _, want := range []string{
		"[!!] config generated: no snippet produced",
		"[!!] client reachable: connection refused",
		"[!!] tools visible: no tool surface available",
		"[!!] first query successful: connection refused",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("report missing %q in:\n%s", want, rendered)
		}
	}
	report = RunVerification("{}", toolsOf(), nil, nil, "")
	if report.AllOK() {
		t.Fatal("AllOK = true with empty tools")
	}
	rendered = RenderVerifyReport(report)
	for _, want := range []string{
		"[--] client reachable: no endpoint to probe (local stdio)",
		"[!!] tools visible: tool surface is empty",
		"[--] first query successful: no endpoint to query (local stdio)",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("report missing %q in:\n%s", want, rendered)
		}
	}
	report = RunVerification("{}", toolsOf("a"), nil, nil, "OAuth is interactive")
	if !report.AllOK() {
		t.Fatalf("AllOK = false with only skips for %+v", report)
	}
	if !strings.Contains(RenderVerifyReport(report), "OAuth is interactive") {
		t.Fatalf("custom skip reason missing in:\n%s", RenderVerifyReport(report))
	}
}

func TestWriteConfig(t *testing.T) {
	claude, _ := ResolvePlatform("claude")
	generic, _ := ResolvePlatform("generic")
	if err := WriteMCPServerConfig(generic, SetupRequest{}, filepath.Join(t.TempDir(), "c.json")); err == nil {
		t.Fatal("WriteMCPServerConfig(non-writable) = nil error")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "sub", ".mcp.json")
	existing := `{"mcpServers": {"other": {"command": "x"}}, "other": 1}`
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("MkdirAll = %v", err)
	}
	if err := os.WriteFile(target, []byte(existing), 0o600); err != nil {
		t.Fatalf("WriteFile = %v", err)
	}
	req := SetupRequest{Mode: ModeLocalStdio}
	if err := WriteMCPServerConfig(claude, req, target); err != nil {
		t.Fatalf("WriteMCPServerConfig = %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile = %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("Unmarshal = %v", err)
	}
	servers := doc["mcpServers"].(map[string]any)
	if _, ok := servers["eshu"]; !ok {
		t.Fatalf("merged config missing eshu in:\n%s", data)
	}
	if _, ok := servers["other"]; !ok {
		t.Fatalf("merged config dropped other server in:\n%s", data)
	}
	if doc["other"] != float64(1) {
		t.Fatalf("merged config dropped other key in:\n%s", data)
	}
	if err := os.WriteFile(target, []byte("{oops"), 0o600); err != nil {
		t.Fatalf("WriteFile = %v", err)
	}
	if err := WriteMCPServerConfig(claude, req, target); err == nil {
		t.Fatal("WriteMCPServerConfig(invalid JSON) = nil error")
	}
	vscode, _ := ResolvePlatform("vscode")
	vsTarget := filepath.Join(dir, "vscode", "mcp.json")
	if err := WriteMCPServerConfig(vscode, req, vsTarget); err != nil {
		t.Fatalf("WriteMCPServerConfig(vscode) = %v", err)
	}
	vsData, err := os.ReadFile(vsTarget)
	if err != nil || !strings.Contains(string(vsData), `"servers"`) {
		t.Fatalf("vscode config = %s, %v", vsData, err)
	}
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile = %v", err)
	}
	if err := WriteMCPServerConfig(claude, req, filepath.Join(blocker, "c.json")); err == nil {
		t.Fatal("WriteMCPServerConfig(file parent) = nil error")
	}
	if os.Geteuid() == 0 {
		t.Skip("permission fault injection needs a non-root user")
	}
	locked := filepath.Join(dir, "locked")
	if err := os.MkdirAll(filepath.Join(locked, "sub"), 0o755); err != nil {
		t.Fatalf("MkdirAll = %v", err)
	}
	if err := os.Chmod(locked, 0o500); err != nil {
		t.Fatalf("Chmod = %v", err)
	}
	defer func() { _ = os.Chmod(locked, 0o700) }()
	if err := WriteMCPServerConfig(claude, req, filepath.Join(locked, "new", "c.json")); err == nil {
		t.Fatal("WriteMCPServerConfig(locked dir) = nil error")
	}
	pinned := filepath.Join(dir, "pinned")
	if err := os.MkdirAll(pinned, 0o755); err != nil {
		t.Fatalf("MkdirAll = %v", err)
	}
	if err := os.Chmod(pinned, 0o500); err != nil {
		t.Fatalf("Chmod = %v", err)
	}
	defer func() { _ = os.Chmod(pinned, 0o700) }()
	if err := WriteMCPServerConfig(claude, req, filepath.Join(pinned, "c.json")); err == nil {
		t.Fatal("WriteMCPServerConfig(unwritable dir) = nil error")
	}
}

func TestMarshalIndentNoHTMLError(t *testing.T) {
	if _, err := marshalIndentNoHTML(map[string]any{"f": func() {}}); err == nil {
		t.Fatal("marshalIndentNoHTML(func) = nil error")
	}
}

func TestRenderSetupSnippetError(t *testing.T) {
	boom := errors.New("snippet boom")
	p := &Platform{DisplayName: "broken", snippet: func(*Platform, SetupRequest) (string, error) {
		return "", boom
	}}
	if _, err := RenderSetupSnippet(p, SetupRequest{}); err != boom {
		t.Fatalf("RenderSetupSnippet = %v, want snippet error", err)
	}
}

func TestExportOrInjectNoteWithoutEnvSupport(t *testing.T) {
	var b strings.Builder
	writeExportOrInjectNote(&b, &Platform{}, "ESHU_MCP_TOKEN")
	if !strings.Contains(b.String(), "cannot reference env vars") {
		t.Fatalf("note = %q", b.String())
	}
}

func TestCodexSharedKeySnippet(t *testing.T) {
	codex, _ := ResolvePlatform("codex")
	out, err := RenderSetupSnippet(codex, hostedReq(PostureSharedKey))
	if err != nil || !strings.Contains(out, `bearer_token_env_var = "ESHU_API_KEY"`) {
		t.Fatalf("codex shared-key snippet = %v in:\n%s", err, out)
	}
}

func TestMergeConfigShapes(t *testing.T) {
	entry := map[string]any{"command": "eshu"}
	out, err := mergeMCPServerConfig(nil, "mcpServers", entry)
	if err != nil || !strings.Contains(string(out), `"eshu"`) {
		t.Fatalf("merge empty = %s, %v", out, err)
	}
	if strings.Contains(string(out), `\u0026`) {
		t.Fatalf("merge escaped HTML in:\n%s", out)
	}
	out, err = mergeMCPServerConfig([]byte(`{"mcpServers": {"eshu": {"old": 1}}}`), "mcpServers", entry)
	if err != nil || strings.Contains(string(out), `"old"`) {
		t.Fatalf("merge overwrite = %s, %v", out, err)
	}
	if _, err := mergeMCPServerConfig([]byte(`{oops`), "mcpServers", entry); err == nil {
		t.Fatal("merge invalid JSON = nil error")
	}
	if _, err := mergeMCPServerConfig([]byte(`{"mcpServers": []}`), "mcpServers", entry); err == nil {
		t.Fatal("merge wrong-type servers = nil error")
	}
	if _, err := mergeMCPServerConfig(nil, "mcpServers", map[string]any{"f": func() {}}); err == nil {
		t.Fatal("merge unmarshalable entry = nil error")
	}
}

func TestWriteTargets(t *testing.T) {
	for platform, want := range map[string]string{"claude": ".mcp.json", "cursor": ".cursor/mcp.json", "vscode": ".vscode/mcp.json"} {
		p, _ := ResolvePlatform(platform)
		got, err := DefaultWriteTarget(p)
		if err != nil || got != want {
			t.Fatalf("DefaultWriteTarget(%q) = %q, %v", platform, got, err)
		}
	}
	generic, _ := ResolvePlatform("generic")
	if _, err := DefaultWriteTarget(generic); err == nil {
		t.Fatal("DefaultWriteTarget(generic) = nil error")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" || home == string(filepath.Separator) {
		t.Skip("no usable home dir")
	}
	if got := DescribeWriteTarget(filepath.Join(home, "x", "c.json")); got != filepath.Join("~", "x", "c.json") {
		t.Fatalf("DescribeWriteTarget = %q", got)
	}
	if got := DescribeWriteTarget(filepath.Join(string(filepath.Separator), "definitely", "outside", "home", "c.json")); got != filepath.Join(string(filepath.Separator), "definitely", "outside", "home", "c.json") {
		t.Fatalf("DescribeWriteTarget = %q", got)
	}
}
