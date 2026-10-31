// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package entitymap

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

// mappedEnv is a fully populated success envelope: a selected entity, two
// populated sections, and an untruncated evidence count.
func mappedEnv() Envelope {
	return Envelope{
		Data: map[string]any{
			"from":   "terraform/aws_lb.main",
			"status": "mapped",
			"resolution": map[string]any{
				"selected": map[string]any{
					"id": "terraform/aws_lb.main", "name": "main",
					"labels": []any{"LoadBalancer"},
				},
			},
			"sections": map[string]any{
				"depends_on": []any{
					map[string]any{
						"relationship_type": "DEPENDS_ON",
						"entity_name":       "vpc-a", "entity_id": "terraform/aws_vpc.a",
						"repo_id": "repo-a",
					},
					// Non-object rows are skipped, not fatal.
					"not-a-row",
				},
				"runs_as": []any{
					map[string]any{"relationship_type": "RUNS_AS", "entity_id": "arn:svc"},
				},
			},
			"evidence": map[string]any{"relationship_count": float64(2)},
		},
		Truth: map[string]any{"freshness": map[string]any{"state": "fresh"}},
	}
}

// truncatedEnv is mappedEnv with the evidence truncation flag set, so the
// write-stepping sweep reaches the "Truncated: true" write-error arm the
// untruncated fixture never emits.
func truncatedEnv() Envelope {
	env := mappedEnv()
	env.Data["evidence"] = map[string]any{"relationship_count": float64(2), "truncated": true}
	return env
}

// ambiguousEnv carries two candidates with different shapes: one with an id,
// a distinct name, and a repo; one identified by name only.
func ambiguousEnv() Envelope {
	return Envelope{
		Data: map[string]any{
			"status": "ambiguous",
			"resolution": map[string]any{
				"candidates": []any{
					map[string]any{"id": "svc-a", "name": "orders", "repo_id": "repo-a"},
					map[string]any{"name": "orders-staging"},
					"not-a-candidate",
				},
			},
		},
	}
}

func TestWriteJSONMode(t *testing.T) {
	var out bytes.Buffer
	if err := Write(&out, true, mappedEnv(), nil); err != nil {
		t.Fatalf("Write(json) error = %v, want nil", err)
	}
	if !strings.Contains(out.String(), "terraform/aws_lb.main") {
		t.Errorf("Write(json) = %q, want the envelope", out.String())
	}
	// HTML escaping stays off: a selector with & reads back as typed.
	amp := mappedEnv()
	amp.Data["from"] = "svc&a"
	out.Reset()
	if err := Write(&out, true, amp, nil); err != nil {
		t.Fatalf("Write(json) error = %v, want nil", err)
	}
	if !strings.Contains(out.String(), "svc&a") {
		t.Errorf("Write(json) = %q, want the unescaped selector", out.String())
	}
	if strings.Contains(out.String(), "\\u0026") {
		t.Errorf("Write(json) escaped the selector: %q", out.String())
	}
}

func TestWriteFailurePath(t *testing.T) {
	// An ambiguous failure prints the candidate guidance and reports no
	// render error: the caller reports the Failure separately.
	var out bytes.Buffer
	failure := &Failure{Kind: FailureAmbiguous, Code: "ambiguous"}
	if err := Write(&out, false, ambiguousEnv(), failure); err != nil {
		t.Fatalf("Write(failure) error = %v, want nil", err)
	}
	if !strings.Contains(out.String(), "ambiguous") {
		t.Errorf("Write(failure) = %q, want the guidance", out.String())
	}
	// A non-ambiguous failure prints nothing and lets the caller message
	// stand alone.
	out.Reset()
	plain := &Failure{Kind: FailureNoMatch, Code: "no_match", Message: "no match"}
	if err := Write(&out, false, mappedEnv(), plain); err != nil {
		t.Fatalf("Write(no-match failure) error = %v, want nil", err)
	}
	if out.Len() != 0 {
		t.Errorf("Write(no-match failure) = %q, want nothing", out.String())
	}
	// A render error on the failing path is dropped: the caller is about
	// to report the failure, and "short write" would lose the reason.
	if err := Write(failWriter{}, false, ambiguousEnv(), failure); err != nil {
		t.Errorf("Write(failing writer, failure) = %v, want nil (render error dropped)", err)
	}
}

