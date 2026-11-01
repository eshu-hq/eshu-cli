// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// recordedRequest is one request the stub API server saw.
type recordedRequest struct {
	method string
	path   string
	body   map[string]any
}

// stubAPIServer answers every request with status and a small JSON body and
// records what it was asked. It points ESHU_SERVICE_URL at itself and
// isolates the config lookup from the real home directory, so the commands
// under test talk to nothing but this server. Callers must not run in
// parallel: the environment is process state.
func stubAPIServer(t *testing.T, status int) func() []recordedRequest {
	t.Helper()
	var (
		mu       sync.Mutex
		requests []recordedRequest
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		mu.Lock()
		requests = append(requests, recordedRequest{method: r.Method, path: r.URL.Path, body: body})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"data":{},"truth":{},"error":null}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("ESHU_SERVICE_URL", server.URL)
	t.Setenv("ESHU_API_KEY", "")
	t.Setenv("HOME", t.TempDir())
	return func() []recordedRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]recordedRequest(nil), requests...)
	}
}

// executeRoot runs the shared command tree with args and returns the error
// plus everything written through the cobra streams. printJSON writes to the
// process stdout, which the helper also drains so test output stays quiet.
func executeRoot(t *testing.T, args ...string) error {
	t.Helper()
	drain := captureProcessStdout(t)
	var streams bytes.Buffer
	rootCmd.SetOut(&streams)
	rootCmd.SetErr(&streams)
	rootCmd.SetArgs(args)
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})
	err := rootCmd.Execute()
	_ = drain()
	return err
}

// remoteCommandCases pairs each remote-backed command line with the single
// API call it must make.
var remoteCommandCases = []struct {
	name   string
	args   []string
	method string
	path   string
}{
	{"index-status", []string{"index-status"}, "GET", "/api/v0/index-status"},
	{"list", []string{"list"}, "GET", "/api/v0/repositories"},
	{"stats overall", []string{"stats"}, "GET", "/api/v0/status/pipeline"},
	{"stats repo", []string{"stats", "repository:r_1"}, "GET", "/api/v0/repositories/repository:r_1/stats"},
	{"query", []string{"query", "MATCH (n) RETURN n"}, "POST", "/api/v0/code/language-query"},
	{"ecosystem overview", []string{"ecosystem", "overview"}, "GET", "/api/v0/ecosystem/overview"},
	{"find name", []string{"find", "name", "Handler"}, "POST", "/api/v0/entities/resolve"},
	{"find pattern", []string{"find", "pattern", "Handl"}, "POST", "/api/v0/code/search"},
	{"find type", []string{"find", "type", "function"}, "POST", "/api/v0/code/search"},
	{"find variable", []string{"find", "variable", "x"}, "POST", "/api/v0/code/search"},
	{"find content", []string{"find", "content", "TODO"}, "POST", "/api/v0/content/entities/search"},
	{"find decorator", []string{"find", "decorator", "route"}, "POST", "/api/v0/code/search"},
	{"find argument", []string{"find", "argument", "ctx"}, "POST", "/api/v0/code/search"},
	{"analyze calls", []string{"analyze", "calls", "main"}, "POST", "/api/v0/code/relationships"},
	{"analyze callers", []string{"analyze", "callers", "main"}, "POST", "/api/v0/code/relationships"},
	{"analyze chain", []string{"analyze", "chain", "a", "b"}, "POST", "/api/v0/code/call-chain"},
	{"analyze deps", []string{"analyze", "deps", "mod"}, "POST", "/api/v0/code/relationships"},
	{"analyze tree", []string{"analyze", "tree", "Base"}, "POST", "/api/v0/code/relationships"},
	{"analyze complexity", []string{"analyze", "complexity"}, "POST", "/api/v0/code/complexity"},
	{"analyze dead-code", []string{"analyze", "dead-code"}, "POST", "/api/v0/code/dead-code"},
	{"analyze overrides", []string{"analyze", "overrides", "Do"}, "POST", "/api/v0/code/relationships"},
	{"analyze variable", []string{"analyze", "variable", "x"}, "POST", "/api/v0/code/search"},
}

func TestRemoteCommandsCallTheirRoute(t *testing.T) {
	for _, tc := range remoteCommandCases {
		t.Run(tc.name, func(t *testing.T) {
			requests := stubAPIServer(t, http.StatusOK)
			if err := executeRoot(t, tc.args...); err != nil {
				t.Fatalf("%v: %v", tc.args, err)
			}
			got := requests()
			if len(got) != 1 {
				t.Fatalf("%v made %d requests, want 1: %+v", tc.args, len(got), got)
			}
			if got[0].method != tc.method || got[0].path != tc.path {
				t.Errorf("%v called %s %s, want %s %s", tc.args, got[0].method, got[0].path, tc.method, tc.path)
			}
		})
	}
}

func TestRemoteCommandsSurfaceServerErrors(t *testing.T) {
	for _, tc := range remoteCommandCases {
		t.Run(tc.name, func(t *testing.T) {
			stubAPIServer(t, http.StatusInternalServerError)
			if err := executeRoot(t, tc.args...); err == nil {
				t.Errorf("%v returned nil error against a 500 server", tc.args)
			}
		})
	}
}

func TestQueryCommandSendsTheQueryText(t *testing.T) {
	requests := stubAPIServer(t, http.StatusOK)
	if err := executeRoot(t, "query", "MATCH (n) RETURN n"); err != nil {
		t.Fatalf("query: %v", err)
	}
	got := requests()
	if len(got) != 1 || got[0].body["query"] != "MATCH (n) RETURN n" {
		t.Errorf("query body = %+v, want the raw query text", got)
	}
}

func TestRemovedCommandsFailWithoutCallingTheAPI(t *testing.T) {
	cases := [][]string{
		{"ecosystem", "index"},
		{"ecosystem", "status"},
		{"delete", "somewhere"},
		{"delete", "--all"},
		{"clean"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, "-"), func(t *testing.T) {
			requests := stubAPIServer(t, http.StatusOK)
			if err := executeRoot(t, args...); err == nil {
				t.Errorf("%v returned nil, want a removed-command error", args)
			}
			if got := requests(); len(got) != 0 {
				t.Errorf("%v made API calls %+v, want none", args, got)
			}
		})
	}
}

func TestDeleteWithoutArgumentsExplainsUsage(t *testing.T) {
	stubAPIServer(t, http.StatusOK)
	// --all is sticky on the shared command tree, so state it explicitly.
	err := executeRoot(t, "delete", "--all=false")
	if err == nil || !strings.Contains(err.Error(), "missing path") {
		t.Errorf("delete with no args = %v, want the missing path error", err)
	}
}

func TestNormalizeRepositoryStatsSelector(t *testing.T) {
	if got, err := normalizeRepositoryStatsSelector(""); got != "" || err != nil {
		t.Errorf("empty selector = (%q, %v), want empty", got, err)
	}
	if got, err := normalizeRepositoryStatsSelector("repository:not-a-path"); got != "repository:not-a-path" || err != nil {
		t.Errorf("non-path selector = (%q, %v), want it unchanged", got, err)
	}
	dir := t.TempDir()
	got, err := normalizeRepositoryStatsSelector(dir)
	if err != nil || got != dir {
		t.Errorf("existing path = (%q, %v), want the absolute path %q", got, err, dir)
	}
}
