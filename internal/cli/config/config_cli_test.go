// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eshu-hq/eshu-sdk/wirecontract"
)

func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv(homeEnvVar, home)
	return home
}

func TestHomeShapes(t *testing.T) {
	home := isolateHome(t)
	if got := Home(); got != home {
		t.Fatalf("Home() = %q, want %q", got, home)
	}
	t.Setenv(homeEnvVar, "~/sub")
	want := filepath.Join(mustUserHome(t), "sub")
	if got := Home(); got != want {
		t.Fatalf("Home(~) = %q, want %q", got, want)
	}
	t.Setenv(homeEnvVar, "")
	if got := Home(); !strings.HasSuffix(got, homeDirname) {
		t.Fatalf("Home(unset) = %q, want ~/%s", got, homeDirname)
	}
	if got := EnvFilePath(); got != filepath.Join(Home(), envFileName) {
		t.Fatalf("EnvFilePath() = %q", got)
	}
}

func mustUserHome(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home dir")
	}
	return home
}

func TestLoadShapes(t *testing.T) {
	isolateHome(t)
	if got := Load(); got == nil || len(got) != 0 {
		t.Fatalf("Load(missing) = %v, want empty non-nil map", got)
	}
	body := "# comment\n\nbare-line\nA = 1\nB=2=3\n A=overridden \n"
	if err := os.MkdirAll(Home(), 0o750); err != nil {
		t.Fatalf("MkdirAll = %v", err)
	}
	if err := os.WriteFile(EnvFilePath(), []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile = %v", err)
	}
	got := Load()
	want := map[string]string{"A": "overridden", "B": "2=3"}
	if len(got) != len(want) {
		t.Fatalf("Load() = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("Load()[%q] = %q, want %q", k, got[k], v)
		}
	}
}

func TestResolveValueProfiles(t *testing.T) {
	isolateHome(t)
	for k, v := range map[string]string{
		"ESHU_SERVICE_URL":         "https://base",
		"ESHU_SERVICE_URL_STAGING": "https://staging",
		"ESHU_SERVICE_URL_EMPTY":   "",
	} {
		if err := SetValue(k, v); err != nil {
			t.Fatalf("SetValue = %v", err)
		}
	}
	if got := ResolveValue("ESHU_SERVICE_URL", "staging"); got != "https://staging" {
		t.Fatalf("profile hit = %q", got)
	}
	if got := ResolveValue("ESHU_SERVICE_URL", "STAGING"); got != "https://staging" {
		t.Fatalf("uppercase profile = %q", got)
	}
	if got := ResolveValue("ESHU_SERVICE_URL", "prod"); got != "https://base" {
		t.Fatalf("missing profile = %q, want base fallback", got)
	}
	if got := ResolveValue("ESHU_SERVICE_URL", "empty"); got != "https://base" {
		t.Fatalf("empty profile value = %q, want base fallback", got)
	}
	if got := ResolveValue("ESHU_SERVICE_URL", ""); got != "https://base" {
		t.Fatalf("empty profile = %q", got)
	}
	if got := ResolveValue("ESHU_MISSING", "staging"); got != "" {
		t.Fatalf("missing key = %q, want empty", got)
	}
}

func TestSetValueStableBytes(t *testing.T) {
	isolateHome(t)
	if err := SetValue("B_KEY", "2"); err != nil {
		t.Fatalf("SetValue = %v", err)
	}
	if err := SetValue("A_KEY", "1"); err != nil {
		t.Fatalf("SetValue = %v", err)
	}
	first, err := os.ReadFile(EnvFilePath())
	if err != nil {
		t.Fatalf("ReadFile = %v", err)
	}
	if string(first) != "A_KEY=1\nB_KEY=2\n" {
		t.Fatalf("env bytes = %q, want sorted stable output", first)
	}
	if err := SetValue("A_KEY", "1"); err != nil {
		t.Fatalf("SetValue = %v", err)
	}
	second, err := os.ReadFile(EnvFilePath())
	if err != nil {
		t.Fatalf("ReadFile = %v", err)
	}
	if string(second) != string(first) {
		t.Fatalf("rewrite changed bytes: %q vs %q", first, second)
	}
}

func TestSetValueMkdirFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission fault injection needs a non-root user")
	}
	home := isolateHome(t)
	blocker := filepath.Join(home, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile = %v", err)
	}
	t.Setenv(homeEnvVar, filepath.Join(blocker, "sub"))
	if err := SetValue("K", "v"); err == nil {
		t.Fatal("SetValue(blocked dir) = nil error")
	}
}

func TestReset(t *testing.T) {
	isolateHome(t)
	if err := SetValue("K", "v"); err != nil {
		t.Fatalf("SetValue = %v", err)
	}
	if err := Reset(); err != nil {
		t.Fatalf("Reset = %v", err)
	}
	data, err := os.ReadFile(EnvFilePath())
	if err != nil {
		t.Fatalf("ReadFile = %v", err)
	}
	if string(data) != "\n" {
		t.Fatalf("reset bytes = %q, want a single newline", data)
	}
	fresh := filepath.Join(t.TempDir(), "never-created")
	t.Setenv(homeEnvVar, fresh)
	if err := Reset(); err == nil {
		t.Fatal("Reset(missing dir) = nil error, want failed write without side effects")
	}
	if _, statErr := os.Stat(fresh); !os.IsNotExist(statErr) {
		t.Fatal("Reset created the config directory, want no side effect")
	}
}

func TestConfigureDatabaseBackendTable(t *testing.T) {
	for _, tc := range []struct {
		in     string
		want   string
		values map[string]string
	}{
		{"nornicdb", "nornicdb", map[string]string{
			"ESHU_GRAPH_BACKEND":  "nornicdb",
			"DEFAULT_DATABASE":    "nornic",
			"ESHU_NEO4J_DATABASE": "nornic",
		}},
		{" Nornic ", "nornicdb", map[string]string{
			"ESHU_GRAPH_BACKEND":  "nornicdb",
			"DEFAULT_DATABASE":    "nornic",
			"ESHU_NEO4J_DATABASE": "nornic",
		}},
		{"NEO4J", "neo4j", map[string]string{
			"ESHU_GRAPH_BACKEND":  "neo4j",
			"DEFAULT_DATABASE":    "neo4j",
			"ESHU_NEO4J_DATABASE": "neo4j",
		}},
	} {
		isolateHome(t)
		got, err := ConfigureDatabaseBackend(tc.in)
		if err != nil || got != tc.want {
			t.Fatalf("ConfigureDatabaseBackend(%q) = (%q, %v)", tc.in, got, err)
		}
		loaded := Load()
		for k, v := range tc.values {
			if loaded[k] != v {
				t.Fatalf("ConfigureDatabaseBackend(%q)[%q] = %q, want %q (full map %v)",
					tc.in, k, loaded[k], v, loaded)
			}
		}
	}
}

func TestConfigureDatabaseBackendRejects(t *testing.T) {
	isolateHome(t)
	if _, err := ConfigureDatabaseBackend("sqlite"); err == nil ||
		!strings.Contains(err.Error(), "sqlite") {
		t.Fatalf("ConfigureDatabaseBackend(sqlite) = %v, want normalized-value error", err)
	}
	if got := Load(); len(got) != 0 {
		t.Fatalf("rejected backend wrote %v, want nothing", got)
	}
}

func TestConfigureDatabaseBackendWriteFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission fault injection needs a non-root user")
	}
	home := isolateHome(t)
	blocker := filepath.Join(home, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile = %v", err)
	}
	t.Setenv(homeEnvVar, filepath.Join(blocker, "sub"))
	for _, backend := range []string{"neo4j", "nornic"} {
		if _, err := ConfigureDatabaseBackend(backend); err == nil {
			t.Fatalf("ConfigureDatabaseBackend(%q, blocked dir) = nil error", backend)
		}
	}
}