func TestWriteSuccessPath(t *testing.T) {
	var out bytes.Buffer
	if err := Write(&out, false, mappedEnv(), nil); err != nil {
		t.Fatalf("Write(summary) error = %v, want nil", err)
	}
	if !strings.Contains(out.String(), "Map: terraform/aws_lb.main") {
		t.Errorf("Write(summary) = %q, want the map line", out.String())
	}
	if err := Write(failWriter{}, false, mappedEnv(), nil); err == nil {
		t.Error("Write(failWriter, summary) = nil, want write error")
	}
	if err := Write(failWriter{}, true, mappedEnv(), nil); err == nil {
		t.Error("Write(failWriter, json) = nil, want write error")
	}
}

func TestRenderError(t *testing.T) {
	var out bytes.Buffer
	if err := RenderError(&out, ambiguousEnv()); err != nil {
		t.Fatalf("RenderError(ambiguous) error = %v, want nil", err)
	}
	got := out.String()
	for _, want := range []string{
		"Map selector is ambiguous",
		"- svc-a name=orders repo=repo-a",
		"- orders-staging",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("RenderError() missing %q in:\n%s", want, got)
		}
	}
	// id == name prints no name= suffix; a row with neither prints unknown.
	same := Envelope{Data: map[string]any{
		"status": "ambiguous",
		"resolution": map[string]any{"candidates": []any{
			map[string]any{"id": "svc-a", "name": "svc-a"},
			map[string]any{},
		}},
	}}
	out.Reset()
	if err := RenderError(&out, same); err != nil {
		t.Fatalf("RenderError(same) error = %v, want nil", err)
	}
	if strings.Contains(out.String(), "name=") {
		t.Errorf("RenderError() printed a name= suffix for identical id/name:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "<unknown>") {
		t.Errorf("RenderError() = %q, want the unknown placeholder", out.String())
	}
	// Anything but ambiguous prints nothing.
	out.Reset()
	if err := RenderError(&out, mappedEnv()); err != nil {
		t.Fatalf("RenderError(mapped) error = %v, want nil", err)
	}
	if out.Len() != 0 {
		t.Errorf("RenderError(mapped) = %q, want nothing", out.String())
	}
	// No data at all is not ambiguous either.
	out.Reset()
	if err := RenderError(&out, Envelope{}); err != nil {
		t.Fatalf("RenderError(empty) error = %v, want nil", err)
	}
	if out.Len() != 0 {
		t.Errorf("RenderError(empty) = %q, want nothing", out.String())
	}
}

func TestRenderSummary(t *testing.T) {
	var out bytes.Buffer
	if err := RenderSummary(&out, mappedEnv()); err != nil {
		t.Fatalf("RenderSummary() error = %v, want nil", err)
	}
	got := out.String()
	for _, want := range []string{
		"Map: terraform/aws_lb.main",
		"Resolved: LoadBalancer terraform/aws_lb.main (main)",
		"Depends on:",
		"- DEPENDS_ON vpc-a repo=repo-a",
		"Runs as:",
		"- RUNS_AS arn:svc",
		"Evidence: 2 relationships",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("RenderSummary() missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Truncated") {
		t.Errorf("RenderSummary() mentions truncation for an untruncated map:\n%s", got)
	}
	// Empty sections print no title: a map with two populated sections
	// does not read as five sections that mostly failed.
	for _, title := range []string{"Defined by:", "Deployed by:", "Consumed by:"} {
		if strings.Contains(got, title) {
			t.Errorf("RenderSummary() prints empty section %q", title)
		}
	}
}

func TestRenderSummaryVariants(t *testing.T) {
	// No selected entity means no Resolved line, but the rest still prints.
	noSelected := mappedEnv()
	delete(noSelected.Data["resolution"].(map[string]any), "selected")
	var out bytes.Buffer
	if err := RenderSummary(&out, noSelected); err != nil {
		t.Fatalf("RenderSummary(no selected) error = %v, want nil", err)
	}
	if strings.Contains(out.String(), "Resolved:") {
		t.Errorf("RenderSummary(no selected) prints a Resolved line:\n%s", out.String())
	}
	// No labels print as Entity; id-only selection prints without parens.
	bare := Envelope{Data: map[string]any{
		"from":       "svc-a",
		"resolution": map[string]any{"selected": map[string]any{"id": "svc-a"}},
		"evidence":   map[string]any{"relationship_count": float64(0), "truncated": true},
	}}
	out.Reset()
	if err := RenderSummary(&out, bare); err != nil {
		t.Fatalf("RenderSummary(bare) error = %v, want nil", err)
	}
	got := out.String()
	if !strings.Contains(got, "Resolved: Entity svc-a\n") {
		t.Errorf("RenderSummary(bare) = %q, want the Entity fallback without parens", got)
	}
	if !strings.Contains(got, "Truncated: true") {
		t.Errorf("RenderSummary(bare) = %q, want the truncation flag", got)
	}
	// A selected entity with an empty id falls back to its name.
	named := Envelope{Data: map[string]any{
		"from":       "svc-a",
		"resolution": map[string]any{"selected": map[string]any{"name": "orders", "labels": []any{"Service"}}},
	}}
	out.Reset()
	if err := RenderSummary(&out, named); err != nil {
		t.Fatalf("RenderSummary(named) error = %v, want nil", err)
	}
	if !strings.Contains(out.String(), "Resolved: Service orders (orders)") {
		t.Errorf("RenderSummary(named) = %q, want the name fallback", out.String())
	}
}

func TestRenderSection(t *testing.T) {
	// Empty renders nothing, not even the title.
	var out bytes.Buffer
	if err := RenderSection(&out, "Depends on", nil); err != nil {
		t.Fatalf("RenderSection(empty) error = %v, want nil", err)
	}
	if out.Len() != 0 {
		t.Errorf("RenderSection(empty) = %q, want nothing", out.String())
	}
	// Non-object rows skip; nameless rows fall back to the entity id.
	rows := []any{
		map[string]any{"relationship_type": "DEPENDS_ON", "entity_id": "terraform/aws_vpc.a"},
		"not-a-row",
		map[string]any{"relationship_type": "RUNS_AS", "entity_name": "job", "repo_id": "repo-b"},
	}
	out.Reset()
	if err := RenderSection(&out, "Depends on", rows); err != nil {
		t.Fatalf("RenderSection() error = %v, want nil", err)
	}
	got := out.String()
	for _, want := range []string{
		"Depends on:",
		"- DEPENDS_ON terraform/aws_vpc.a",
		"- RUNS_AS job repo=repo-b",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("RenderSection() missing %q in:\n%s", want, got)
		}
	}
}

