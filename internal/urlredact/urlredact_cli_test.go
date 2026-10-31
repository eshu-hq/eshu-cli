// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package urlredact

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestDecodedByteAtTable(t *testing.T) {
	for _, tc := range []struct {
		name      string
		s         string
		i         int
		wantByte  byte
		wantWidth int
	}{
		{"negative", "abc", -1, 0, 0},
		{"past end", "abc", 3, 0, 0},
		{"empty", "", 0, 0, 0},
		{"plain", "abc", 1, 'b', 1},
		{"escape", "a%3Db", 1, '=', EscapeWidth},
		{"lower hex", "a%3db", 1, '=', EscapeWidth},
		{"space", "%20", 0, ' ', EscapeWidth},
		{"lone percent", "100%", 3, '%', 1},
		{"truncated", "a%4", 1, '%', 1},
		{"bad high", "a%ZZ", 1, '%', 1},
		{"bad low", "a%4Z", 1, '%', 1},
		{"bad high 2", "a%Z4", 1, '%', 1},
	} {
		gotByte, gotWidth := DecodedByteAt(tc.s, tc.i)
		if gotByte != tc.wantByte || gotWidth != tc.wantWidth {
			t.Fatalf("%s: DecodedByteAt(%q, %d) = (%q, %d), want (%q, %d)",
				tc.name, tc.s, tc.i, gotByte, gotWidth, tc.wantByte, tc.wantWidth)
		}
	}
}

func TestDecodedEscapeBefore(t *testing.T) {
	if _, ok := DecodedEscapeBefore("ab", 2); ok {
		t.Fatal("DecodedEscapeBefore short = true, want false")
	}
	if _, ok := DecodedEscapeBefore("ab", 9); ok {
		t.Fatal("DecodedEscapeBefore past end = true, want false")
	}
	got, ok := DecodedEscapeBefore("ab%3D", 5)
	if !ok || got != '=' {
		t.Fatalf("DecodedEscapeBefore = (%q, %v), want ('=', true)", got, ok)
	}
	if _, ok := DecodedEscapeBefore("abc", 3); ok {
		t.Fatal("DecodedEscapeBefore plain = true, want false")
	}
	if _, ok := DecodedEscapeBefore("a%ZZ", 4); ok {
		t.Fatal("DecodedEscapeBefore malformed = true, want false")
	}
}

func TestIndexBoundaryDepths(t *testing.T) {
	if i, w := IndexBoundary("a=b", "=", LiteralOnly); i != 1 || w != 1 {
		t.Fatalf("LiteralOnly = (%d, %d), want (1, 1)", i, w)
	}
	if i, w := IndexBoundary("abc", "=", LiteralOnly); i != -1 || w != 0 {
		t.Fatalf("LiteralOnly miss = (%d, %d), want (-1, 0)", i, w)
	}
	if i, w := IndexBoundary("a%3Db", "=", LiteralOrEscaped); i != 1 || w != EscapeWidth {
		t.Fatalf("LiteralOrEscaped = (%d, %d), want (1, 3)", i, w)
	}
	if i, w := IndexBoundary("a=b", "=", LiteralOrEscaped); i != 1 || w != 1 {
		t.Fatalf("LiteralOrEscaped literal = (%d, %d), want (1, 1)", i, w)
	}
	if i, w := IndexBoundary("abc", "=", LiteralOrEscaped); i != -1 || w != 0 {
		t.Fatalf("LiteralOrEscaped miss = (%d, %d), want (-1, 0)", i, w)
	}
	if i, w := IndexBoundary("", "=", LiteralOrEscaped); i != -1 || w != 0 {
		t.Fatalf("empty = (%d, %d), want (-1, 0)", i, w)
	}
}

