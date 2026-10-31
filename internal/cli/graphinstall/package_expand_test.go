// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package graphinstall

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The #32 hole in one premise: a small highly-compressible artifact passes
// both #30 pre-expansion legs. The fixture is a gzip bomb in miniature --
// zeros compress ~1000x (64 MiB of zeros gzips to ~64 KiB) -- so a few-KiB
// .pkg stands in for megabytes of expansion while both legs only see the
// compressed size and the current temp free room.
func TestPreExpansionBoundPassesHighlyCompressibleArtifact(t *testing.T) {
	oldFree := tempVolumeFreeBytes
	tempVolumeFreeBytes = func(dir string) (uint64, error) { return 1 << 40, nil }
	t.Cleanup(func() { tempVolumeFreeBytes = oldFree })

	pkg := filepath.Join(t.TempDir(), "NornicDB.pkg")
	if err := os.WriteFile(pkg, make([]byte, 8<<10), 0o644); err != nil {
		t.Fatalf("stage compressible package: %v", err)
	}
	if err := checkPackageExpansionBound(pkg); err != nil {
		t.Fatalf("pre-expansion bound refused a small under-cap artifact: %v", err)
	}
}

// fakeExpandCommand stubs the pkgutil subprocess seam with the test binary
// itself running TestExpandHelperProcess, so expansion-time enforcement is
// pinned on Linux where pkgutil does not exist.
func fakeExpandCommand(t *testing.T, mode string) {
	t.Helper()
	oldNew := newExpandCommand
	newExpandCommand = func(pkgPath, targetDir string) *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=TestExpandHelperProcess")
		cmd.Env = append(os.Environ(),
			"ESHU_EXPAND_HELPER=1",
			"ESHU_EXPAND_MODE="+mode,
			"ESHU_EXPAND_DIR="+targetDir,
		)
		return cmd
	}
	t.Cleanup(func() { newExpandCommand = oldNew })
}

// TestExpandHelperProcess is not a test: it is the fake pkgutil subprocess
// the bounded-expansion tests above drive through newExpandCommand.
func TestExpandHelperProcess(t *testing.T) {
	if os.Getenv("ESHU_EXPAND_HELPER") != "1" {
		return
	}
	dir := os.Getenv("ESHU_EXPAND_DIR")
	const chunk = 256 << 10
	writeChunks := func(n int) {
		f, err := os.OpenFile(filepath.Join(dir, "big"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			os.Exit(3)
		}
		defer func() { _ = f.Close() }()
		buf := make([]byte, chunk)
		for i := 0; i < n; i++ {
			if _, err := f.Write(buf); err != nil {
				os.Exit(3)
			}
			if os.Getenv("ESHU_EXPAND_MODE") == "gradual" {
				time.Sleep(200 * time.Millisecond)
			}
		}
	}
	switch os.Getenv("ESHU_EXPAND_MODE") {
	case "gradual":
		writeChunks(8) // 2 MiB total, slowly: the watcher must kill this.
	case "fast":
		writeChunks(8) // 2 MiB at once: over before the first poll tick.
	case "small":
		writeChunks(1)
	case "fail":
		_, _ = os.Stderr.WriteString("fake pkgutil boom")
		os.Exit(1)
	}
	os.Exit(0)
}

func TestBoundedExpansionKillsOvergrownTree(t *testing.T) {
	// A zip-bomb-like expansion that grows past the bound mid-run must be
	// killed, not ridden to completion: the error names the expansion
	// bound, and the tree left behind stays short of the bomb's full size.
	fakeExpandCommand(t, "gradual")
	dir := t.TempDir()
	target := filepath.Join(dir, "expanded")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("stage target: %v", err)
	}
	cmd := newExpandCommand("NornicDB.pkg", target)
	if err := runBoundedExpansion(cmd, "NornicDB.pkg", target, 1<<20, 25*time.Millisecond); err == nil ||
		!strings.Contains(err.Error(), "expansion bound") {
		t.Fatalf("got %v, want expansion-bound refusal", err)
	}
	size, statErr := expandedDirSize(target)
	if statErr != nil {
		t.Fatalf("size expanded tree: %v", statErr)
	}
	if size >= 2<<20 {
		t.Fatalf("expansion ran to %d bytes; the overgrown subprocess was not killed", size)
	}
}

func TestBoundedExpansionRefusesFastOverCapTree(t *testing.T) {
	// A bomb that finishes between two poll ticks must still be refused by
	// the post-exit tree check: no poll interval can catch everything.
	fakeExpandCommand(t, "fast")
	target := filepath.Join(t.TempDir(), "expanded")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("stage target: %v", err)
	}
	cmd := newExpandCommand("NornicDB.pkg", target)
	if err := runBoundedExpansion(cmd, "NornicDB.pkg", target, 1<<20, 25*time.Millisecond); err == nil ||
		!strings.Contains(err.Error(), "expansion bound") {
		t.Fatalf("got %v, want expansion-bound refusal", err)
	}
}

func TestBoundedExpansionAllowsUnderBoundTree(t *testing.T) {
	// The bound must not break the normal path: a small expansion that
	// exits cleanly under the bound succeeds.
	fakeExpandCommand(t, "small")
	target := filepath.Join(t.TempDir(), "expanded")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("stage target: %v", err)
	}
	cmd := newExpandCommand("NornicDB.pkg", target)
	if err := runBoundedExpansion(cmd, "NornicDB.pkg", target, 1<<20, 25*time.Millisecond); err != nil {
		t.Fatalf("under-bound expansion refused: %v", err)
	}
}

func TestBoundedExpansionReportsSubprocessFailure(t *testing.T) {
	// A failing expander keeps the old contract: the error carries the
	// subprocess output so the operator sees why pkgutil died.
	fakeExpandCommand(t, "fail")
	target := filepath.Join(t.TempDir(), "expanded")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("stage target: %v", err)
	}
	cmd := newExpandCommand("NornicDB.pkg", target)
	err := runBoundedExpansion(cmd, "NornicDB.pkg", target, 1<<20, 25*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "fake pkgutil boom") {
		t.Fatalf("got %v, want subprocess output in the error", err)
	}
}
