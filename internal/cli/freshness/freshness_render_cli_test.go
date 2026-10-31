// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package freshness

import (
	"bytes"
	"errors"
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
func countWrites(render summaryRenderer, env Envelope) int {
	w := &dripWriter{failAfter: 1 << 30}
	_ = render(w, env)
	return w.writes
}

// fakeFetcher serves canned envelopes by path for the Run tests.
type fakeFetcher struct {
	env Envelope
	err error
}

func (f fakeFetcher) GetEnvelope(path string, result any) error {
	if f.err != nil {
		return f.err
	}
	if env, ok := result.(*Envelope); ok {
		*env = f.env
	}
	return nil
}

func generationsEnv() Envelope {
	return Envelope{
		Truth: map[string]any{"freshness": map[string]any{"state": "fresh"}},
		Data: map[string]any{
			"count":     float64(2),
			"truncated": true,
			"generations": []any{
				map[string]any{
					"generation_id": "gen-2", "status": "active",
					"scope_id": "scope-a", "trigger_kind": "schedule",
					"is_active":    true,
					"queue_status": map[string]any{"outstanding": float64(1), "failed": float64(0), "dead_letter": float64(0)},
					"latest_failure": map[string]any{
						"failure_class": "timeout",
						"prior_failure": map[string]any{"failure_class": "oom"},
					},
				},
				// Non-object rows are skipped, not fatal.
				"not-a-row",
				map[string]any{
					"generation_id": "gen-1", "status": "complete",
					"scope_id": "scope-a", "trigger_kind": "manual",
				},
			},
		},
	}
}

func TestRenderGenerationsSummary(t *testing.T) {
	var out bytes.Buffer
	if err := RenderGenerationsSummary(&out, generationsEnv()); err != nil {
		t.Fatalf("RenderGenerationsSummary() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"Truth freshness: fresh",
		"Generations: 2 (truncated=true)",
		"* gen-2 status=active",
		"queue[outstanding=1 failed=0 dead_letter=0]",
		"failure=timeout prior_failure=oom",
		"  gen-1 status=complete",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("RenderGenerationsSummary() missing %q in:\n%s", want, got)
		}
	}
}

func changedSinceEnv() Envelope {
	return Envelope{
		Data: map[string]any{
			"since_generation_id":          "gen-1",
			"current_active_generation_id": "gen-2",
			"scope_id":                     "scope-a",
			"categories": []any{
				map[string]any{
					"category": "files",
					"counts": map[string]any{
						"added": float64(1), "updated": float64(2),
						"unchanged": float64(3), "retired": float64(0),
						"superseded": float64(0),
					},
				},
				map[string]any{"category": "services", "unavailable": true},
				"not-a-category",
			},
		},
	}
}

func TestRenderChangedSinceSummary(t *testing.T) {
	var out bytes.Buffer
	if err := RenderChangedSinceSummary(&out, changedSinceEnv()); err != nil {
		t.Fatalf("RenderChangedSinceSummary() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"Changed since gen-1 -> gen-2 (scope=scope-a)",
		"files", "added=1 updated=2 unchanged=3 retired=0 superseded=0",
		"services", "unavailable",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("RenderChangedSinceSummary() missing %q in:\n%s", want, got)
		}
	}
}

func TestRenderChangedSinceUnavailable(t *testing.T) {
	env := Envelope{Data: map[string]any{
		"since_generation_id": "gen-1", "scope_id": "scope-a",
		"unavailable": true,
	}}
	var out bytes.Buffer
	if err := RenderChangedSinceSummary(&out, env); err != nil {
		t.Fatalf("RenderChangedSinceSummary() error = %v", err)
	}
	if !strings.Contains(out.String(), "diff unavailable") {
		t.Errorf("RenderChangedSinceSummary() = %q, want diff-unavailable notice", out.String())
	}
}

// serviceEnv is a service changed-since envelope with family rows, so the
// write-stepping test reaches the category writes on that path too.
func serviceEnv() Envelope {
	return Envelope{
		Truth: map[string]any{"freshness": map[string]any{"state": "fresh"}},
		Data: map[string]any{
			"since_generation_id":          "gen-1",
			"current_active_generation_id": "gen-2",
			"service_id":                   "svc-a",
			"categories": []any{map[string]any{
				"category": "evidence",
				"counts":   map[string]any{"added": float64(4)},
			}},
		},
	}
}

func TestRenderServiceChangedSinceSummary(t *testing.T) {
	env := Envelope{Data: map[string]any{
		"since_generation_id":          "gen-1",
		"current_active_generation_id": "gen-2",
		"service_id":                   "svc-a",
		"categories": []any{map[string]any{
			"category": "evidence",
			"counts":   map[string]any{"added": float64(4)},
		}},
	}}
	var out bytes.Buffer
	if err := RenderServiceChangedSinceSummary(&out, env); err != nil {
		t.Fatalf("RenderServiceChangedSinceSummary() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"Service changed since gen-1 -> gen-2 (service=svc-a)",
		"evidence", "added=4",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("RenderServiceChangedSinceSummary() missing %q in:\n%s", want, got)
		}
	}
	// No current generation renders the service notice, not zeroed counts.
	out.Reset()
	env.Data["unavailable"] = true
	if err := RenderServiceChangedSinceSummary(&out, env); err != nil {
		t.Fatalf("RenderServiceChangedSinceSummary() error = %v", err)
	}
	if !strings.Contains(out.String(), "service has no current active generation") {
		t.Errorf("RenderServiceChangedSinceSummary() = %q, want service notice", out.String())
	}
}

func TestRunGenerations(t *testing.T) {
	// Human success path.
	var out bytes.Buffer
	if err := RunGenerations(&out, fakeFetcher{env: generationsEnv()}, GenerationsOptions{}); err != nil {
		t.Fatalf("RunGenerations() error = %v", err)
	}
	if !strings.Contains(out.String(), "Generations: 2") {
		t.Errorf("RunGenerations() wrote %q, want the summary", out.String())
	}
	// JSON success path writes the envelope.
	out.Reset()
	if err := RunGenerations(&out, fakeFetcher{env: generationsEnv()}, GenerationsOptions{JSON: true}); err != nil {
		t.Fatalf("RunGenerations(json) error = %v", err)
	}
	if !strings.Contains(out.String(), "gen-2") {
		t.Errorf("RunGenerations(json) wrote %q, want the envelope", out.String())
	}
	// Transport failure reports through the envelope shape.
	out.Reset()
	err := RunGenerations(&out, fakeFetcher{err: errors.New("dial tcp: connection refused")}, GenerationsOptions{})
	if err == nil {
		t.Fatal("RunGenerations(transport fail) = nil, want failure")
	}
	if !strings.Contains(out.String(), "connection refused") {
		t.Errorf("RunGenerations(transport fail) wrote %q, want the transport text", out.String())
	}
	// Server-reported error returns a *Failure.
	out.Reset()
	env := generationsEnv()
	env.Error = &EnvelopeError{Code: "stale", Message: "scope is stale"}
	err = RunGenerations(&out, fakeFetcher{env: env}, GenerationsOptions{})
	var failure *Failure
	if !errors.As(err, &failure) {
		t.Fatalf("RunGenerations(server error) = %v (%T), want *Failure", err, err)
	}
	if failure.Code != 4 {
		t.Errorf("failure.Code = %d, want 4", failure.Code)
	}
}

func TestRunChangedSince(t *testing.T) {
	var out bytes.Buffer
	if err := RunChangedSince(&out, fakeFetcher{env: changedSinceEnv()}, ChangedSinceOptions{}); err != nil {
		t.Fatalf("RunChangedSince() error = %v", err)
	}
	if !strings.Contains(out.String(), "Changed since gen-1") {
		t.Errorf("RunChangedSince() wrote %q, want the summary", out.String())
	}
	out.Reset()
	err := RunChangedSince(&out, fakeFetcher{err: errors.New("boom")}, ChangedSinceOptions{})
	if err == nil {
		t.Fatal("RunChangedSince(transport fail) = nil, want failure")
	}
}

func TestRunServiceChangedSince(t *testing.T) {
	env := Envelope{Data: map[string]any{
		"since_generation_id": "gen-1", "current_active_generation_id": "gen-2",
		"service_id": "svc-a",
	}}
	var out bytes.Buffer
	if err := RunServiceChangedSince(&out, fakeFetcher{env: env}, ServiceChangedSinceOptions{}); err != nil {
		t.Fatalf("RunServiceChangedSince() error = %v", err)
	}
	if !strings.Contains(out.String(), "Service changed since gen-1") {
		t.Errorf("RunServiceChangedSince() wrote %q, want the summary", out.String())
	}
	out.Reset()
	err := RunServiceChangedSince(&out, fakeFetcher{err: errors.New("boom")}, ServiceChangedSinceOptions{})
	if err == nil {
		t.Fatal("RunServiceChangedSince(transport fail) = nil, want failure")
	}
}

func TestWriteErrorArms(t *testing.T) {
	w := failWriter{}
	renderers := map[string]summaryRenderer{
		"generations":           RenderGenerationsSummary,
		"changed-since":         RenderChangedSinceSummary,
		"service-changed-since": RenderServiceChangedSinceSummary,
	}
	for name, render := range renderers {
		t.Run(name, func(t *testing.T) {
			if err := render(w, generationsEnv()); err == nil {
				t.Errorf("%s(failWriter) = nil, want write error", name)
			}
		})
	}
	if err := RenderEnvelopeError(w, Envelope{Error: &EnvelopeError{Code: "x"}}); err == nil {
		t.Error("RenderEnvelopeError(failWriter) = nil, want write error")
	}
	if err := renderGenerationRow(w, map[string]any{"generation_id": "gen-1"}); err == nil {
		t.Error("renderGenerationRow(failWriter) = nil, want write error")
	}
	if err := renderCategory(w, map[string]any{"category": "files"}); err == nil {
		t.Error("renderCategory(failWriter) = nil, want write error")
	}
	if err := finish(w, true, generationsEnv(), RenderGenerationsSummary, nil); err == nil {
		t.Error("finish(json, failWriter) = nil, want write error")
	}
	// Human mode with a command error renders the envelope error first, so
	// the failing writer surfaces the render error.
	failEnv := Envelope{Error: &EnvelopeError{Code: "stale", Message: "scope is stale"}}
	cmdErr := errors.New("cmd failed")
	if err := finish(w, false, failEnv, RenderGenerationsSummary, cmdErr); err == nil {
		t.Error("finish(human, failWriter) = nil, want write error")
	}
	// Human success renders the summary, so the failing writer errors.
	if err := finish(w, false, generationsEnv(), RenderGenerationsSummary, nil); err == nil {
		t.Error("finish(human ok, failWriter) = nil, want write error")
	}
}

// A render that swallowed a mid-sequence write error would return nil here.
func TestEveryWriteFailurePropagates(t *testing.T) {
	fixtures := map[string]struct {
		render summaryRenderer
		env    Envelope
	}{
		"generations":           {RenderGenerationsSummary, generationsEnv()},
		"changed-since":         {RenderChangedSinceSummary, changedSinceEnv()},
		"service-changed-since": {RenderServiceChangedSinceSummary, serviceEnv()},
	}
	for name, fx := range fixtures {
		t.Run(name, func(t *testing.T) {
			total := countWrites(fx.render, fx.env)
			if total == 0 {
				t.Fatalf("%s performed no writes; the loop below proves nothing", name)
			}
			for i := 0; i < total; i++ {
				w := &dripWriter{failAfter: i}
				if err := fx.render(w, fx.env); err == nil {
					t.Errorf("%s with write %d failing returned nil, want write error", name, i+1)
				}
			}
		})
	}
}
