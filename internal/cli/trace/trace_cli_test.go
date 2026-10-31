// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package trace_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/eshu-hq/eshu-cli/internal/cli/trace"
)

type fakeFetcher struct {
	path string
	resp trace.ServiceEnvelope
	err  error
}

func (f *fakeFetcher) GetEnvelope(path string, result any) error {
	f.path = path
	if f.err != nil {
		return f.err
	}
	if env, ok := result.(*trace.ServiceEnvelope); ok {
		*env = f.resp
	}
	return nil
}

type errWriter struct{ err error }

func (w errWriter) Write([]byte) (int, error) { return 0, w.err }

// failAfterWriter fails on the nth Write call (1-based), succeeding before
// that, so a sweep over n fails every sequential write position in turn.
type failAfterWriter struct {
	n   int
	err error
}

func (w *failAfterWriter) Write(p []byte) (int, error) {
	w.n--
	if w.n <= 0 {
		return 0, w.err
	}
	return len(p), nil
}

func TestFetchServiceStoryTrimsEscapesAndFilters(t *testing.T) {
	client := &fakeFetcher{}
	query := trace.ServiceQuery{Repo: "org/web", Environment: "prod", ServiceID: "svc-1"}
	if _, err := trace.FetchServiceStory(client, "  pay/svc ", query); err != nil {
		t.Fatalf("FetchServiceStory = %v", err)
	}
	want := "/api/v0/services/pay%2Fsvc/story?environment=prod&repo=org%2Fweb&service_id=svc-1"
	if client.path != want {
		t.Fatalf("FetchServiceStory requested %q, want %q", client.path, want)
	}
}

func TestFetchServiceStoryBareSelector(t *testing.T) {
	client := &fakeFetcher{}
	if _, err := trace.FetchServiceStory(client, "svc", trace.ServiceQuery{}); err != nil {
		t.Fatalf("FetchServiceStory = %v", err)
	}
	if client.path != "/api/v0/services/svc/story" {
		t.Fatalf("FetchServiceStory requested %q", client.path)
	}
}

func TestFetchServiceStoryTransportError(t *testing.T) {
	boom := errors.New("transport down")
	_, err := trace.FetchServiceStory(&fakeFetcher{err: boom}, "svc", trace.ServiceQuery{})
	if err != boom {
		t.Fatalf("FetchServiceStory = %v, want unwrapped transport error", err)
	}
}

func TestServiceFreshnessState(t *testing.T) {
	for selector, want := range map[string]string{
		"stale": "stale", "building": "building", "live": "live",
	} {
		env := trace.ServiceEnvelope{Truth: map[string]any{
			"freshness": map[string]any{"state": selector},
		}}
		if got := trace.ServiceFreshnessState(env); got != want {
			t.Fatalf("ServiceFreshnessState = %q, want %q", got, want)
		}
	}
	if got := trace.ServiceFreshnessState(trace.ServiceEnvelope{}); got != "" {
		t.Fatalf("ServiceFreshnessState(empty) = %q, want empty", got)
	}
}

func TestServiceStatus(t *testing.T) {
	env := trace.ServiceEnvelope{Data: map[string]any{
		"code_to_runtime_trace": map[string]any{"status": "partial"},
	}}
	if got := trace.ServiceStatus(env); got != "partial" {
		t.Fatalf("ServiceStatus = %q, want partial", got)
	}
	if got := trace.ServiceStatus(trace.ServiceEnvelope{}); got != "" {
		t.Fatalf("ServiceStatus(empty) = %q, want empty", got)
	}
}

