// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package evidredact_test

import (
	"strings"
	"testing"

	"github.com/eshu-hq/eshu-cli/internal/cli/evidredact"
)

// Tests for the Tier-2 evidredact copy pin the frozen source at pin
// 78c5b3d1a4 (docs/tier2-pin.md); this test is destination-authored, so
// it is excluded from the no-fork freeze and pinned here instead, per the
// coverage-ratchet standing ruling (eshu-hq/eshu-cli#6).
func TestEndpointRemovesUserinfoKeepsTarget(t *testing.T) {
	got := evidredact.Endpoint("https://user:s3cret@example.com:8443/v0/items?page=2")
	if strings.Contains(got, "s3cret") || strings.Contains(got, "user:") {
		t.Fatalf("Endpoint kept credential material: %q", got)
	}
	for _, want := range []string{"example.com:8443", "/v0/items", "page=2"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Endpoint(%q) lost recognizability %q", got, want)
		}
	}
}

func TestEndpointRedactsCredentialQueryAndDropsFragment(t *testing.T) {
	got := evidredact.Endpoint("https://app.example.com/cb?api_key=topsecret&next=/v0/y#access_token= fragtok")
	if strings.Contains(got, "topsecret") || strings.Contains(got, "fragtok") {
		t.Fatalf("Endpoint kept credential material: %q", got)
	}
	if strings.Contains(got, "#") {
		t.Fatalf("Endpoint kept a fragment: %q", got)
	}
	if !strings.Contains(got, "api_key="+evidredact.Marker) {
		t.Fatalf("Endpoint did not mark the credential query value: %q", got)
	}
	if !strings.Contains(got, "next=") {
		t.Fatalf("Endpoint dropped a benign query pair: %q", got)
	}
}

func TestEndpointBlankAndUnparseable(t *testing.T) {
	for _, raw := range []string{"", "   "} {
		if got := evidredact.Endpoint(raw); got != "" {
			t.Fatalf("Endpoint(%q) = %q, want empty", raw, got)
		}
	}
	got := evidredact.Endpoint("not a url at all")
	if got == "not a url at all" || got == "" {
		t.Fatalf("Endpoint(non-URL) = %q, want masked fallback", got)
	}
}

func TestPathKeepsOnlyFinalElementOfAbsolute(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"", ""},
		{"   ", ""},
		{"relative/name", "relative/name"},
		{"bare", "bare"},
		{"/home/operator/.config/eshu/key", ".../key"},
		{"/", ".../"},
	} {
		if got := evidredact.Path(tc.in); got != tc.want {
			t.Fatalf("Path(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRedactTokenMasksByLength(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"", ""},
		{"   ", ""},
		{"short", "*****"},
		{"12345678", "********"},
		{"123456789", "1234*****"},
		{"abcdefghijklmnopqrstuvw", "abcd*******************"},
	} {
		if got := evidredact.RedactToken(tc.in); got != tc.want {
			t.Fatalf("RedactToken(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if got := evidredact.RedactToken("  secret12345  "); got != "secr*******" {
		t.Fatalf("RedactToken(padded) = %q, want %q", got, "secr*******")
	}
}
