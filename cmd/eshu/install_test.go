// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureProcessStdout redirects the process stdout fd into a pipe and
// returns a reader func that restores stdout and drains what RunE printed.
// printJSON writes to os.Stdout (the Tier-3 workspace precedent), so the
// install result is asserted on bytes, not on a buffer the command owns.
func captureProcessStdout(t *testing.T) func() []byte {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })
	return func() []byte {
		if err := w.Close(); err != nil {
			t.Fatalf("close pipe writer: %v", err)
		}
		out, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("read pipe: %v", err)
		}
		return out
	}
}

func TestInstallCommandRegisteredWithCoreContract(t *testing.T) {
	installCmd, _, err := rootCmd.Find([]string{"install"})
	if err != nil {
		t.Fatalf("find install: %v", err)
	}
	if installCmd == nil || installCmd.Name() != "install" {
		t.Fatal("install command not registered")
	}
	nornicdbCmd, _, err := rootCmd.Find([]string{"install", "nornicdb"})
	if err != nil {
		t.Fatalf("find install nornicdb: %v", err)
	}
	if nornicdbCmd == nil || nornicdbCmd.Name() != "nornicdb" {
		t.Fatal("install nornicdb command not registered")
	}
	if nornicdbCmd.Short != "Install the local NornicDB binary" {
		t.Fatalf("Short = %q, want the core contract text", nornicdbCmd.Short)
	}
	for _, flag := range []string{"from", "sha256", "force", "full"} {
		if nornicdbCmd.Flags().Lookup(flag) == nil {
			t.Fatalf("install nornicdb misses --%s", flag)
		}
	}
}

func TestInstallNornicDBRejectsArgs(t *testing.T) {
	cmd := newInstallNornicDBCommand()
	if err := runInstallNornicDB(cmd, []string{"extra"}); err == nil ||
		!strings.Contains(err.Error(), "accepts flags only") {
		t.Fatalf("got %v, want flags-only error", err)
	}
}

func TestInstallNornicDBRejectsFullWithFrom(t *testing.T) {
	// Flags resolve through a fresh command so global flag state cannot leak.
	fresh := newInstallNornicDBCommand()
	if err := fresh.Flags().Set("from", "/tmp/nornicdb-headless"); err != nil {
		t.Fatalf("set from: %v", err)
	}
	if err := fresh.Flags().Set("full", "true"); err != nil {
		t.Fatalf("set full: %v", err)
	}
	if err := runInstallNornicDB(fresh, nil); err == nil ||
		!strings.Contains(err.Error(), "--full is reserved") {
		t.Fatalf("got %v, want --full reservation error", err)
	}
}

func TestInstallNornicDBInstallsFakeBinary(t *testing.T) {
	home := t.TempDir()
	t.Setenv("ESHU_HOME", home)
	srcDir := t.TempDir()
	fake := filepath.Join(srcDir, "nornicdb-headless")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho \"NornicDB 6.6.6\"\n"), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	fresh := newInstallNornicDBCommand()
	if err := fresh.Flags().Set("from", fake); err != nil {
		t.Fatalf("set from: %v", err)
	}
	// printJSON renders to process stdout, so the test captures the fd
	// around RunE and restores it before asserting.
	stdout := captureProcessStdout(t)
	if err := runInstallNornicDB(fresh, nil); err != nil {
		t.Fatalf("install: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(stdout(), &result); err != nil {
		t.Fatalf("decode result JSON: %v", err)
	}
	if result["installed"] != true || result["version"] != "6.6.6" || result["backend"] != "nornicdb" {
		t.Fatalf("result = %v, want installed 6.6.6 nornicdb", result)
	}
	want := filepath.Join(home, "bin", "nornicdb-headless")
	if result["binary_path"] != want {
		t.Fatalf("binary_path = %v, want %q", result["binary_path"], want)
	}
}
