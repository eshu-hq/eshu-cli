// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package opdigest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validOptions(t *testing.T) Options {
	t.Helper()
	opts, err := OptionsFromFlags("repo:owner/name", "local_authoritative", 8)
	if err != nil {
		t.Fatalf("OptionsFromFlags = %v", err)
	}
	return opts
}

func TestOptionsFromFlagsTable(t *testing.T) {
	opts, err := OptionsFromFlags(" service:web ", " local_authoritative ", 5)
	if err != nil {
		t.Fatalf("OptionsFromFlags = %v", err)
	}
	if opts.Scope.Type != "service" || opts.Scope.Label != "web" || opts.Scope.ID != "service:web" {
		t.Fatalf("OptionsFromFlags scope = %+v", opts.Scope)
	}
	if opts.Profile != "local_authoritative" || opts.QuestionLimit != 5 {
		t.Fatalf("OptionsFromFlags = %+v", opts)
	}
	for prefix, typ := range map[string]string{
		"repo": "repository", "repository": "repository", "workload": "workload",
		"environment": "environment", "project": "project",
	} {
		opts, err := OptionsFromFlags(prefix+":x", "p", 0)
		if err != nil {
			t.Fatalf("OptionsFromFlags(%q) = %v", prefix, err)
		}
		if opts.Scope.Type != typ {
			t.Fatalf("OptionsFromFlags(%q).Type = %q, want %q", prefix, opts.Scope.Type, typ)
		}
	}
	opts, err = OptionsFromFlags("owner/name", "p", 0)
	if err != nil {
		t.Fatalf("OptionsFromFlags(bare) = %v", err)
	}
	if opts.Scope.Type != "repository" || opts.Scope.ID != "repo:owner/name" {
		t.Fatalf("OptionsFromFlags(bare) scope = %+v", opts.Scope)
	}
}

func TestOptionsFromFlagsRejects(t *testing.T) {
	for _, tc := range []struct {
		name           string
		scope, profile string
		limit          int
	}{
		{"empty scope", "", "p", 1},
		{"blank scope", "   ", "p", 1},
		{"url scope", "https://x/y", "p", 1},
		{"abs scope", "/etc/passwd", "p", 1},
		{"home scope", "~operator", "p", 1},
		{"backslash scope", `a\b`, "p", 1},
		{"unknown prefix", "cluster:web", "p", 1},
		{"empty label", "repo:  ", "p", 1},
		{"bad label chars", "repo:web app!", "p", 1},
		{"long label", "repo:" + strings.Repeat("a", 161), "p", 1},
		{"dotdot label", "repo:a/../b", "p", 1},
		{"colon label", "repo:a:b", "p", 1},
		{"empty profile", "repo:x", "", 1},
		{"blank profile", "repo:x", "  ", 1},
		{"bad profile", "repo:x", "not safe!", 1},
		{"negative limit", "repo:x", "p", -1},
		{"over limit", "repo:x", "p", 26},
	} {
		if _, err := OptionsFromFlags(tc.scope, tc.profile, tc.limit); err == nil {
			t.Fatalf("OptionsFromFlags(%q, %q, %d) = nil error, want rejection", tc.scope, tc.profile, tc.limit)
		}
	}
}

