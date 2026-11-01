// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"bytes"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/eshu-hq/eshu-cli/internal/cli/change"
)

// stubChangeFetch replaces the two fetch indirections for one test and
// restores them afterwards. Tests using it must not run in parallel.
func stubChangeFetch(t *testing.T, impact, plan func(*APIClient, change.Options) (change.Envelope, error)) {
	t.Helper()
	origImpact, origPlan := changeImpactFetch, changePlanFetch
	changeImpactFetch, changePlanFetch = impact, plan
	t.Cleanup(func() { changeImpactFetch, changePlanFetch = origImpact, origPlan })
}

// runChangeCommand builds a fresh change subcommand, applies args, and
// returns what RunE produced.
func runChangeCommand(t *testing.T, build func() *cobra.Command, args ...string) (stdout string, err error) {
	t.Helper()
	cmd := build()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), err
}

func exitCodeOf(t *testing.T, err error) int {
	t.Helper()
	var exit commandExitError
	if !errors.As(err, &exit) {
		t.Fatalf("error %v (%T) is not a commandExitError", err, err)
	}
	return exit.code
}

func TestChangeImpactOptionsFromCommandParsesEveryFlag(t *testing.T) {
	cmd := newChangeImpactCommand()
	args := []string{
		"--json", "--repo-id", " repo-1 ", "--base", " main ", "--head", " HEAD ",
		"--file", " a.go ", "--file", "b/c.go", "--repo-path", " /tmp/r ",
		"--target", " T ", "--target-type", " function ", "--service-name", " svc ",
		"--workload-id", " wl ", "--resource-id", " res ", "--module-id", " mod ",
		"--topic", " topic ", "--env", " prod ", "--max-depth", "6", "--limit", "10", "--offset", "3",
	}
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	opts, err := changeImpactOptionsFromCommand(cmd)
	if err != nil {
		t.Fatalf("changeImpactOptionsFromCommand: %v", err)
	}
	want := map[string]any{
		"RepoID": "repo-1", "BaseRef": "main", "HeadRef": "HEAD", "RepoPath": "/tmp/r",
		"Target": "T", "TargetType": "function", "ServiceName": "svc", "WorkloadID": "wl",
		"ResourceID": "res", "ModuleID": "mod", "Topic": "topic", "Environment": "prod",
		"MaxDepth": 6, "Limit": 10, "Offset": 3, "JSON": true,
	}
	got := reflect.ValueOf(opts)
	for field, w := range want {
		if g := got.FieldByName(field).Interface(); g != w {
			t.Errorf("options.%s = %v, want %v", field, g, w)
		}
	}
	if len(opts.Changes) != 2 || len(opts.ChangedPaths) != 2 {
		t.Errorf("changes = %v paths = %v, want two entries each", opts.Changes, opts.ChangedPaths)
	}
}

func TestChangeImpactOptionsFromCommandRejectsMissingFlagDefinitions(t *testing.T) {
	// A bare command without the change flags must surface the flag lookup
	// error instead of silently using zero values.
	cmd := &cobra.Command{Use: "bare"}
	if _, err := changeImpactOptionsFromCommand(cmd); err == nil {
		t.Fatal("changeImpactOptionsFromCommand on a flagless command returned nil error")
	}
}