func testRegistry(t *testing.T) *wirecontract.Registry {
	t.Helper()
	registry, err := wirecontract.New([]wirecontract.Entry{
		{Name: "ESHU_SERVICE_URL", Type: wirecontract.VarString},
		{Name: "ESHU_TIMEOUT", Type: wirecontract.VarInt},
		{Name: "ESHU_DEBUG", Type: wirecontract.VarBool},
		{Name: "ESHU_BACKEND", Type: wirecontract.VarEnum, Allowed: []string{"a", "b"}},
		{Name: "ESHU_OLD", Type: wirecontract.VarString, Deprecated: true, ReplacedBy: "ESHU_SERVICE_URL"},
		{Name: "ESHU_ALIASED", Type: wirecontract.VarString, Aliases: []string{"ESHU_ALIAS"}},
	})
	if err != nil {
		t.Fatalf("wirecontract.New = %v", err)
	}
	return registry
}

func TestEnvironMap(t *testing.T) {
	got := EnvironMap([]string{"A=1", "B=2=3", "BARE", "A=over"})
	if len(got) != 2 || got["A"] != "over" || got["B"] != "2=3" {
		t.Fatalf("EnvironMap = %v", got)
	}
}

func TestValidateEnvOK(t *testing.T) {
	var buf bytes.Buffer
	err := ValidateEnv(&buf, testRegistry(t), map[string]string{
		"ESHU_SERVICE_URL": "https://x",
		"ESHU_TIMEOUT":     "30",
		"ESHU_ALIAS":       "v",
		"OTHER":            "ignored",
	}, false)
	if err != nil {
		t.Fatalf("ValidateEnv = %v", err)
	}
	if !strings.Contains(buf.String(), "no ESHU_* configuration problems found") {
		t.Fatalf("report = %q", buf.String())
	}
}

func TestValidateEnvErrorsSortFirst(t *testing.T) {
	var buf bytes.Buffer
	err := ValidateEnv(&buf, testRegistry(t), map[string]string{
		"ESHU_ZZZ":     "x",
		"ESHU_TIMEOUT": "not-a-number",
		"ESHU_BACKEND": "zzz",
		"ESHU_OLD":     "v",
	}, true)
	if err == nil || !strings.Contains(err.Error(), "2 error(s)") {
		t.Fatalf("ValidateEnv = %v, want 2 errors", err)
	}
	out := buf.String()
	errPos := strings.Index(out, "ERROR")
	warnPos := strings.Index(out, "WARN ")
	if errPos < 0 || warnPos < 0 || errPos > warnPos {
		t.Fatalf("errors do not sort ahead of warnings in:\n%s", out)
	}
	if !strings.Contains(out, "2 warning(s)") {
		t.Fatalf("summary missing in:\n%s", out)
	}
}

func TestValidateEnvTypoAndStrict(t *testing.T) {
	registry := testRegistry(t)
	var buf bytes.Buffer
	if err := ValidateEnv(&buf, registry, map[string]string{"ESHU_SERVICE_UR": "x"}, false); err != nil {
		t.Fatalf("ValidateEnv typo path = %v, want warning-only nil", err)
	}
	if !strings.Contains(buf.String(), "did you mean ESHU_SERVICE_URL?") {
		t.Fatalf("typo report = %q, want suggestion", buf.String())
	}
	var strict bytes.Buffer
	if err := ValidateEnv(&strict, registry, map[string]string{"ESHU_FAR_AWAY_UNRELATED": "x"}, true); err != nil {
		t.Fatalf("ValidateEnv strict = %v, want warning-only nil", err)
	}
	if !strings.Contains(strict.String(), "not a registered variable") {
		t.Fatalf("strict report = %q", strict.String())
	}
	var silent bytes.Buffer
	if err := ValidateEnv(&silent, registry, map[string]string{"ESHU_FAR_AWAY_UNRELATED": "x"}, false); err != nil {
		t.Fatalf("ValidateEnv non-strict = %v", err)
	}
	if strings.Contains(silent.String(), "FAR_AWAY") {
		t.Fatalf("non-strict report noisy:\n%s", silent.String())
	}
}
