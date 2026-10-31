// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package mcpsetup

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func probeServer(t *testing.T, status int, body string, seen *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if seen != nil {
			*seen = r.URL.Path
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func TestProbeSSO(t *testing.T) {
	var path string
	server := probeServer(t, http.StatusOK, `{"resource":"https://h","authorization_servers":["https://idp"],"eshu_preregistered_client_id":"cid-1"}`, &path)
	defer server.Close()
	got := HostedPostureProbe(server.URL + "/ ")
	if got.Posture != PostureSSO {
		t.Fatalf("probe = %+v, want SSO", got)
	}
	if len(got.Issuers) != 1 || got.Issuers[0] != "https://idp" || got.PreregisteredClientID != "cid-1" {
		t.Fatalf("probe = %+v, want issuer and client id", got)
	}
	if got.Warning != "" {
		t.Fatalf("probe warning = %q, want empty", got.Warning)
	}
	if path != "/.well-known/oauth-protected-resource" {
		t.Fatalf("probe path = %q", path)
	}
}

func TestProbeTokenFallbacks(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       string
		wantWarn   string
		wantSilent bool
	}{
		{"clean 404", http.StatusNotFound, "nope", "", true},
		{"empty issuers", http.StatusOK, `{"resource":"x","authorization_servers":[]}`, "no authorization servers", false},
		{"status 500", http.StatusInternalServerError, "boom", "returned status 500", false},
		{"malformed JSON", http.StatusOK, "{oops", "malformed JSON", false},
	}
	for _, tc := range cases {
		server := probeServer(t, tc.status, tc.body, nil)
		got := HostedPostureProbe(server.URL)
		server.Close()
		if got.Posture != PostureToken {
			t.Fatalf("%s: posture = %v, want token", tc.name, got.Posture)
		}
		if tc.wantSilent && got.Warning != "" {
			t.Fatalf("%s: warning = %q, want empty", tc.name, got.Warning)
		}
		if !tc.wantSilent && !strings.Contains(got.Warning, tc.wantWarn) {
			t.Fatalf("%s: warning = %q, want %q", tc.name, got.Warning, tc.wantWarn)
		}
	}
}

func TestProbeNetworkError(t *testing.T) {
	server := probeServer(t, http.StatusOK, "{}", nil)
	url := server.URL
	server.Close()
	got := HostedPostureProbe(url)
	if got.Posture != PostureToken || !strings.Contains(got.Warning, "failed") {
		t.Fatalf("probe = %+v, want token with failure warning", got)
	}
}

func TestProbeClientTimeout(t *testing.T) {
	if got := newPostureProbeClient().Timeout; got != 3*time.Second {
		t.Fatalf("probe timeout = %v, want 3s", got)
	}
}

func panickingProbe(string) PostureProbeResult {
	panic("probe must not run on this path")
}

func TestResolveAuthPostureTable(t *testing.T) {
	for _, tc := range []struct {
		name              string
		authFlag          string
		sharedKey, hosted bool
		want              AuthPosture
	}{
		{"shared wins over auto hosted", "auto", true, true, PostureSharedKey},
		{"sso explicit", "sso", false, true, PostureSSO},
		{"sso padded case", " SSO ", false, false, PostureSSO},
		{"token explicit", "token", false, true, PostureToken},
		{"shared explicit", "shared-key", false, false, PostureSharedKey},
		{"blank local", "", false, false, PostureToken},
		{"auto local", "auto", false, false, PostureToken},
	} {
		got, err := ResolveAuthPosture(tc.authFlag, tc.sharedKey, tc.hosted, panickingProbe, "https://h")
		if err != nil {
			t.Fatalf("%s: ResolveAuthPosture = %v", tc.name, err)
		}
		if got.Posture != tc.want {
			t.Fatalf("%s: posture = %v, want %v", tc.name, got.Posture, tc.want)
		}
		if got.Warning != "" {
			t.Fatalf("%s: warning = %q, want empty", tc.name, got.Warning)
		}
	}
}

func TestResolveAuthPostureProbePaths(t *testing.T) {
	want := PostureProbeResult{Posture: PostureSSO, Issuers: []string{"https://idp"}}
	got, err := ResolveAuthPosture("auto", false, true, func(url string) PostureProbeResult {
		if url != "https://h" {
			t.Fatalf("probe url = %q", url)
		}
		return want
	}, "https://h")
	if err != nil || got.Posture != PostureSSO {
		t.Fatalf("ResolveAuthPosture = %+v, %v", got, err)
	}
	if _, err := ResolveAuthPosture("", false, true, nil, "https://h"); err == nil ||
		!strings.Contains(err.Error(), "sso, token, or shared-key") {
		t.Fatalf("ResolveAuthPosture(nil probe) = %v, want recovery error", err)
	}
	if _, err := ResolveAuthPosture("bogus", false, true, panickingProbe, "https://h"); err == nil ||
		!strings.Contains(err.Error(), "auto, sso, token, or shared-key") {
		t.Fatalf("ResolveAuthPosture(bogus) = %v, want accepted-values error", err)
	}
}

func TestPlatformRegistry(t *testing.T) {
	if got := SupportedPlatformNames(); strings.Join(got, ",") != "claude,codex,cursor,generic,vscode" {
		t.Fatalf("SupportedPlatformNames = %v", got)
	}
	claude, err := ResolvePlatform("Claude")
	if err != nil {
		t.Fatalf("ResolvePlatform = %v", err)
	}
	alias, err := ResolvePlatform("claude-code")
	if err != nil || alias.DisplayName != claude.DisplayName {
		t.Fatalf("ResolvePlatform(alias) = %+v, %v", alias, err)
	}
	generic, err := ResolvePlatform("  ")
	if err != nil || generic.Name != "generic" {
		t.Fatalf("ResolvePlatform(blank) = %+v, %v", generic, err)
	}
	if _, err := ResolvePlatform("nope"); err == nil || !strings.Contains(err.Error(), "claude") {
		t.Fatalf("ResolvePlatform(nope) = %v, want supported-names error", err)
	}
}

func TestTokenReference(t *testing.T) {
	claude, _ := ResolvePlatform("claude")
	if got := TokenReference(claude, "ESHU_MCP_TOKEN", ""); got != "${ESHU_MCP_TOKEN}" {
		t.Fatalf("TokenReference = %q", got)
	}
	bare := &Platform{SupportsEnvVarToken: false}
	if got := TokenReference(bare, "ESHU_API_KEY", ""); got != "<set ESHU_API_KEY out of band>" {
		t.Fatalf("TokenReference = %q", got)
	}
	secret := "abcdefghijklmnopqrstuvw"
	if got := TokenReference(bare, "ESHU_API_KEY", secret); got != "abcd*******************" {
		t.Fatalf("TokenReference = %q, want masked prefix", got)
	}
	if got := RedactToken(secret); got != "abcd*******************" {
		t.Fatalf("RedactToken = %q", got)
	}
}

func hostedReq(posture AuthPosture) SetupRequest {
	return SetupRequest{Mode: ModeHostedHTTP, ServiceURL: "https://eshu.example/", Posture: posture}
}

func TestSnippets(t *testing.T) {
	claude, _ := ResolvePlatform("claude")
	out, err := RenderSetupSnippet(claude, SetupRequest{Mode: ModeLocalStdio})
	if err != nil {
		t.Fatalf("RenderSetupSnippet = %v", err)
	}
	for _, want := range []string{"MCP setup for Claude Code", `"command": "eshu"`, "ESHU_MCP_RESULT_SHAPE", ".mcp.json", "Notes:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdio snippet missing %q in:\n%s", want, out)
		}
	}
	out, err = RenderSetupSnippet(claude, hostedReq(PostureToken))
	if err != nil {
		t.Fatalf("RenderSetupSnippet = %v", err)
	}
	for _, want := range []string{"https://eshu.example/mcp/message?shape=structured", "${ESHU_MCP_TOKEN}", "export ESHU_MCP_TOKEN=..."} {
		if !strings.Contains(out, want) {
			t.Fatalf("hosted snippet missing %q in:\n%s", want, out)
		}
	}
	cursor, _ := ResolvePlatform("cursor")
	out, err = RenderSetupSnippet(cursor, hostedReq(PostureToken))
	if err != nil || strings.Contains(out, "?shape=") {
		t.Fatalf("cursor snippet = %v in:\n%s", err, out)
	}
	vscode, _ := ResolvePlatform("vscode")
	out, err = RenderSetupSnippet(vscode, hostedReq(PostureToken))
	if err != nil || !strings.Contains(out, `"servers"`) || strings.Contains(out, `"mcpServers"`) {
		t.Fatalf("vscode snippet = %v in:\n%s", err, out)
	}
}

