// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package graphinstall

import (
	"errors"
	"path/filepath"
	"testing"
)

func homeEnv(pairs ...string) func(string) string {
	return func(key string) string {
		for i := 0; i+1 < len(pairs); i += 2 {
			if pairs[i] == key {
				return pairs[i+1]
			}
		}
		return ""
	}
}

func fixedHome(home string) func() (string, error) {
	return func() (string, error) { return home, nil }
}

func TestResolveHomeDirESHUHomeWinsVerbatim(t *testing.T) {
	got, err := resolveHomeDir(homeEnv("ESHU_HOME", "/custom/eshu-home"), fixedHome("/home/op"), "linux")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	// No "eshu" segment is appended to ESHU_HOME: it names the home itself.
	if got != "/custom/eshu-home" {
		t.Fatalf("got %q, want verbatim ESHU_HOME", got)
	}
}

func TestResolveHomeDirESHUHomeExpandsTilde(t *testing.T) {
	got, err := resolveHomeDir(homeEnv("ESHU_HOME", "~/managed"), fixedHome("/home/op"), "linux")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	want := filepath.Join("/home/op", "managed")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolveHomeDirPlatformDefaults(t *testing.T) {
	cases := []struct {
		name string
		goos string
		env  func(string) string
		home string
		want string
	}{
		{"darwin", "darwin", homeEnv(), "/Users/op", filepath.Join("/Users/op", "Library", "Application Support", "eshu")},
		{"linux-xdg", "linux", homeEnv("XDG_DATA_HOME", "/data/xdg"), "/home/op", filepath.Join("/data/xdg", "eshu")},
		{"linux-home", "linux", homeEnv(), "/home/op", filepath.Join("/home/op", ".local", "share", "eshu")},
		{"windows-localappdata", "windows", homeEnv("LOCALAPPDATA", `C:\Local`), `C:\Users\op`, filepath.Join(`C:\Local`, "eshu")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveHomeDir(tc.env, fixedHome(tc.home), tc.goos)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveHomeDirWindowsFallsBackToProfile(t *testing.T) {
	got, err := resolveHomeDir(homeEnv(), fixedHome(`C:\Users\op`), "windows")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	want := filepath.Join(`C:\Users\op`, "AppData", "Local", "eshu")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolveHomeDirPropagatesHomeError(t *testing.T) {
	failing := func() (string, error) { return "", errors.New("no home") }
	if _, err := resolveHomeDir(homeEnv(), failing, "darwin"); err == nil {
		t.Fatal("want error when the user home is unavailable, got nil")
	}
	if _, err := resolveHomeDir(homeEnv("ESHU_HOME", "~/x"), failing, "linux"); err == nil {
		t.Fatal("want error when ~ expands with no home, got nil")
	}
}

func TestExpandHomeLeavesNonTildeVerbatim(t *testing.T) {
	for _, path := range []string{"/abs/path", "relative/path", ""} {
		got, err := expandHome(path, fixedHome("/home/op"))
		if err != nil {
			t.Fatalf("expand %q: %v", path, err)
		}
		if got != path {
			t.Fatalf("expand %q = %q, want verbatim", path, got)
		}
	}
}
