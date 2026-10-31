// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package change

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

// failWriter is an io.Writer that always fails, exercising the write-error
// arms the bytes.Buffer-based tests can never reach.
type failWriter struct{}

func (failWriter) Write(p []byte) (int, error) { return 0, errors.New("no space") }

// dripWriter succeeds for the first failAfter writes, then fails. Stepping
// failAfter from 0 upward makes each successive write in a render sequence
// the failing one, so every mid-sequence write-error arm is exercised.
type dripWriter struct {
	failAfter int
	writes    int
}

func (w *dripWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes > w.failAfter {
		return 0, errors.New("no space")
	}
	return len(p), nil
}

// countWrites measures how many writes a render performs on its fixture.
func countWrites(render func(io.Writer, Envelope) error, env Envelope) int {
	w := &dripWriter{failAfter: 1 << 30}
	_ = render(w, env)
	return w.writes
}

func TestImpactSummaryContent(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	if err := renderImpactSummary(&out, impactEnvelope()); err != nil {
		t.Fatalf("renderImpactSummary() error = %v", err)
	}
	for _, want := range []string{
		"Truth freshness: fresh",
		"Pre-change impact: 2 changed files",
		"symbols=3 direct=1 transitive=2 missing_evidence=0",
		"next=get_entity_context reason=expand the direct callers",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("renderImpactSummary() missing %q in:\n%s", want, out.String())
		}
	}
	// No freshness claim, no freshness line.
	out.Reset()
	bare := Envelope{Data: map[string]any{}}
	if err := renderImpactSummary(&out, bare); err != nil {
		t.Fatalf("renderImpactSummary(bare) error = %v", err)
	}
	if strings.Contains(out.String(), "Truth freshness") {
		t.Errorf("renderImpactSummary(bare) wrote a freshness line for no claim: %q", out.String())
	}
	if !strings.Contains(out.String(), "Pre-change impact: 0 changed files") {
		t.Errorf("renderImpactSummary(bare) = %q, want zeroed counts", out.String())
	}
}

func TestPlanSummaryContent(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	if err := renderPlanSummary(&out, planEnvelope()); err != nil {
		t.Fatalf("renderPlanSummary() error = %v", err)
	}
	for _, want := range []string{
		"Developer change plan: 2 actions for 1 changed files",
		"action=edit risk=low title=update handler",
		"next=api target=svc-a",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("renderPlanSummary() missing %q in:\n%s", want, out.String())
		}
	}
}

// TestFirstWriteFailures pins every render's first-write failure: a writer
// that fails immediately must produce an error, never a silent empty render.
func TestFirstWriteFailures(t *testing.T) {
	t.Parallel()

	w := failWriter{}
	if err := renderError(w, impactEnvelopeWithError()); err == nil {
		t.Error("renderError(failWriter) = nil, want write error")
	}
	if err := renderImpactSummary(w, impactEnvelope()); err == nil {
		t.Error("renderImpactSummary(failWriter) = nil, want write error")
	}
	if err := renderPlanSummary(w, planEnvelope()); err == nil {
		t.Error("renderPlanSummary(failWriter) = nil, want write error")
	}
	if err := writeJSON(w, impactEnvelope()); err == nil {
		t.Error("writeJSON(failWriter) = nil, want write error")
	}
	sentinel := errors.New("cmd failed")
	if err := FinishImpact(w, Options{JSON: true}, impactEnvelope(), sentinel); err == nil {
		t.Error("FinishImpact(json, failWriter) = nil, want write error")
	}
	if err := FinishImpact(w, Options{}, impactEnvelopeWithError(), sentinel); err == nil {
		t.Error("FinishImpact(error, failWriter) = nil, want write error")
	}
	if err := FinishImpact(w, Options{}, impactEnvelope(), sentinel); err == nil {
		t.Error("FinishImpact(summary, failWriter) = nil, want write error")
	}
	if err := FinishImpact(w, Options{}, impactEnvelope(), nil); err == nil {
		t.Error("FinishImpact(ok, failWriter) = nil, want write error")
	}
	if err := FinishPlan(w, Options{}, planEnvelope(), nil); err == nil {
		t.Error("FinishPlan(ok, failWriter) = nil, want write error")
	}
}

// TestEveryWriteFailurePropagates steps a failing writer through each write
// of both summaries. A render that swallowed a mid-sequence write error
// would return nil for one of these positions.
func TestEveryWriteFailurePropagates(t *testing.T) {
	t.Parallel()

	fixtures := map[string]struct {
		render func(io.Writer, Envelope) error
		env    Envelope
	}{
		"impact": {renderImpactSummary, impactEnvelope()},
		"plan":   {renderPlanSummary, planEnvelope()},
	}
	for name, fx := range fixtures {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			total := countWrites(fx.render, fx.env)
			if total == 0 {
				t.Fatalf("%s performed no writes; the loop below proves nothing", name)
			}
			for i := range total {
				w := &dripWriter{failAfter: i}
				if err := fx.render(w, fx.env); err == nil {
					t.Errorf("%s with write %d failing returned nil, want write error", name, i+1)
				}
			}
		})
	}
}

func impactEnvelopeWithError() Envelope {
	env := impactEnvelope()
	env.Error = &EnvelopeError{Code: "stale", Message: "scope is stale"}
	return env
}