func TestDisplayLabel(t *testing.T) {
	if got := displayLabel(map[string]any{"labels": []any{"Service", "Other"}}); got != "Service" {
		t.Errorf("displayLabel() = %q, want the first label", got)
	}
	if got := displayLabel(map[string]any{"labels": []string{"Service"}}); got != "Service" {
		t.Errorf("displayLabel([]string) = %q, want Service", got)
	}
	if got := displayLabel(map[string]any{}); got != "Entity" {
		t.Errorf("displayLabel(no labels) = %q, want Entity", got)
	}
	if got := displayLabel(nil); got != "Entity" {
		t.Errorf("displayLabel(nil) = %q, want Entity", got)
	}
}

// countWrites measures how many writes a render performs on its fixture.
func countWrites(f func(w *dripWriter) error) int {
	w := &dripWriter{failAfter: 1 << 30}
	_ = f(w)
	return w.writes
}

func TestWriteErrorArms(t *testing.T) {
	if err := RenderError(failWriter{}, ambiguousEnv()); err == nil {
		t.Error("RenderError(failWriter) = nil, want write error")
	}
	if err := RenderSummary(failWriter{}, mappedEnv()); err == nil {
		t.Error("RenderSummary(failWriter) = nil, want write error")
	}
	if err := RenderSection(failWriter{}, "T", []any{map[string]any{"a": "b"}}); err == nil {
		t.Error("RenderSection(failWriter) = nil, want write error")
	}
	if err := WriteJSON(failWriter{}, mappedEnv()); err == nil {
		t.Error("WriteJSON(failWriter) = nil, want write error")
	}
}

// A render that swallowed a mid-sequence write error would return nil here.
func TestEveryWriteFailurePropagates(t *testing.T) {
	type fixture struct {
		name   string
		render func(w *dripWriter) error
	}
	sectionRows := []any{map[string]any{
		"relationship_type": "DEPENDS_ON", "entity_name": "vpc-a", "repo_id": "repo-a",
	}}
	fixtures := []fixture{
		{name: "summary", render: func(w *dripWriter) error { return RenderSummary(w, mappedEnv()) }},
		{name: "summary-truncated", render: func(w *dripWriter) error { return RenderSummary(w, truncatedEnv()) }},
		{name: "ambiguous-error", render: func(w *dripWriter) error { return RenderError(w, ambiguousEnv()) }},
		{name: "section", render: func(w *dripWriter) error {
			return RenderSection(w, "Depends on", sectionRows)
		}},
	}
	for _, fx := range fixtures {
		t.Run(fx.name, func(t *testing.T) {
			total := countWrites(fx.render)
			if total == 0 {
				t.Fatalf("%s performed no writes; the loop below proves nothing", fx.name)
			}
			for i := 0; i < total; i++ {
				w := &dripWriter{failAfter: i}
				if err := fx.render(w); err == nil {
					t.Errorf("%s with write %d failing returned nil, want write error", fx.name, i+1)
				}
			}
		})
	}
}
