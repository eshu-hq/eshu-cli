// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAPIClientSendsBearerToken pins the Proof-table bearer-carriage
// figure (Refs eshu-hq/eshu-cli#23): when the client carries an API key,
// every request it builds carries it as an `Authorization: Bearer` header.
// The stub-server AUTH log is gitignored, so without this test the 22/22
// count has no committed provenance.
func TestAPIClientSendsBearerToken(t *testing.T) {
	var gotAuth []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = append(gotAuth, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client := NewAPIClient(srv.URL, "test-token", "")
	var result map[string]any
	if err := client.Get("/api/v0/repositories", &result); err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}
	if len(gotAuth) != 1 {
		t.Fatalf("server saw %d requests, want 1", len(gotAuth))
	}
	if want := "Bearer test-token"; gotAuth[0] != want {
		t.Fatalf("Authorization = %q, want %q", gotAuth[0], want)
	}
}

// TestAPIClientOmitsBearerTokenWhenKeyless pins the other half of the
// contract: with no API key configured, no Authorization header is sent,
// so unauthenticated local/dev-mode traffic stays headerless.
func TestAPIClientOmitsBearerTokenWhenKeyless(t *testing.T) {
	var gotAuth []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = append(gotAuth, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	// Isolate both credential sources: the .env home and the env fallback.
	t.Setenv("ESHU_HOME", t.TempDir())
	t.Setenv("ESHU_API_KEY", "")
	client := NewAPIClient(srv.URL, "", "")
	var result map[string]any
	if err := client.Get("/api/v0/repositories", &result); err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}
	if len(gotAuth) != 1 {
		t.Fatalf("server saw %d requests, want 1", len(gotAuth))
	}
	if gotAuth[0] != "" {
		t.Fatalf("Authorization = %q, want empty", gotAuth[0])
	}
}
