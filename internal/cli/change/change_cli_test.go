// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package change

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// statusErr carries an HTTP status through apierr.StatusCode, exercising the
// status switch in ErrorCodeFromTransport.
type statusErr struct {
	status int
	msg    string
}

func (e statusErr) Error() string       { return e.msg }
func (e statusErr) HTTPStatusCode() int { return e.status }

func TestNormalizeStatus(t *testing.T) {
	t.Parallel()

	cases := []struct{ in, want string }{
		{"A", "added"},
		{"a", "added"},
		{"D", "deleted"},
		{"d", "deleted"},
		{"R100", "renamed"},
		{"r99", "renamed"},
		{"R", "renamed"},
		{"C85", "copied"},
		{"c", "copied"},
		{"M", "modified"},
		{"T", "modified"},
		{"U", "modified"},
		{"X", "modified"},
		{"Q", "modified"},
		{"", "modified"},
		{" R100 ", "renamed"},
	}
	for _, tc := range cases {
		if got := NormalizeStatus(tc.in); got != tc.want {
			t.Errorf("NormalizeStatus(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseNameStatusDiff(t *testing.T) {
	t.Parallel()

	changes := ParseNameStatusDiff("M\tgo/a.go\nR100\tgo/old.go\tgo/new.go\nC85\tgo/src.go\tgo/dst.go\nA\tgo/b.go\nD\tgo/c.go\n\nX\n")
	byPath := map[string]FileChange{}
	for _, c := range changes {
		byPath[c.Path] = c
	}
	if len(changes) != 5 {
		t.Fatalf("ParseNameStatusDiff() produced %d rows, want 5 (blank and short lines skipped)", len(changes))
	}
	if got := byPath["go/new.go"]; got.Status != "renamed" || got.OldPath != "go/old.go" {
		t.Errorf("rename row = %+v, want status renamed old go/old.go", got)
	}
	if got := byPath["go/dst.go"]; got.Status != "copied" || got.OldPath != "go/src.go" {
		t.Errorf("copy row = %+v, want status copied old go/src.go", got)
	}
	if got := byPath["go/a.go"]; got.Status != "modified" || got.OldPath != "" {
		t.Errorf("modify row = %+v, want status modified no old path", got)
	}
	// A rename line with only two fields keeps the single path; the
	// three-field assignment needs the third field.
	short := ParseNameStatusDiff("R100\tonly-one\n")
	if len(short) != 1 || short[0].Path != "only-one" || short[0].OldPath != "" {
		t.Errorf("short rename row = %+v, want path only-one with no old path", short)
	}
}

// initGitRepo builds a one-commit repo and leaves a.txt modified in the
// working tree, so the no-ref diff has something to report.
func initGitRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(cmd.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+t.TempDir())
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")
	if err := exec.Command("sh", "-c", "echo one > "+filepath.Join(dir, "a.txt")).Run(); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	run("add", "a.txt")
	run("commit", "-qm", "seed")
	if err := exec.Command("sh", "-c", "echo two >> "+filepath.Join(dir, "a.txt")).Run(); err != nil {
		t.Fatalf("modify a.txt: %v", err)
	}
	return dir
}

func TestGitDiffNameStatus(t *testing.T) {
	t.Parallel()

	dir := initGitRepo(t)
	for _, tc := range []struct {
		name    string
		baseRef string
		headRef string
	}{
		{"neither ref diffs worktree against index", "", ""},
		{"both refs compare them", "HEAD", "HEAD"},
		{"base only compares worktree against it", "HEAD", ""},
		{"head only is read as the single ref", "", "HEAD"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			changes, err := GitDiffNameStatus(dir, tc.baseRef, tc.headRef)
			if err != nil {
				t.Fatalf("GitDiffNameStatus(%q, %q) error = %v", tc.baseRef, tc.headRef, err)
			}
			if tc.baseRef == "HEAD" && tc.headRef == "HEAD" {
				if len(changes) != 0 {
					t.Errorf("HEAD..HEAD = %+v, want no rows", changes)
				}
				return
			}
			if len(changes) != 1 || changes[0].Path != "a.txt" || changes[0].Status != "modified" {
				t.Errorf("worktree diff = %+v, want one modified a.txt", changes)
			}
		})
	}
}

func TestGitDiffNameStatusError(t *testing.T) {
	t.Parallel()

	if _, err := GitDiffNameStatus(t.TempDir(), "", ""); err == nil {
		t.Error("GitDiffNameStatus(non-repo) = nil, want derive error")
	} else if !strings.Contains(err.Error(), "derive git diff") {
		t.Errorf("GitDiffNameStatus(non-repo) error = %q, want derive prefix", err)
	}
}

func TestModifiedFilesAndPaths(t *testing.T) {
	t.Parallel()

	got := ModifiedFiles([]string{"  go/a.go ", "", "   ", "go/b.go"})
	if len(got) != 2 || got[0] != (FileChange{Path: "go/a.go", Status: "modified"}) {
		t.Errorf("ModifiedFiles() = %+v, want two trimmed modified rows", got)
	}
	if paths := ChangedPaths([]FileChange{{Path: "new.go", OldPath: "old.go"}, {Path: "  "}, {Path: "b.go"}}); len(paths) != 2 {
		t.Errorf("ChangedPaths() = %v, want the two non-blank new paths", paths)
	}
	if cleaned := CleanValues(nil); cleaned == nil || len(cleaned) != 0 {
		t.Errorf("CleanValues(nil) = %#v, want non-nil empty", cleaned)
	}
}

func TestValueReaders(t *testing.T) {
	t.Parallel()

	if got := mapValue(nil, "k"); got != nil {
		t.Errorf("mapValue(nil) = %v, want nil", got)
	}
	if got := mapValue(map[string]any{"k": "v"}, "k"); got != nil {
		t.Errorf("mapValue(string) = %v, want nil", got)
	}
	if got := mapValue(map[string]any{"k": map[string]any{"a": 1}}, "k"); len(got) != 1 {
		t.Errorf("mapValue(object) = %v, want the object", got)
	}
	if got := sliceValue(map[string]any{"rows": []map[string]any{{"a": "b"}}}, "rows"); len(got) != 1 {
		t.Errorf("sliceValue([]map) len = %d, want 1", len(got))
	}
	if got := sliceValue(map[string]any{"rows": []any{"x"}}, "rows"); len(got) != 1 {
		t.Errorf("sliceValue([]any) len = %d, want 1", len(got))
	}
	if got := sliceValue(map[string]any{"rows": "nope"}, "rows"); got != nil {
		t.Errorf("sliceValue(string) = %v, want nil", got)
	}
	if got := sliceValue(nil, "rows"); got != nil {
		t.Errorf("sliceValue(nil) = %v, want nil", got)
	}
	if got := stringValue(map[string]any{"k": "  padded  "}, "k"); got != "padded" {
		t.Errorf("stringValue() = %q, want trimmed", got)
	}
	if got := stringValue(map[string]any{"k": 7}, "k"); got != "" {
		t.Errorf("stringValue(int) = %q, want empty", got)
	}
	if got := stringValue(nil, "k"); got != "" {
		t.Errorf("stringValue(nil) = %q, want empty", got)
	}
	if got := intValue(map[string]any{"n": 7}, "n"); got != 7 {
		t.Errorf("intValue(int) = %d, want 7", got)
	}
	if got := intValue(map[string]any{"n": float64(2.9)}, "n"); got != 2 {
		t.Errorf("intValue(float) = %d, want 2", got)
	}
	if got := intValue(map[string]any{"n": "seven"}, "n"); got != 0 {
		t.Errorf("intValue(string) = %d, want 0", got)
	}
	if got := intValue(nil, "n"); got != 0 {
		t.Errorf("intValue(nil) = %d, want 0", got)
	}
	if got := boolValue(map[string]any{"b": true}, "b"); !got {
		t.Error("boolValue(true) = false, want true")
	}
	if got := boolValue(map[string]any{"b": "yes"}, "b"); got {
		t.Error("boolValue(string) = true, want false")
	}
	if got := boolValue(nil, "b"); got {
		t.Error("boolValue(nil) = true, want false")
	}
	if got := freshnessState(Envelope{Truth: map[string]any{"freshness": map[string]any{"state": "stale"}}}); got != "stale" {
		t.Errorf("freshnessState() = %q, want stale", got)
	}
	if got := freshnessState(Envelope{}); got != "" {
		t.Errorf("freshnessState(empty) = %q, want empty", got)
	}
}

func TestKinds(t *testing.T) {
	t.Parallel()

	want := []FailureKind{KindInvalidArgument, KindEnvelope, KindFreshness, KindIncomplete}
	got := Kinds()
	if len(got) != len(want) {
		t.Fatalf("Kinds() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Kinds()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	var f Failure = Failure{Kind: KindEnvelope, Code: "stale", Message: "scope is stale"}
	if f.Error() != "scope is stale" {
		t.Errorf("Failure.Error() = %q, want the message", f.Error())
	}
}

func TestErrorCodeFromTransport(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want string
	}{
		{"refused", errors.New("dial tcp: connection refused"), "backend_unavailable"},
		{"failed", errors.New("request failed after retries"), "backend_unavailable"},
		{"bad-request", statusErr{status: 400, msg: "bad"}, "invalid_argument"},
		{"not-found", statusErr{status: 404, msg: "missing"}, "not_found"},
		{"not-implemented", statusErr{status: 501, msg: "nope"}, "unsupported_capability"},
		{"unavailable", statusErr{status: 503, msg: "down"}, "backend_unavailable"},
		{"teapot", statusErr{status: 418, msg: "tea"}, "api_error"},
		{"plain", errors.New("boom"), "api_error"},
		{"nil", nil, "api_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := ErrorCodeFromTransport(tc.err); got != tc.want {
				t.Errorf("ErrorCodeFromTransport(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

// TestErrorCodeFromTransportPrefersMessageOverStatus pins the ordering: the
// message checks run before the status switch, so an error carrying both an
// HTTP status and a refused/failed message classifies as backend_unavailable.
func TestErrorCodeFromTransportPrefersMessageOverStatus(t *testing.T) {
	t.Parallel()

	err := statusErr{status: 400, msg: "dial tcp: connection refused"}
	if got := ErrorCodeFromTransport(err); got != "backend_unavailable" {
		t.Errorf("ErrorCodeFromTransport(precedence) = %q, want backend_unavailable", got)
	}
}

func TestEnvelopeFromTransportError(t *testing.T) {
	t.Parallel()

	if got := EnvelopeFromTransportError(nil); got.Error != nil || got.Data != nil {
		t.Errorf("EnvelopeFromTransportError(nil) = %+v, want zero envelope", got)
	}
	got := EnvelopeFromTransportError(errors.New("dial tcp: connection refused"))
	if got.Error == nil || got.Error.Code != "backend_unavailable" {
		t.Fatalf("EnvelopeFromTransportError(refused) = %+v, want backend_unavailable", got)
	}
	if !strings.Contains(got.Error.Message, "connection refused") {
		t.Errorf("EnvelopeFromTransportError(refused) message = %q, want the Go error text", got.Error.Message)
	}
}

func TestEnvelopeFailure(t *testing.T) {
	t.Parallel()

	if got := EnvelopeFailure(nil); got != nil {
		t.Errorf("EnvelopeFailure(nil) = %v, want nil", got)
	}
	got := EnvelopeFailure(&EnvelopeError{Code: "stale", Message: "scope is stale"})
	failure, ok := got.(Failure)
	if !ok || failure.Kind != KindEnvelope || failure.Code != "stale" || failure.Message != "scope is stale" {
		t.Errorf("EnvelopeFailure(message) = %#v, want envelope failure with the message", got)
	}
	got = EnvelopeFailure(&EnvelopeError{Code: "ambiguous", Message: "   "})
	if failure, _ := got.(Failure); failure.Message != "ambiguous" {
		t.Errorf("EnvelopeFailure(blank message) = %#v, want code fallback", got)
	}
	got = EnvelopeFailure(&EnvelopeError{})
	if failure, _ := got.(Failure); failure.Message != "pre-change impact failed" {
		t.Errorf("EnvelopeFailure(empty) = %#v, want fixed fallback", got)
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	valid := Options{Target: "svc-a", MaxDepth: 4, Limit: 25}
	cases := []struct {
		name    string
		opts    Options
		wantErr string
	}{
		{"changes need repo id", Options{Changes: ModifiedFiles([]string{"a.go"}), MaxDepth: 4, Limit: 25}, "--repo-id is required when changed files are provided"},
		{"something required", Options{MaxDepth: 4, Limit: 25}, "--file, --base/--head, --target, --service-name, or --topic is required"},
		{"limit zero", Options{Target: "s", MaxDepth: 4}, "--limit must be between 1 and 100"},
		{"limit over", Options{Target: "s", MaxDepth: 4, Limit: 101}, "--limit must be between 1 and 100"},
		{"depth zero", Options{Target: "s", Limit: 1}, "--max-depth must be between 1 and 8"},
		{"depth over", Options{Target: "s", MaxDepth: 9, Limit: 1}, "--max-depth must be between 1 and 8"},
		{"negative offset", Options{Target: "s", MaxDepth: 1, Limit: 1, Offset: -1}, "--offset must be greater than or equal to 0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := Validate(tc.opts)
			failure, ok := err.(Failure)
			if !ok || failure.Kind != KindInvalidArgument {
				t.Fatalf("Validate() = %#v, want invalid-argument failure", err)
			}
			if failure.Message != tc.wantErr {
				t.Errorf("Validate() message = %q, want %q", failure.Message, tc.wantErr)
			}
		})
	}
	if err := Validate(valid); err != nil {
		t.Errorf("Validate(valid) = %v, want nil", err)
	}
	if err := Validate(Options{Changes: ModifiedFiles([]string{"a.go"}), RepoID: "r", MaxDepth: 1, Limit: 1}); err != nil {
		t.Errorf("Validate(changes+repo) = %v, want nil", err)
	}
}

func TestClassifyImpact(t *testing.T) {
	t.Parallel()

	stale := Envelope{Truth: map[string]any{"freshness": map[string]any{"state": "stale"}}, Data: map[string]any{"truncated": true}}
	if err := ClassifyImpact(stale); err.(Failure).Kind != KindFreshness {
		t.Errorf("ClassifyImpact(stale+truncated) = %v, want freshness first", err)
	}
	building := Envelope{Truth: map[string]any{"freshness": map[string]any{"state": "building"}}}
	if err := ClassifyImpact(building); err.(Failure).Kind != KindFreshness {
		t.Errorf("ClassifyImpact(building) = %v, want freshness", err)
	}
	truncated := Envelope{Data: map[string]any{"truncated": true}}
	if err := ClassifyImpact(truncated); err.(Failure).Kind != KindIncomplete {
		t.Errorf("ClassifyImpact(truncated) = %v, want incomplete", err)
	}
	partial := Envelope{Data: map[string]any{"answer_packet": map[string]any{"partial": true}}}
	if err := ClassifyImpact(partial); err.(Failure).Kind != KindIncomplete {
		t.Errorf("ClassifyImpact(partial) = %v, want incomplete", err)
	}
	if err := ClassifyImpact(impactEnvelope()); err != nil {
		t.Errorf("ClassifyImpact(fresh) = %v, want nil", err)
	}
}

func TestClassifyPlan(t *testing.T) {
	t.Parallel()

	stale := Envelope{Truth: map[string]any{"freshness": map[string]any{"state": "stale"}}, Data: map[string]any{"blocked": true}}
	if err := ClassifyPlan(stale); err.(Failure).Kind != KindFreshness {
		t.Errorf("ClassifyPlan(stale+blocked) = %v, want freshness first", err)
	}
	for _, tc := range []struct {
		name string
		data map[string]any
	}{
		{"blocked", map[string]any{"blocked": true}},
		{"truncated", map[string]any{"truncated": true}},
		{"partial", map[string]any{"answer_packet": map[string]any{"partial": true}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := ClassifyPlan(Envelope{Data: tc.data}); err.(Failure).Kind != KindIncomplete {
				t.Errorf("ClassifyPlan(%s) = %v, want incomplete", tc.name, err)
			}
		})
	}
	if err := ClassifyPlan(Envelope{Data: map[string]any{}}); err != nil {
		t.Errorf("ClassifyPlan(clean) = %v, want nil", err)
	}
}