func TestIsShareSafeTokenTable(t *testing.T) {
	for _, ok := range []string{"a", "aB3", "x.y_z-w/1"} {
		if !isShareSafeToken(ok) {
			t.Fatalf("isShareSafeToken(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", "a b", "a:b", "a~b", "a\\b", "a+b"} {
		if isShareSafeToken(bad) {
			t.Fatalf("isShareSafeToken(%q) = true, want false", bad)
		}
	}
}

func TestBuildDigestShape(t *testing.T) {
	digest := BuildDigest(validOptions(t))
	if digest.Schema != Schema {
		t.Fatalf("BuildDigest schema = %q", digest.Schema)
	}
	if len(digest.Sections) != len(sectionTemplates) {
		t.Fatalf("BuildDigest sections = %d, want %d", len(digest.Sections), len(sectionTemplates))
	}
	for _, section := range digest.Sections {
		if section.Status != "unsupported" || len(section.Limitations) != 1 || len(section.SourceRefs) == 0 {
			t.Fatalf("BuildDigest section %+v missing contract shape", section)
		}
	}
	if len(digest.SuggestedQuestions) != len(questionTemplates) {
		t.Fatalf("BuildDigest questions = %d, want %d", len(digest.SuggestedQuestions), len(questionTemplates))
	}
	if digest.Truth.Reason != "bounded_read_surface_not_connected" {
		t.Fatalf("BuildDigest truth reason = %q", digest.Truth.Reason)
	}
	if len(digest.Limitations) != 1 {
		t.Fatalf("BuildDigest limitations = %d, want only the read-surface one", len(digest.Limitations))
	}
}

func TestBuildDigestTruncatesQuestions(t *testing.T) {
	opts := validOptions(t)
	opts.QuestionLimit = 2
	digest := BuildDigest(opts)
	if len(digest.SuggestedQuestions) != 2 {
		t.Fatalf("BuildDigest questions = %d, want 2", len(digest.SuggestedQuestions))
	}
	if len(digest.Limitations) != 2 || digest.Limitations[1].Reason != "suggested_questions_truncated" {
		t.Fatalf("BuildDigest limitations = %+v, want truncation record", digest.Limitations)
	}
	opts.QuestionLimit = 0
	digest = BuildDigest(opts)
	if len(digest.SuggestedQuestions) != 0 || len(digest.Limitations) != 2 {
		t.Fatalf("BuildDigest limit 0 = %d questions %+v", len(digest.SuggestedQuestions), digest.Limitations)
	}
}

func TestQuestionWhyBranches(t *testing.T) {
	if got := questionWhy("s1", ""); !strings.Contains(got, "needs a bounded read surface") {
		t.Fatalf("questionWhy(empty) = %q", got)
	}
	if got := questionWhy("s1", "sig-9"); !strings.Contains(got, "sig-9") {
		t.Fatalf("questionWhy(signal) = %q", got)
	}
}

func TestTargetKindTable(t *testing.T) {
	for target, want := range map[string]string{
		"mcp:tool": "mcp_tool", "route:/x": "http_route",
		"query-playbook:q": "query_playbook", "other": "reference",
	} {
		if got := targetKind(target); got != want {
			t.Fatalf("targetKind(%q) = %q, want %q", target, got, want)
		}
	}
}

func TestDedupeSourceRefs(t *testing.T) {
	got := dedupeSourceRefs([]SourceRef{
		{ID: "b"}, {ID: ""}, {ID: "a"}, {ID: "b"}, {ID: "  "},
	})
	if len(got) != 2 || got[0].ID != "b" || got[1].ID != "a" {
		t.Fatalf("dedupeSourceRefs = %+v", got)
	}
}

func TestQuestionSourceRefsSkipsBlank(t *testing.T) {
	got := questionSourceRefs([]Question{{Target: ""}, {Target: "mcp:t"}})
	if len(got) != 1 || got[0].Kind != "mcp_tool" || got[0].ID != "mcp:t" {
		t.Fatalf("questionSourceRefs = %+v", got)
	}
}

func validArtifact(t *testing.T) Artifact {
	t.Helper()
	artifact, err := BuildArtifact(BuildDigest(validOptions(t)))
	if err != nil {
		t.Fatalf("BuildArtifact = %v", err)
	}
	return artifact
}

func TestBuildArtifactMarkers(t *testing.T) {
	artifact := validArtifact(t)
	if artifact.Schema != artifactSchema {
		t.Fatalf("BuildArtifact schema = %q", artifact.Schema)
	}
	if artifact.Artifact.WriterKind != artifactWriterCLI || artifact.Artifact.Format != artifactFormat {
		t.Fatalf("BuildArtifact metadata = %+v", artifact.Artifact)
	}
	if !strings.HasPrefix(artifact.Artifact.ID, "operator-digest-artifact:") {
		t.Fatalf("BuildArtifact ID = %q", artifact.Artifact.ID)
	}
	if artifact.Redaction.Profile != redactionProfile || len(artifact.Redaction.AppliedRules) == 0 {
		t.Fatalf("BuildArtifact redaction = %+v", artifact.Redaction)
	}
	for _, check := range artifact.Validation.Checks {
		if !check.OK {
			t.Fatalf("BuildArtifact check %+v not ok", check)
		}
	}
	if len(artifact.SourceRefs) == 0 {
		t.Fatal("BuildArtifact has no source refs")
	}
}

func TestArtifactIDDeterministic(t *testing.T) {
	digest := BuildDigest(validOptions(t))
	first, err := BuildArtifact(digest)
	if err != nil {
		t.Fatalf("BuildArtifact = %v", err)
	}
	second, err := BuildArtifact(digest)
	if err != nil {
		t.Fatalf("BuildArtifact = %v", err)
	}
	if first.Artifact.ID != second.Artifact.ID {
		t.Fatal("BuildArtifact ID not deterministic")
	}
	other := digest
	other.Scope.ID = "service:other"
	third, err := BuildArtifact(other)
	if err != nil {
		t.Fatalf("BuildArtifact = %v", err)
	}
	if third.Artifact.ID == first.Artifact.ID {
		t.Fatal("BuildArtifact ID does not cover the scope")
	}
}

func TestBuildArtifactRejects(t *testing.T) {
	cases := map[string]func(*Digest){
		"empty scope":        func(d *Digest) { d.Scope.ID = "  " },
		"empty profile":      func(d *Digest) { d.Profile = "" },
		"empty truth class":  func(d *Digest) { d.Truth.TruthClass = "" },
		"empty freshness":    func(d *Digest) { d.Truth.Freshness = "" },
		"no sections":        func(d *Digest) { d.Sections = nil },
		"no limitations":     func(d *Digest) { d.Limitations = nil },
		"no source refs":     func(d *Digest) { d.SourceRefs = nil; d.SuggestedQuestions = nil },
		"orphan section":     func(d *Digest) { d.Sections[0].SourceRefs = nil; d.Sections[0].Limitations = nil },
		"unknown section":    func(d *Digest) { d.Sections[0].Limitations = nil; d.Sections[0].SourceRefs = []string{"nope"} },
		"question no why":    func(d *Digest) { d.SuggestedQuestions[0].Why = "" },
		"question no signal": func(d *Digest) { d.SuggestedQuestions[0].SourceSignal = ""; d.SuggestedQuestions[0].EvidenceRefs = nil },
		// A blank target is the only one questionSourceRefs does not
		// self-register, so it is the only unknown-target trigger.
		"question no target": func(d *Digest) { d.SuggestedQuestions[0].Target = "" },
	}
	for name, mutate := range cases {
		digest := BuildDigest(validOptions(t))
		mutate(&digest)
		if _, err := BuildArtifact(digest); err == nil {
			t.Fatalf("BuildArtifact(%s) = nil error, want rejection", name)
		}
	}
}

func TestValidateArtifactDefensiveBranches(t *testing.T) {
	artifact := validArtifact(t)
	cases := map[string]func(*Artifact){
		"schema":            func(a *Artifact) { a.Schema = "wrong" },
		"digest schema":     func(a *Artifact) { a.Digest.Schema = "wrong" },
		"blank source ref":  func(a *Artifact) { a.SourceRefs = append(a.SourceRefs, SourceRef{}) },
		"redaction profile": func(a *Artifact) { a.Redaction.Profile = "wrong" },
		"validation status": func(a *Artifact) { a.Validation.Status = "failed" },
		"artifact status":   func(a *Artifact) { a.Artifact.ValidationStatus = "failed" },
		"empty artifact id": func(a *Artifact) { a.Artifact.ID = "" },
	}
	for name, mutate := range cases {
		mutated := artifact
		mutate(&mutated)
		if err := validateArtifact(mutated); err == nil {
			t.Fatalf("validateArtifact(%s) = nil error, want rejection", name)
		}
	}
	if err := validateArtifact(artifact); err != nil {
		t.Fatalf("validateArtifact(valid) = %v", err)
	}
}

func TestWriteArtifactRoundTrip(t *testing.T) {
	digest := BuildDigest(validOptions(t))
	path := filepath.Join(t.TempDir(), "artifact.json")
	if err := WriteArtifact(path, digest); err != nil {
		t.Fatalf("WriteArtifact = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile = %v", err)
	}
	if !strings.HasSuffix(string(data), "}\n") || !strings.Contains(string(data), "\n  \"schema\"") {
		t.Fatalf("WriteArtifact wrote non-indented JSON: %.80q", data)
	}
	var artifact Artifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("Unmarshal = %v", err)
	}
	if artifact.Schema != artifactSchema || artifact.Artifact.ID == "" {
		t.Fatalf("WriteArtifact round trip = %+v", artifact.Artifact)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("WriteArtifact mode = %v, %v", info.Mode(), err)
	}
}

func TestWriteArtifactErrors(t *testing.T) {
	if err := WriteArtifact(filepath.Join(t.TempDir(), "missing", "a.json"), BuildDigest(validOptions(t))); err == nil {
		t.Fatal("WriteArtifact(missing dir) = nil error, want PathError")
	}
	if err := WriteArtifact(filepath.Join(t.TempDir(), "a.json"), Digest{}); err == nil {
		t.Fatal("WriteArtifact(invalid) = nil error, want validation error")
	}
	if err := writeArtifactFile(filepath.Join(t.TempDir(), "missing", "a.json"), []byte("{}")); err == nil {
		t.Fatal("writeArtifactFile(missing dir) = nil error, want PathError")
	}
	if _, err := os.Stat("/dev/full"); err == nil {
		if err := writeArtifactFile("/dev/full", []byte("x")); err == nil {
			t.Fatal("writeArtifactFile(/dev/full) = nil error, want write error")
		}
	}
}

func TestRenderText(t *testing.T) {
	var buf strings.Builder
	RenderText(&buf, BuildDigest(validOptions(t)))
	out := buf.String()
	for _, want := range []string{
		"Operator digest\n",
		"  scope    : owner/name (repository)\n",
		"  profile  : local_authoritative\n",
		"  truth    : unsupported freshness=unavailable reason=bounded_read_surface_not_connected\n",
		"sections:\n",
		"  - hub_services: unsupported\n",
		"suggested questions:\n",
		"    target: mcp:get_service_story\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("RenderText missing %q in:\n%s", want, out)
		}
	}
	var bare strings.Builder
	RenderText(&bare, Digest{})
	if strings.Contains(bare.String(), "suggested questions:") {
		t.Fatalf("RenderText(empty) rendered questions:\n%s", bare.String())
	}
}