func TestRunChangeImpactExitCodes(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		envelope change.Envelope
		fetchErr error
		wantCode int // 0 means success
		wantCall bool
	}{
		{name: "no selector is invalid", args: nil, wantCode: 2},
		{name: "changed file without repo id is invalid", args: []string{"--file", "a.go"}, wantCode: 2},
		{name: "limit out of range is invalid", args: []string{"--target", "x", "--limit", "500"}, wantCode: 2},
		{name: "depth out of range is invalid", args: []string{"--target", "x", "--max-depth", "9"}, wantCode: 2},
		{name: "clean answer succeeds", args: []string{"--target", "x"}, wantCall: true},
		{
			name: "stale freshness exits 4", args: []string{"--target", "x"}, wantCall: true, wantCode: 4,
			envelope: change.Envelope{Truth: map[string]any{"freshness": map[string]any{"state": "stale"}}},
		},
		{
			name: "truncated answer exits 5", args: []string{"--target", "x"}, wantCall: true, wantCode: 5,
			envelope: change.Envelope{Data: map[string]any{"truncated": true}},
		},
		{
			name: "transport failure is an error exit", args: []string{"--target", "x"}, wantCall: true, wantCode: 1,
			fetchErr: errors.New("connection refused"),
		},
		{
			name: "api error envelope is an error exit", args: []string{"--target", "x"}, wantCall: true, wantCode: 1,
			envelope: change.Envelope{Error: &change.EnvelopeError{Code: "internal_error", Message: "boom"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var called bool
			stubChangeFetch(t,
				func(_ *APIClient, _ change.Options) (change.Envelope, error) {
					called = true
					return tc.envelope, tc.fetchErr
				},
				func(*APIClient, change.Options) (change.Envelope, error) {
					t.Fatal("plan fetch must not run for the impact command")
					return change.Envelope{}, nil
				})
			_, err := runChangeCommand(t, newChangeImpactCommand, tc.args...)
			if called != tc.wantCall {
				t.Errorf("fetch called = %v, want %v", called, tc.wantCall)
			}
			if tc.wantCode == 0 {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if got := exitCodeOf(t, err); got != tc.wantCode {
				t.Errorf("exit code = %d, want %d (err: %v)", got, tc.wantCode, err)
			}
		})
	}
}

func TestRunChangePlanPassesIntentAndClassifiesBlocked(t *testing.T) {
	var gotIntent string
	stubChangeFetch(t,
		func(*APIClient, change.Options) (change.Envelope, error) {
			t.Fatal("impact fetch must not run for the plan command")
			return change.Envelope{}, nil
		},
		func(_ *APIClient, opts change.Options) (change.Envelope, error) {
			gotIntent = opts.DeveloperIntent
			return change.Envelope{Data: map[string]any{"blocked": true}}, nil
		})
	_, err := runChangeCommand(t, newChangePlanCommand, "--target", "svc", "--intent", "  rename handler  ")
	if gotIntent != "rename handler" {
		t.Errorf("developer intent = %q, want it trimmed", gotIntent)
	}
	if got := exitCodeOf(t, err); got != 5 {
		t.Errorf("blocked plan exit code = %d, want 5", got)
	}
}

func TestRunChangePlanFailurePaths(t *testing.T) {
	stubChangeFetch(t, nil, func(*APIClient, change.Options) (change.Envelope, error) {
		return change.Envelope{}, errors.New("dial tcp: refused")
	})
	if _, err := runChangeCommand(t, newChangePlanCommand); exitCodeOf(t, err) != 2 {
		t.Errorf("plan without a selector: err = %v, want exit code 2", err)
	}
	if _, err := runChangeCommand(t, newChangePlanCommand, "--target", "x"); exitCodeOf(t, err) != 1 {
		t.Errorf("plan with a transport failure: err = %v, want exit code 1", err)
	}
	stubChangeFetch(t, nil, func(*APIClient, change.Options) (change.Envelope, error) {
		return change.Envelope{Error: &change.EnvelopeError{Code: "internal_error", Message: "boom"}}, nil
	})
	if _, err := runChangeCommand(t, newChangePlanCommand, "--target", "x"); exitCodeOf(t, err) != 1 {
		t.Errorf("plan with an api error: err = %v, want exit code 1", err)
	}
	stubChangeFetch(t, nil, func(*APIClient, change.Options) (change.Envelope, error) {
		return change.Envelope{Truth: map[string]any{"freshness": map[string]any{"state": "building"}}}, nil
	})
	if _, err := runChangeCommand(t, newChangePlanCommand, "--target", "x"); exitCodeOf(t, err) != 4 {
		t.Errorf("plan on a building index: err = %v, want exit code 4", err)
	}
}

func TestResolveChangeDiffKeepsExplicitFilesAndRejectsBadRepo(t *testing.T) {
	explicit := change.Options{Changes: change.ModifiedFiles([]string{"a.go"}), BaseRef: "main"}
	if err := resolveChangeDiff(&explicit); err != nil || len(explicit.Changes) != 1 {
		t.Errorf("explicit files must win over refs: changes=%v err=%v", explicit.Changes, err)
	}
	none := change.Options{}
	if err := resolveChangeDiff(&none); err != nil || len(none.Changes) != 0 {
		t.Errorf("no refs and no files must be a no-op: changes=%v err=%v", none.Changes, err)
	}
	bad := change.Options{BaseRef: "main", HeadRef: "HEAD", RepoPath: t.TempDir()}
	if err := resolveChangeDiff(&bad); err == nil {
		t.Error("a ref diff outside a git repository must fail")
	}
}

func TestResolveChangeDiffDerivesChangesFromGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo, "-c", "user.email=t@example.com", "-c", "user.name=t"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-q")
	if err := writeTestFile(t, repo+"/a.txt", "one\n"); err != nil {
		t.Fatal(err)
	}
	run("add", "a.txt")
	run("commit", "-q", "-m", "first")
	if err := writeTestFile(t, repo+"/b.txt", "two\n"); err != nil {
		t.Fatal(err)
	}
	run("add", "b.txt")
	run("commit", "-q", "-m", "second")

	opts := change.Options{BaseRef: "HEAD~1", HeadRef: "HEAD", RepoPath: repo}
	if err := resolveChangeDiff(&opts); err != nil {
		t.Fatalf("resolveChangeDiff: %v", err)
	}
	if len(opts.Changes) != 1 || opts.Changes[0].Path != "b.txt" || len(opts.ChangedPaths) != 1 {
		t.Errorf("derived changes = %+v paths = %v, want just b.txt", opts.Changes, opts.ChangedPaths)
	}
}