func TestRequestBodies(t *testing.T) {
	t.Parallel()

	opts := Options{
		RepoID: "repo-a", BaseRef: "base", HeadRef: "head",
		ChangedPaths: []string{"a.go"}, Changes: ModifiedFiles([]string{"a.go"}),
		Target: "svc", TargetType: "service", ServiceName: "svc", WorkloadID: "w",
		ResourceID: "r", ModuleID: "m", Topic: "t", Environment: "prod",
		MaxDepth: 4, Limit: 25, Offset: 5, DeveloperIntent: "roll out",
	}
	impact := ImpactRequestBody(opts)
	for _, key := range []string{
		"repo_id", "base_ref", "head_ref", "changed_paths", "changes", "target",
		"target_type", "service_name", "workload_id", "resource_id", "module_id",
		"topic", "environment", "max_depth", "limit", "offset",
	} {
		if _, ok := impact[key]; !ok {
			t.Errorf("ImpactRequestBody() missing key %q", key)
		}
	}
	if _, ok := impact["developer_intent"]; ok {
		t.Error("ImpactRequestBody() carries developer_intent, want impact without it")
	}
	if impact["repo_id"] != "repo-a" || impact["limit"] != 25 {
		t.Errorf("ImpactRequestBody() = %v, want option values", impact)
	}
	plan := PlanRequestBody(opts)
	if plan["developer_intent"] != "roll out" {
		t.Errorf("PlanRequestBody()[developer_intent] = %v, want the intent", plan["developer_intent"])
	}
	if plan["repo_id"] != "repo-a" {
		t.Errorf("PlanRequestBody() lost the impact keys: %v", plan)
	}
}

func TestFinishBranches(t *testing.T) {
	t.Parallel()

	env := impactEnvelope()
	// JSON mode returns the command error untouched after writing the envelope.
	var out bytes.Buffer
	sentinel := errors.New("cmd failed")
	if err := FinishImpact(&out, Options{JSON: true}, env, sentinel); err != sentinel {
		t.Errorf("FinishImpact(json, cmdErr) = %v, want the command error", err)
	}
	if !strings.Contains(out.String(), "changed_file_count") {
		t.Errorf("FinishImpact(json) wrote %q, want the envelope", out.String())
	}
	out.Reset()
	if err := FinishPlan(&out, Options{JSON: true}, env, nil); err != nil {
		t.Errorf("FinishPlan(json, nil) = %v, want nil", err)
	}
	// Human mode with an envelope error renders the error line.
	out.Reset()
	failEnv := Envelope{Error: &EnvelopeError{Code: "stale", Message: "scope is stale"}}
	if err := FinishImpact(&out, Options{}, failEnv, sentinel); err != sentinel {
		t.Errorf("FinishImpact(error, cmdErr) = %v, want the command error", err)
	}
	if !strings.Contains(out.String(), "Pre-change impact error (stale)") {
		t.Errorf("FinishImpact(error) wrote %q, want the error line", out.String())
	}
	// Human mode with data but no envelope error renders the summary anyway.
	out.Reset()
	if err := FinishImpact(&out, Options{}, env, sentinel); err != sentinel {
		t.Errorf("FinishImpact(data, cmdErr) = %v, want the command error", err)
	}
	if !strings.Contains(out.String(), "Pre-change impact") {
		t.Errorf("FinishImpact(data, cmdErr) wrote %q, want the summary", out.String())
	}
	// Human mode with neither error nor data returns the command error
	// with nothing rendered.
	out.Reset()
	bare := Envelope{}
	if err := FinishImpact(&out, Options{}, bare, sentinel); err != sentinel {
		t.Errorf("FinishImpact(bare, cmdErr) = %v, want the command error", err)
	}
	if out.String() != "" {
		t.Errorf("FinishImpact(bare, cmdErr) wrote %q, want nothing", out.String())
	}
	// Human success renders the plan summary for FinishPlan.
	out.Reset()
	if err := FinishPlan(&out, Options{}, planEnvelope(), nil); err != nil {
		t.Fatalf("FinishPlan(human) error = %v", err)
	}
	if !strings.Contains(out.String(), "Developer change plan") {
		t.Errorf("FinishPlan(human) wrote %q, want the plan summary", out.String())
	}
	// renderError is a no-op for an envelope with no error member.
	out.Reset()
	if err := renderError(&out, Envelope{}); err != nil || out.String() != "" {
		t.Errorf("renderError(clean) = (%q, %v), want nothing and nil", out.String(), err)
	}
}

// planEnvelope is a developer change plan answer with one well-formed action,
// one skipped non-object row, and one bounded follow-up call.
func planEnvelope() Envelope {
	return Envelope{
		Data: map[string]any{
			"changed_file_count": float64(1),
			"blocked":            false,
			"truncated":          false,
			"actions": []any{
				map[string]any{"kind": "edit", "risk": "low", "title": "update handler"},
				"not an object",
			},
			"bounded_next_calls": []any{
				map[string]any{"kind": "api", "target": "svc-a"},
				"not an object",
			},
		},
		Truth: map[string]any{"freshness": map[string]any{"state": "fresh"}},
	}
}
