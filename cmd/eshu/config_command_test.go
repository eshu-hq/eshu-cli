// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// isolateConfigHome points the CLI config at an empty temp home so the
// tests never read or write the operator's real ~/.eshu directory.
func isolateConfigHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ESHU_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
}

// runConfigCommand runs `eshu config <args>` on a fresh command and returns
// what it wrote to the cobra stream and the process stdout.
func runConfigCommand(t *testing.T, args ...string) (cobraOut, processOut string, err error) {
	t.Helper()
	drain := captureProcessStdout(t)
	cmd := newConfigCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), string(drain()), err
}

func TestConfigShowReportsEmptyAndPopulatedConfig(t *testing.T) {
	isolateConfigHome(t)
	_, out, err := runConfigCommand(t, "show")
	if err != nil || !strings.Contains(out, "No configuration found") {
		t.Fatalf("empty config: err=%v out=%q, want the no-configuration message", err, out)
	}

	if _, _, err := runConfigCommand(t, "set", "ESHU_SERVICE_URL", "http://example.test:8080"); err != nil {
		t.Fatalf("config set: %v", err)
	}
	_, out, err = runConfigCommand(t, "show")
	if err != nil {
		t.Fatalf("config show: %v", err)
	}
	for _, want := range []string{"Key", "Value", "ESHU_SERVICE_URL", "http://example.test:8080"} {
		if !strings.Contains(out, want) {
			t.Errorf("config show output lacks %q:\n%s", want, out)
		}
	}
}

func TestConfigSetReportsSuccessAndRejectsBadKeys(t *testing.T) {
	isolateConfigHome(t)
	_, out, err := runConfigCommand(t, "set", "ESHU_API_KEY", "abc")
	if err != nil || !strings.Contains(out, "Set ESHU_API_KEY") {
		t.Errorf("config set: err=%v out=%q, want a success line", err, out)
	}
	if _, _, err := runConfigCommand(t, "set", "only-one-arg"); err == nil {
		t.Error("config set with one argument must fail argument validation")
	}
}

func TestConfigDBSwitchesBackend(t *testing.T) {
	isolateConfigHome(t)
	for _, backend := range []string{"neo4j", "nornicdb"} {
		_, out, err := runConfigCommand(t, "db", backend)
		if err != nil || !strings.Contains(out, "Default database switched to "+backend) {
			t.Errorf("config db %s: err=%v out=%q", backend, err, out)
		}
	}
	if _, _, err := runConfigCommand(t, "db", "oracle"); err == nil {
		t.Error("an unsupported backend must be rejected")
	}
}

// feedStdin replaces os.Stdin with a pipe carrying input for one test.
func feedStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close stdin writer: %v", err)
	}
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old; _ = r.Close() })
}

func TestConfigResetAsksBeforeClearing(t *testing.T) {
	isolateConfigHome(t)
	if _, _, err := runConfigCommand(t, "set", "ESHU_API_KEY", "abc"); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	feedStdin(t, "n\n")
	_, out, err := runConfigCommand(t, "reset")
	if err != nil || !strings.Contains(out, "Reset cancelled") {
		t.Fatalf("declined reset: err=%v out=%q, want the cancelled message", err, out)
	}
	if _, show, _ := runConfigCommand(t, "show"); !strings.Contains(show, "ESHU_API_KEY") {
		t.Errorf("a declined reset must keep the config, show = %q", show)
	}

	feedStdin(t, "y\n")
	if _, _, err := runConfigCommand(t, "reset"); err != nil {
		t.Fatalf("confirmed reset: %v", err)
	}
	if _, show, _ := runConfigCommand(t, "show"); !strings.Contains(show, "No configuration found") {
		t.Errorf("a confirmed reset must clear the config, show = %q", show)
	}

	feedStdin(t, "")
	if _, _, err := runConfigCommand(t, "reset"); err == nil {
		t.Error("reset with no answer on stdin must return the read error")
	}
}

func TestConfigValidateReferenceAndEnvironment(t *testing.T) {
	isolateConfigHome(t)
	out, _, err := runConfigCommand(t, "validate", "--reference")
	if err != nil || !strings.Contains(out, "ESHU_") {
		t.Fatalf("validate --reference: err=%v out=%q, want the generated reference", err, out)
	}

	t.Setenv("ESHU_SERVICE_URL", "http://localhost:8080")
	out, _, err = runConfigCommand(t, "validate")
	if err != nil {
		t.Errorf("validate with a sane environment: err=%v out=%q", err, out)
	}

	t.Setenv("ESHU_TOTALLY_UNKNOWN_SETTING", "1")
	out, _, err = runConfigCommand(t, "validate", "--strict")
	if err != nil || !strings.Contains(out, "WARN") || !strings.Contains(out, "ESHU_TOTALLY_UNKNOWN_SETTING") {
		t.Errorf("validate --strict: err=%v out=%q, want a warning naming the unknown variable and no failure", err, out)
	}

	t.Setenv("ESHU_QUERY_PROFILE", "not-a-profile")
	out, _, err = runConfigCommand(t, "validate")
	if err == nil || !strings.Contains(out, "ERROR") {
		t.Errorf("validate with an invalid known value: err=%v out=%q, want an error finding and a failure", err, out)
	}
}

func TestPrintHelpersWriteToTheirStreams(t *testing.T) {
	drain := captureProcessStdout(t)
	printTable([]string{"A", "B"}, [][]string{{"1", "2"}})
	printSuccess("done")
	out := string(drain())
	for _, want := range []string{"A", "B", "1", "2", "OK done"} {
		if !strings.Contains(out, want) {
			t.Errorf("printTable/printSuccess output lacks %q:\n%s", want, out)
		}
	}

	// printWarning writes to stderr; capture it the same way.
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	printWarning("careful")
	os.Stderr = old
	_ = w.Close()
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "warning: careful") {
		t.Errorf("printWarning wrote %q, want the warning prefix", buf.String())
	}
}

func TestPrintVersionNamesTheBuild(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{Use: "version"}
	cmd.SetOut(&out)
	if err := printVersion(cmd); err != nil || !strings.HasPrefix(out.String(), "Eshu ") {
		t.Errorf("printVersion: err=%v out=%q, want an 'Eshu <version>' line", err, out.String())
	}
}
