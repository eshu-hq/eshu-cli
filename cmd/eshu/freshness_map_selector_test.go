// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/eshu-hq/eshu-cli/internal/cli/entitymap"
	"github.com/eshu-hq/eshu-cli/internal/cli/freshness"
)

// parseCommandFlags builds a command, parses args, and fails the test on a
// parse error.
func parseCommandFlags(t *testing.T, cmd *cobra.Command, args ...string) *cobra.Command {
	t.Helper()
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("parse flags %v: %v", args, err)
	}
	return cmd
}

func TestFreshnessGenerationsOptionsFromCommand(t *testing.T) {
	cmd := &cobra.Command{Use: "generations"}
	addFreshnessGenerationsFlags(cmd)
	parseCommandFlags(t, cmd,
		"--json", "--scope-id", "scope-1", "--repository", "repo-1", "--collector-kind", "git",
		"--source-system", "github", "--generation-id", "gen-1", "--status", "active", "--limit", "7")
	got, err := freshnessGenerationsOptionsFromCommand(cmd)
	if err != nil {
		t.Fatalf("options: %v", err)
	}
	want := freshness.GenerationsOptions{
		JSON: true, ScopeID: "scope-1", Repository: "repo-1", CollectorKind: "git",
		SourceSystem: "github", GenerationID: "gen-1", Status: "active", Limit: 7,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("options = %+v, want %+v", got, want)
	}
	if _, err := freshnessGenerationsOptionsFromCommand(&cobra.Command{Use: "bare"}); err == nil {
		t.Error("a command without the freshness flags must return the flag lookup error")
	}
}

func TestFreshnessChangedSinceOptionsFromCommand(t *testing.T) {
	cmd := newFreshnessChangedSinceCommand()
	parseCommandFlags(t, cmd,
		"--json", "--scope-id", "scope-1", "--repository", "repo-1",
		"--since-generation-id", "gen-0", "--since-observed-at", "2026-10-01T00:00:00Z", "--sample-limit", "9")
	got, err := freshnessChangedSinceOptionsFromCommand(cmd)
	if err != nil {
		t.Fatalf("options: %v", err)
	}
	want := freshness.ChangedSinceOptions{
		JSON: true, ScopeID: "scope-1", Repository: "repo-1",
		SinceGenerationID: "gen-0", SinceObservedAt: "2026-10-01T00:00:00Z", SampleLimit: 9,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("options = %+v, want %+v", got, want)
	}
	if _, err := freshnessChangedSinceOptionsFromCommand(&cobra.Command{Use: "bare"}); err == nil {
		t.Error("a command without the changed-since flags must return the flag lookup error")
	}
}

func TestFreshnessServiceChangedSinceOptionsFromCommand(t *testing.T) {
	cmd := newFreshnessServiceChangedSinceCommand()
	parseCommandFlags(t, cmd,
		"--json", "--service-id", "svc-1", "--scope-id", "scope-1", "--since-generation-id", "gen-0", "--sample-limit", "4")
	got, err := freshnessServiceChangedSinceOptionsFromCommand(cmd)
	if err != nil {
		t.Fatalf("options: %v", err)
	}
	want := freshness.ServiceChangedSinceOptions{
		JSON: true, ServiceID: "svc-1", ScopeID: "scope-1", SinceGenerationID: "gen-0", SampleLimit: 4,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("options = %+v, want %+v", got, want)
	}
	if _, err := freshnessServiceChangedSinceOptionsFromCommand(&cobra.Command{Use: "bare"}); err == nil {
		t.Error("a command without the service changed-since flags must return the flag lookup error")
	}
}

func TestFreshnessExitErrorConvertsFailuresOnly(t *testing.T) {
	if err := freshnessExitError(nil); err != nil {
		t.Errorf("nil error = %v, want nil", err)
	}
	plain := errors.New("disk on fire")
	if err := freshnessExitError(plain); !errors.Is(err, plain) {
		t.Errorf("a non-failure error must pass through untouched, got %v", err)
	}
	err := freshnessExitError(&freshness.Failure{Message: "stale index", Code: 4})
	var exit commandExitError
	if !errors.As(err, &exit) || exit.code != 4 || exit.message != "stale index" {
		t.Errorf("failure conversion = %#v, want exit code 4 with the message", err)
	}
}

func TestEntityMapOptionsFromCommandNormalizesInput(t *testing.T) {
	cmd := &cobra.Command{Use: "map"}
	addEntityMapFlags(cmd)
	parseCommandFlags(t, cmd,
		"--from", " terraform/aws_lb.main ", "--type", " terraform_resource ", "--repo", " r ",
		"--env", " prod ", "--relationship", " depends_on ", "--depth", "3", "--limit", "11", "--json")
	got, err := entityMapOptionsFromCommand(cmd)
	if err != nil {
		t.Fatalf("options: %v", err)
	}
	want := entitymap.Options{
		From: "terraform/aws_lb.main", FromType: "terraform_resource", Repo: "r", Environment: "prod",
		Relationship: "DEPENDS_ON", Depth: 3, Limit: 11, JSON: true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("options = %+v, want %+v", got, want)
	}
	if _, err := entityMapOptionsFromCommand(&cobra.Command{Use: "bare"}); err == nil {
		t.Error("a command without the map flags must return the flag lookup error")
	}
}