func TestIndexBoundaryBySpellingSets(t *testing.T) {
	if i, _ := IndexBoundaryBySpelling("a?b", "&", "?"); i != -1 {
		t.Fatalf("escaped-only byte as literal matched at %d", i)
	}
	if i, w := IndexBoundaryBySpelling("a%3Fb", "&?", "&?"); i != 1 || w != EscapeWidth {
		t.Fatalf("escaped set = (%d, %d), want (1, 3)", i, w)
	}
	if i, w := IndexBoundaryBySpelling("a&b", "&?", "&?"); i != 1 || w != 1 {
		t.Fatalf("literal set = (%d, %d), want (1, 1)", i, w)
	}
}

func TestDecodeTable(t *testing.T) {
	got, ok := Decode("a%3Db")
	if !ok || got != "a=b" {
		t.Fatalf("Decode = (%q, %v), want (\"a=b\", true)", got, ok)
	}
	got, ok = Decode("abc")
	if ok || got != "abc" {
		t.Fatalf("Decode plain = (%q, %v), want (\"abc\", false)", got, ok)
	}
	got, ok = Decode("%ZZ")
	if ok || got != "%ZZ" {
		t.Fatalf("Decode malformed = (%q, %v), want (\"%%ZZ\", false)", got, ok)
	}
	got, ok = Decode("a+b")
	if ok || got != "a+b" {
		t.Fatalf("Decode plus = (%q, %v), want (\"a+b\", false): plus is not an escape here", got, ok)
	}
	got, ok = Decode("a%2Bb")
	if !ok || got != "a+b" {
		t.Fatalf("Decode escaped plus = (%q, %v), want (\"a+b\", true)", got, ok)
	}
}

func TestQueryEncodedNameAndEscapedSeparator(t *testing.T) {
	if got := Query("tok%65n=SECRET&x=1", "redacted"); got != "tok%65n=redacted&x=1" {
		t.Fatalf("Query(encoded name) = %q", got)
	}
	if got := Query("api_key=AB%26CD&x=1", "redacted"); got != "api_key=redacted&x=1" {
		t.Fatalf("Query(escaped separator in value) = %q", got)
	}
	if got := Query("x=1&api_key=", "redacted"); got != "x=1&api_key=" {
		t.Fatalf("Query(trailing empty value) = %q, want untouched", got)
	}
}

func TestUnhexTable(t *testing.T) {
	for c, want := range map[byte]byte{'0': 0, '9': 9, 'a': 10, 'f': 15, 'A': 10, 'F': 15} {
		got, ok := unhex(c)
		if !ok || got != want {
			t.Fatalf("unhex(%q) = (%d, %v), want (%d, true)", c, got, ok, want)
		}
	}
	if _, ok := unhex('g'); ok {
		t.Fatal("unhex('g') = true, want false")
	}
}

func TestAuthorityShapes(t *testing.T) {
	parsed, rewritten, err := Authority("https://alice:xyzzy9@h.internal:5432/tool")
	if err != nil || rewritten {
		t.Fatalf("Authority(direct) = (rewritten=%v, %v), want plain parse", rewritten, err)
	}
	if parsed.User == nil {
		t.Fatal("Authority(direct) lost userinfo")
	}
	parsed, rewritten, err = Authority("svc:alice@h.internal:5432/tool")
	if err != nil || !rewritten {
		t.Fatalf("Authority(opaque) = (rewritten=%v, %v), want rewrite", rewritten, err)
	}
	if parsed.User == nil {
		t.Fatal("Authority(opaque) lost userinfo through the rewrite")
	}
	for _, plain := range []string{"mcp:tool/name", "pkg:npm/lodash@4.17.21", "https://h.internal/tool"} {
		parsed, rewritten, err = Authority(plain)
		if err != nil || rewritten {
			t.Fatalf("Authority(%q) = (rewritten=%v, %v), want plain parse", plain, rewritten, err)
		}
		if parsed.User != nil {
			t.Fatalf("Authority(%q) reports userinfo, want none", plain)
		}
	}
}