func TestRenderServiceErrorShapes(t *testing.T) {
	var buf bytes.Buffer
	if err := trace.RenderServiceError(&buf, trace.ServiceEnvelope{}); err != nil {
		t.Fatalf("RenderServiceError = %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("RenderServiceError(nil) wrote %q, want nothing", buf.String())
	}
	other := trace.ServiceEnvelope{Error: &trace.ServiceError{Code: "denied", Message: "no"}}
	if err := trace.RenderServiceError(&buf, other); err != nil {
		t.Fatalf("RenderServiceError = %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("RenderServiceError(other) wrote %q, want nothing", buf.String())
	}
}

func TestRenderServiceErrorAmbiguous(t *testing.T) {
	env := trace.ServiceEnvelope{Error: &trace.ServiceError{
		Code: "ambiguous",
		Details: map[string]any{"candidates": []any{
			map[string]any{
				"service_id": "svc-1", "service_name": "web",
				"repo_id": "org/web", "environment": "prod",
			},
			map[string]any{"id": "svc-2", "name": "svc-2"},
			"not-a-row",
		}},
	}}
	var buf bytes.Buffer
	if err := trace.RenderServiceError(&buf, env); err != nil {
		t.Fatalf("RenderServiceError = %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Service selector is ambiguous.",
		"- svc-1 name=web repo=org/web env=prod\n",
		"- svc-2\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("RenderServiceError missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderServiceErrorDataCandidatesFallback(t *testing.T) {
	env := trace.ServiceEnvelope{
		Error: &trace.ServiceError{Code: "ambiguous"},
		Data:  map[string]any{"candidates": []any{map[string]any{}}},
	}
	var buf bytes.Buffer
	if err := trace.RenderServiceError(&buf, env); err != nil {
		t.Fatalf("RenderServiceError = %v", err)
	}
	if !strings.Contains(buf.String(), "- <unknown>\n") {
		t.Fatalf("RenderServiceError wrote %q, want unknown fallback", buf.String())
	}
}

func TestRenderServiceErrorWriteError(t *testing.T) {
	writeErr := errors.New("broken pipe")
	env := trace.ServiceEnvelope{Error: &trace.ServiceError{Code: "ambiguous"}}
	if err := trace.RenderServiceError(errWriter{writeErr}, env); err != writeErr {
		t.Fatalf("RenderServiceError = %v, want write error", err)
	}
}

func fullEnvelope() trace.ServiceEnvelope {
	return trace.ServiceEnvelope{
		Data: map[string]any{
			"service_identity": map[string]any{
				"service_name":           "web",
				"repo_id":                "org/web",
				"repo_name":              "web",
				"materialization_status": "done",
				"query_basis":            "live",
				"limitations":            []string{"", "edge-cache"},
			},
			"investigation": map[string]any{
				"coverage_summary": map[string]any{"state": "full", "reason": "all-segments"},
			},
			"code_to_runtime_trace": map[string]any{
				"status": "partial",
				"segments": []any{
					map[string]any{"name": "build", "status": "ok", "evidence_count": float64(3), "basis": "ci"},
					map[string]any{"name": "deploy", "status": "ok"},
					map[string]any{"name": "", "status": "ok"},
					"not-a-segment",
				},
				"missing_segments": []any{"canary", 7},
			},
			"deployment_lanes":      []any{"a", "b"},
			"runtime_instances":     []any{"i1"},
			"upstream_dependencies": []map[string]any{{"id": "u1"}},
			"downstream_consumers": map[string]any{
				"graph_dependent_count": float64(2), "content_consumer_count": 3,
			},
		},
		Truth: map[string]any{
			"freshness": map[string]any{"state": "live", "detail": "just-synced"},
		},
	}
}

func TestRenderServiceSummaryFull(t *testing.T) {
	var buf bytes.Buffer
	if err := trace.RenderServiceSummary(&buf, fullEnvelope()); err != nil {
		t.Fatalf("RenderServiceSummary = %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Service: web\n",
		"Repository: org/web (web)\n",
		"Materialization: done\n",
		"Basis: live\n",
		"Truth freshness: live\n",
		"Freshness detail: just-synced\n",
		"Code to runtime:\n",
		"Trace status: partial\n",
		"- build: ok (3 evidence) via ci\n",
		"- deploy: ok\n",
		"Missing evidence: canary\n",
		"Deployment lanes: 2\n",
		"Runtime instances: 1\n",
		"Upstream dependencies: 1\n",
		"Downstream consumers: 5\n",
		"Coverage: full\n",
		"Coverage reason: all-segments\n",
		"What to worry about:\n",
		"- edge-cache\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("RenderServiceSummary missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderServiceSummaryEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := trace.RenderServiceSummary(&buf, trace.ServiceEnvelope{}); err != nil {
		t.Fatalf("RenderServiceSummary = %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Service: <unknown>\n",
		"Repository: <unknown> (<unknown>)\n",
		"Deployment lanes: 0\n",
		"Runtime instances: 0\n",
		"Upstream dependencies: 0\n",
		"Downstream consumers: 0\n",
		"Coverage: unknown\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("RenderServiceSummary(empty) missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Code to runtime:") || strings.Contains(out, "Truth freshness:") {
		t.Fatalf("RenderServiceSummary(empty) rendered empty sections:\n%s", out)
	}
}

func TestRenderServiceSummaryFallbacks(t *testing.T) {
	env := trace.ServiceEnvelope{Data: map[string]any{
		"service_identity": map[string]any{
			"name":      "fallback-name",
			"instances": []any{"i1", "i2"},
		},
		"downstream_consumers": map[string]any{"items": []any{"a", "b", "c"}},
		"limitations":          []any{"stale f", "x"},
	}}
	var buf bytes.Buffer
	if err := trace.RenderServiceSummary(&buf, env); err != nil {
		t.Fatalf("RenderServiceSummary = %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Service: fallback-name\n",
		"Runtime instances: 2\n",
		"Downstream consumers: 3\n",
		"- stale f\n",
		"- x\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("RenderServiceSummary missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderServiceSummaryDownstreamList(t *testing.T) {
	for _, consumers := range []any{
		[]any{"a", "b"},
		[]map[string]any{{"id": "a"}, {"id": "b"}},
	} {
		env := trace.ServiceEnvelope{Data: map[string]any{"downstream_consumers": consumers}}
		var buf bytes.Buffer
		if err := trace.RenderServiceSummary(&buf, env); err != nil {
			t.Fatalf("RenderServiceSummary = %v", err)
		}
		if !strings.Contains(buf.String(), "Downstream consumers: 2\n") {
			t.Fatalf("RenderServiceSummary wrote:\n%s", buf.String())
		}
	}
}

func TestRenderServiceSummaryFreshnessReasonFallback(t *testing.T) {
	env := trace.ServiceEnvelope{Truth: map[string]any{
		"freshness": map[string]any{"state": "stale"},
		"reason":    "lagging",
	}}
	var buf bytes.Buffer
	if err := trace.RenderServiceSummary(&buf, env); err != nil {
		t.Fatalf("RenderServiceSummary = %v", err)
	}
	if !strings.Contains(buf.String(), "Freshness detail: lagging\n") {
		t.Fatalf("RenderServiceSummary wrote:\n%s", buf.String())
	}
}

func TestRenderServiceSummaryWriteError(t *testing.T) {
	writeErr := errors.New("broken pipe")
	if err := trace.RenderServiceSummary(errWriter{writeErr}, fullEnvelope()); err != writeErr {
		t.Fatalf("RenderServiceSummary = %v, want write error", err)
	}
}

func TestRenderServiceSummaryEveryWriteFailure(t *testing.T) {
	writeErr := errors.New("broken pipe")
	failed := 0
	for n := 2; n <= 80; n++ {
		err := trace.RenderServiceSummary(&failAfterWriter{n: n, err: writeErr}, fullEnvelope())
		if err == nil {
			break
		}
		if err != writeErr {
			t.Fatalf("n=%d: RenderServiceSummary = %v, want write error", n, err)
		}
		failed++
	}
	if failed == 0 {
		t.Fatal("no write position failed; sweep proves nothing")
	}
}

func TestRenderServiceErrorEveryWriteFailure(t *testing.T) {
	writeErr := errors.New("broken pipe")
	env := trace.ServiceEnvelope{Error: &trace.ServiceError{
		Code: "ambiguous",
		Details: map[string]any{"candidates": []any{map[string]any{
			"service_id": "s1", "service_name": "web",
			"repo_id": "org/web", "environment": "prod",
		}}},
	}}
	failed := 0
	for n := 2; n <= 20; n++ {
		err := trace.RenderServiceError(&failAfterWriter{n: n, err: writeErr}, env)
		if err == nil {
			break
		}
		if err != writeErr {
			t.Fatalf("n=%d: RenderServiceError = %v, want write error", n, err)
		}
		failed++
	}
	if failed == 0 {
		t.Fatal("no write position failed; sweep proves nothing")
	}
}

func TestRenderServiceSummarySegmentlessTrace(t *testing.T) {
	env := trace.ServiceEnvelope{Data: map[string]any{
		"code_to_runtime_trace": map[string]any{"status": "ok"},
	}}
	var buf bytes.Buffer
	if err := trace.RenderServiceSummary(&buf, env); err != nil {
		t.Fatalf("RenderServiceSummary = %v", err)
	}
	if strings.Contains(buf.String(), "Code to runtime:") {
		t.Fatalf("RenderServiceSummary rendered a segmentless section:\n%s", buf.String())
	}
}

func TestRenderServiceSummaryNilMapDownstream(t *testing.T) {
	env := trace.ServiceEnvelope{Data: map[string]any{
		"downstream_consumers": map[string]any(nil),
	}}
	var buf bytes.Buffer
	if err := trace.RenderServiceSummary(&buf, env); err != nil {
		t.Fatalf("RenderServiceSummary = %v", err)
	}
	if !strings.Contains(buf.String(), "Downstream consumers: 0\n") {
		t.Fatalf("RenderServiceSummary wrote:\n%s", buf.String())
	}
}