func TestRunMapFromOutcomes(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		envelope entitymap.Envelope
		fetchErr error
		wantCode int // 0 means success
		wantCall bool
	}{
		{name: "missing --from is a usage error", args: nil, wantCode: 2},
		{name: "clean map succeeds", args: []string{"--from", "svc"}, wantCall: true},
		{
			name: "ambiguous selector exits 3", args: []string{"--from", "svc"}, wantCall: true, wantCode: 3,
			envelope: entitymap.Envelope{Data: map[string]any{"status": "ambiguous"}},
		},
		{
			name: "no match exits 2", args: []string{"--from", "svc"}, wantCall: true, wantCode: 2,
			envelope: entitymap.Envelope{Data: map[string]any{"status": "no_match"}},
		},
		{
			name: "stale index exits 4", args: []string{"--from", "svc"}, wantCall: true, wantCode: 4,
			envelope: entitymap.Envelope{Truth: map[string]any{"freshness": map[string]any{"state": "stale"}}},
		},
		{
			name: "transport failure is an error exit", args: []string{"--from", "svc"}, wantCall: true, wantCode: 1,
			fetchErr: errors.New("connection refused"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orig := entityMapFetch
			t.Cleanup(func() { entityMapFetch = orig })
			var called bool
			entityMapFetch = func(entitymap.EnvelopePoster, entitymap.Options) (entitymap.Envelope, error) {
				called = true
				return tc.envelope, tc.fetchErr
			}
			cmd := &cobra.Command{Use: "map", Args: cobra.NoArgs, RunE: runMapFrom}
			addEntityMapFlags(cmd)
			addRemoteFlags(cmd)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
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

func TestEntityMapExitCodeDefaultsToOne(t *testing.T) {
	if got := entityMapExitCode(&entitymap.Failure{Kind: entitymap.FailureKind(99)}); got != 1 {
		t.Errorf("unknown failure kind exit code = %d, want 1", got)
	}
}

func TestReadRepositorySelectorFlag(t *testing.T) {
	cases := []struct {
		name      string
		flags     func(*cobra.Command)
		args      []string
		wantValue string
		wantExact bool
	}{
		{name: "no selector flags", flags: func(*cobra.Command) {}},
		{
			name:      "repo is fuzzy and trimmed",
			flags:     func(c *cobra.Command) { c.Flags().String("repo", "", "") },
			args:      []string{"--repo", "  my-repo "},
			wantValue: "my-repo",
		},
		{
			name:      "repo-id is exact",
			flags:     func(c *cobra.Command) { c.Flags().String("repo-id", "", "") },
			args:      []string{"--repo-id", " repository:r_1 "},
			wantValue: "repository:r_1", wantExact: true,
		},
		{
			name: "repo wins over repo-id",
			flags: func(c *cobra.Command) {
				c.Flags().String("repo", "", "")
				c.Flags().String("repo-id", "", "")
			},
			args:      []string{"--repo", "fuzzy", "--repo-id", "repository:r_1"},
			wantValue: "fuzzy",
		},
		{
			name: "blank repo falls through to repo-id",
			flags: func(c *cobra.Command) {
				c.Flags().String("repo", "", "")
				c.Flags().String("repo-id", "", "")
			},
			args:      []string{"--repo", "  ", "--repo-id", "repository:r_2"},
			wantValue: "repository:r_2", wantExact: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "x"}
			tc.flags(cmd)
			parseCommandFlags(t, cmd, tc.args...)
			value, exact, err := readRepositorySelectorFlag(cmd)
			if err != nil {
				t.Fatalf("readRepositorySelectorFlag: %v", err)
			}
			if value != tc.wantValue || exact != tc.wantExact {
				t.Errorf("got (%q, %v), want (%q, %v)", value, exact, tc.wantValue, tc.wantExact)
			}
		})
	}
	if value, exact, err := readRepositorySelectorFlag(nil); value != "" || exact || err != nil {
		t.Errorf("nil command = (%q, %v, %v), want empty", value, exact, err)
	}
}

func TestReadRepositorySelectorFlagReportsWrongFlagType(t *testing.T) {
	for _, name := range []string{"repo", "repo-id"} {
		cmd := &cobra.Command{Use: "x"}
		cmd.Flags().Bool(name, false, "")
		if _, _, err := readRepositorySelectorFlag(cmd); err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("a non-string --%s must report a read error naming it, got %v", name, err)
		}
	}
}

func TestResolveRepositorySelectorFromFlags(t *testing.T) {
	exactCmd := &cobra.Command{Use: "x"}
	exactCmd.Flags().String("repo-id", "", "")
	parseCommandFlags(t, exactCmd, "--repo-id", "repository:r_1")
	if got, err := resolveRepositorySelectorFromFlags(exactCmd, nil); got != "repository:r_1" || err != nil {
		t.Errorf("exact id = (%q, %v), want it returned without a client", got, err)
	}

	emptyCmd := &cobra.Command{Use: "x"}
	emptyCmd.Flags().String("repo", "", "")
	if got, err := resolveRepositorySelectorFromFlags(emptyCmd, nil); got != "" || err != nil {
		t.Errorf("no selector = (%q, %v), want empty and nil", got, err)
	}

	fuzzyCmd := &cobra.Command{Use: "x"}
	fuzzyCmd.Flags().String("repo", "", "")
	parseCommandFlags(t, fuzzyCmd, "--repo", "my-repo")
	if _, err := resolveRepositorySelectorFromFlags(fuzzyCmd, nil); err == nil || !strings.Contains(err.Error(), "missing API client") {
		t.Errorf("fuzzy selector with no client = %v, want the missing API client error", err)
	}

	badCmd := &cobra.Command{Use: "x"}
	badCmd.Flags().Bool("repo", false, "")
	if _, err := resolveRepositorySelectorFromFlags(badCmd, nil); err == nil {
		t.Error("a flag read error must be returned")
	}
}