func TestAuthorityParseErrorHidesValue(t *testing.T) {
	_, rewritten, err := Authority("http://[::1")
	if err == nil || rewritten {
		t.Fatalf("Authority(bad) = (rewritten=%v, %v), want parse error", rewritten, err)
	}
	if !strings.Contains(err.Error(), "missing ']' in host") {
		t.Fatalf("Authority error = %q, want the static reason", err)
	}
	if strings.Contains(err.Error(), "[::1") {
		t.Fatalf("Authority error repeats the value: %q", err)
	}
}

func TestAuthorityRewriteError(t *testing.T) {
	_, rewritten, err := Authority("svc:%zz@h.internal/")
	if err == nil || !rewritten {
		t.Fatalf("Authority(bad rewrite) = (rewritten=%v, %v), want rewrite error", rewritten, err)
	}
	if !strings.Contains(err.Error(), "parse value authority:") {
		t.Fatalf("Authority error = %q, want the authority envelope", err)
	}
}

func TestCarriesUserinfoTable(t *testing.T) {
	for value, want := range map[string]bool{
		"https://alice:xyzzy9@h.internal/tool": true,
		"svc:alice@h.internal:5432/tool":       true,
		"http://[::1":                          true,
		"https://h.internal/tool":              false,
		"mcp:tool/name":                        false,
		"mailto:user@example.com":              true,
	} {
		if got := CarriesUserinfo(value); got != want {
			t.Fatalf("CarriesUserinfo(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestOpaqueHasAuthorityTable(t *testing.T) {
	for opaque, want := range map[string]bool{
		"":         false,
		"abc":      false,
		"a/b@c":    false,
		"a@b/c":    true,
		"a@b":      true,
		"pass@h:1": true,
	} {
		if got := opaqueHasAuthority(opaque); got != want {
			t.Fatalf("opaqueHasAuthority(%q) = %v, want %v", opaque, got, want)
		}
	}
}

func TestErrWithoutValue(t *testing.T) {
	boom := errors.New("boom")
	if err := errWithoutValue(boom); err != boom {
		t.Fatalf("errWithoutValue(plain) = %v, want passthrough", err)
	}
	// Built from a variable: staticcheck rejects a constant invalid URL
	// here, while the test needs exactly the parse failure it produces.
	badHost := "[::1"
	_, parseErr := url.Parse("http://" + badHost)
	if parseErr == nil {
		t.Fatal("setup: expected a parse error")
	}
	got := errWithoutValue(parseErr)
	if !strings.Contains(got.Error(), "parse:") || strings.Contains(got.Error(), "[::1") {
		t.Fatalf("errWithoutValue = %q, want op plus reason without the value", got)
	}
}

func TestParseErrorReasonTable(t *testing.T) {
	if got := ParseErrorReason(url.EscapeError("%zz")); got != "invalid URL escape" {
		t.Fatalf("EscapeError = %q", got)
	}
	if got := ParseErrorReason(&url.Error{Op: "parse", Err: url.EscapeError("%zz")}); got != "invalid URL escape" {
		t.Fatalf("wrapped EscapeError = %q", got)
	}
	if got := ParseErrorReason(url.InvalidHostError("bad host!")); got != "invalid character in host name" {
		t.Fatalf("InvalidHostError = %q", got)
	}
	for _, msg := range []string{
		"missing protocol scheme",
		"empty url",
		"invalid URI for request",
		"first path segment in URL cannot contain colon",
		"net/url: invalid control character in URL",
		"net/url: invalid userinfo",
		"invalid IP-literal",
		"missing ']' in host",
	} {
		if got := ParseErrorReason(errors.New(msg)); got != msg {
			t.Fatalf("static %q mapped to %q", msg, got)
		}
	}
	if got := ParseErrorReason(errors.New(`invalid port ":xx" after host`)); got != "invalid port after host" {
		t.Fatalf("port = %q", got)
	}
	if got := ParseErrorReason(errors.New("invalid host: some netip detail")); got != "invalid host" {
		t.Fatalf("host = %q", got)
	}
	if got := ParseErrorReason(errors.New("some future message")); got != "malformed URL" {
		t.Fatalf("unknown = %q, want fail-closed generic", got)
	}
}
