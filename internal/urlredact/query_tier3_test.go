// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package urlredact

import (
	"strings"
	"testing"
)

// TestQueryRedactsCredentialNames pins the Tier-3 urlredact.Query copy:
// every credential-named pair value is replaced while names, order, and
// the separators the URL arrived with survive. query.go is a verbatim
// freeze copy (docs/tier3-pin.md); this test is destination-authored, so
// it is excluded from the no-fork freeze and pinned here instead.
func TestQueryRedactsCredentialNames(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{"ampersand", "a=1&token=SECRET", "a=1&token=redacted"},
		{"semicolon", "a=1;api_key=SECRET", "a=1;api_key=redacted"},
		{"nested", "next=/v0/y?api_key=SECRET", "next=/v0/y?api_key=redacted"},
		{"public untouched", "a=1&repo=demo/app", "a=1&repo=demo/app"},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Query(tc.raw, "redacted"); got != tc.want {
				t.Fatalf("Query(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

// TestQueryLeavesBareNamesAlone pins the no-value rule: a credential name
// with no "=" and no value carries nothing to remove, so rewriting it
// would report a credential the URL never carried.
func TestQueryLeavesBareNamesAlone(t *testing.T) {
	if got := Query("token", "redacted"); got != "token" {
		t.Fatalf("Query(bare) = %q, want it unchanged", got)
	}
	if strings.Contains(Query("a=1&token=", "redacted"), "redacted") {
		t.Fatalf("Query(empty value) rewrote a pair with no value")
	}
}