func TestSnippetCredentialWiring(t *testing.T) {
	claude, _ := ResolvePlatform("claude")
	out, err := RenderSetupSnippet(claude, hostedReq(PostureSSO))
	if err != nil || strings.Contains(out, "headers") {
		t.Fatalf("SSO snippet = %v in:\n%s", err, out)
	}
	if !strings.Contains(out, "OAuth") || !strings.Contains(out, "sign-in") {
		t.Fatalf("SSO guidance missing in:\n%s", out)
	}
	raw := "test-secret-value-00"
	req := hostedReq(PostureSharedKey)
	req.APIKey = raw
	out, err = RenderSetupSnippet(claude, req)
	if err != nil || strings.Contains(out, raw) || !strings.Contains(out, "${ESHU_API_KEY}") {
		t.Fatalf("shared-key snippet = %v in:\n%s", err, out)
	}
	if !strings.Contains(out, "WARNING") {
		t.Fatalf("shared-key caveat missing in:\n%s", out)
	}
	codex, _ := ResolvePlatform("codex")
	out, err = RenderSetupSnippet(codex, hostedReq(PostureToken))
	if err != nil || !strings.Contains(out, `bearer_token_env_var = "ESHU_MCP_TOKEN"`) {
		t.Fatalf("codex snippet = %v in:\n%s", err, out)
	}
	out, err = RenderSetupSnippet(codex, SetupRequest{Mode: ModeLocalStdio})
	if err != nil || !strings.Contains(out, "[mcp_servers.eshu]") {
		t.Fatalf("codex stdio snippet = %v in:\n%s", err, out)
	}
	out, err = RenderSetupSnippet(codex, hostedReq(PostureSSO))
	if err != nil || strings.Contains(out, "bearer_token_env_var") || !strings.Contains(out, "--auth token") {
		t.Fatalf("codex SSO snippet = %v in:\n%s", err, out)
	}
	probeReq := hostedReq(PostureSSO)
	probeReq.Issuers = []string{"https://idp"}
	probeReq.PreregisteredClientID = "cid-1"
	out, err = RenderSetupSnippet(claude, probeReq)
	if err != nil || !strings.Contains(out, "https://idp") || !strings.Contains(out, "cid-1") {
		t.Fatalf("SSO probe guidance = %v in:\n%s", err, out)
	}
	empty := SetupRequest{Mode: ModeHostedHTTP}
	out, err = RenderSetupSnippet(claude, empty)
	if err != nil || !strings.Contains(out, "https://your-eshu-host/mcp/message") {
		t.Fatalf("placeholder snippet = %v in:\n%s", err, out)
	}
}
