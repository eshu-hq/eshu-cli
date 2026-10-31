// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package component_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/eshu-hq/eshu-cli/internal/cli/component"
)

type fakeFetcher struct {
	path string
	resp component.Envelope
	err  error
}

func (f *fakeFetcher) GetEnvelope(path string, result any) error {
	f.path = path
	if f.err != nil {
		return f.err
	}
	if env, ok := result.(*component.Envelope); ok {
		*env = f.resp
	}
	return nil
}

type errWriter struct{ err error }

func (w errWriter) Write([]byte) (int, error) { return 0, w.err }

// failAfterWriter fails on the nth Write call (1-based), succeeding before
// that, so a test can push a renderer past its header into a row failure.
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

func TestFetchInventoryDefaultLimit(t *testing.T) {
	client := &fakeFetcher{}
	if _, err := component.FetchInventory(client, 0); err != nil {
		t.Fatalf("FetchInventory = %v", err)
	}
	if client.path != "/api/v0/component-extensions?limit=100" {
		t.Fatalf("FetchInventory requested %q, want default limit 100", client.path)
	}
}

func TestFetchInventoryCustomLimit(t *testing.T) {
	client := &fakeFetcher{}
	if _, err := component.FetchInventory(client, 25); err != nil {
		t.Fatalf("FetchInventory = %v", err)
	}
	if client.path != "/api/v0/component-extensions?limit=25" {
		t.Fatalf("FetchInventory requested %q", client.path)
	}
}

func TestFetchInventoryTransportError(t *testing.T) {
	boom := errors.New("transport down")
	_, err := component.FetchInventory(&fakeFetcher{err: boom}, 10)
	if err != boom {
		t.Fatalf("FetchInventory = %v, want unwrapped transport error", err)
	}
}

func TestFetchDiagnosticsEscapesID(t *testing.T) {
	client := &fakeFetcher{}
	if _, err := component.FetchDiagnostics(client, "a/b c"); err != nil {
		t.Fatalf("FetchDiagnostics = %v", err)
	}
	want := "/api/v0/component-extensions/a%2Fb%20c/diagnostics"
	if client.path != want {
		t.Fatalf("FetchDiagnostics requested %q, want %q", client.path, want)
	}
}

func TestFetchDiagnosticsTransportError(t *testing.T) {
	boom := errors.New("transport down")
	_, err := component.FetchDiagnostics(&fakeFetcher{err: boom}, "c1")
	if err != boom {
		t.Fatalf("FetchDiagnostics = %v, want unwrapped transport error", err)
	}
}

func TestFinishAPIJSONWritesEnvelopeAndPassesError(t *testing.T) {
	env := component.Envelope{Data: map[string]any{"count": 1}}
	var buf bytes.Buffer
	if err := component.FinishAPI(&buf, true, env, nil); err != nil {
		t.Fatalf("FinishAPI = %v", err)
	}
	if !strings.Contains(buf.String(), "\n  \"data\"") {
		t.Fatalf("FinishAPI wrote non-indented JSON: %q", buf.String())
	}
	cmdErr := errors.New("exit 3")
	buf.Reset()
	if err := component.FinishAPI(&buf, true, env, cmdErr); err != cmdErr {
		t.Fatalf("FinishAPI = %v, want command error passthrough", err)
	}
	if buf.Len() == 0 {
		t.Fatal("FinishAPI wrote nothing in JSON failure mode, want the envelope")
	}
}

func TestFinishAPIJSONWriteErrorOutranks(t *testing.T) {
	writeErr := errors.New("disk full")
	cmdErr := errors.New("exit 3")
	env := component.Envelope{Data: map[string]any{"count": 1}}
	if err := component.FinishAPI(errWriter{writeErr}, true, env, cmdErr); err != writeErr {
		t.Fatalf("FinishAPI = %v, want write error", err)
	}
}

func TestFinishAPITextErrorRendersOneLine(t *testing.T) {
	env := component.Envelope{Error: &component.EnvelopeError{Code: "denied", Message: "no grant"}}
	var buf bytes.Buffer
	cmdErr := errors.New("exit 3")
	if err := component.FinishAPI(&buf, false, env, cmdErr); err != cmdErr {
		t.Fatalf("FinishAPI = %v, want command error passthrough", err)
	}
	want := "Component extension error (denied): no grant\n"
	if buf.String() != want {
		t.Fatalf("FinishAPI wrote %q, want %q", buf.String(), want)
	}
}

func TestFinishAPITextErrorWithoutEnvelopeErrorWritesNothing(t *testing.T) {
	var buf bytes.Buffer
	cmdErr := errors.New("exit 3")
	if err := component.FinishAPI(&buf, false, component.Envelope{}, cmdErr); err != cmdErr {
		t.Fatalf("FinishAPI = %v, want command error passthrough", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("FinishAPI wrote %q, want nothing", buf.String())
	}
}

func TestFinishAPISummaryRows(t *testing.T) {
	env := component.Envelope{
		Data: map[string]any{
			"count":       float64(2),
			"total_count": float64(9),
			"limit":       float64(100),
			"truncated":   true,
			"components": []any{
				map[string]any{
					"id":      "c1",
					"version": "v2",
					"states":  []any{"ready", " ", 7},
					"diagnostics": map[string]any{
						"policy_code":   "allow",
						"policy_reason": "grant-present",
					},
				},
				"not-a-row",
			},
		},
		Truth: map[string]any{"freshness": map[string]any{"state": "live"}},
	}
	var buf bytes.Buffer
	if err := component.FinishAPI(&buf, false, env, nil); err != nil {
		t.Fatalf("FinishAPI = %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Truth freshness: live\n",
		"Component extensions: 2 of 9 (limit=100, truncated=true)\n",
		"c1@v2 states=ready\n",
		"  policy=allow reason=grant-present\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("FinishAPI output missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "not-a-row") {
		t.Fatalf("FinishAPI rendered a non-row component:\n%s", out)
	}
}

func TestFinishAPISummarySingleComponent(t *testing.T) {
	env := component.Envelope{Data: map[string]any{
		"component": map[string]any{"id": "solo", "version": "v1"},
	}}
	var buf bytes.Buffer
	if err := component.FinishAPI(&buf, false, env, nil); err != nil {
		t.Fatalf("FinishAPI = %v", err)
	}
	if buf.String() != "solo@v1 states=\n" {
		t.Fatalf("FinishAPI wrote %q", buf.String())
	}
}

func TestFinishAPISummaryBareCount(t *testing.T) {
	env := component.Envelope{Data: map[string]any{"count": 3}}
	var buf bytes.Buffer
	if err := component.FinishAPI(&buf, false, env, nil); err != nil {
		t.Fatalf("FinishAPI = %v", err)
	}
	if buf.String() != "Component extensions: 3\n" {
		t.Fatalf("FinishAPI wrote %q", buf.String())
	}
}

func TestFinishAPIEmptyEnvelopeSummary(t *testing.T) {
	var buf bytes.Buffer
	if err := component.FinishAPI(&buf, false, component.Envelope{}, nil); err != nil {
		t.Fatalf("FinishAPI = %v", err)
	}
	if buf.String() != "Component extensions: 0\n" {
		t.Fatalf("FinishAPI wrote %q", buf.String())
	}
}

func TestFinishAPISummaryReaderShapes(t *testing.T) {
	env := component.Envelope{Data: map[string]any{
		"count":      "not-a-number",
		"components": []map[string]any{{"id": "g1", "version": 7, "states": []string{"on"}}},
	}}
	var buf bytes.Buffer
	if err := component.FinishAPI(&buf, false, env, nil); err != nil {
		t.Fatalf("FinishAPI = %v", err)
	}
	if buf.String() != "Component extensions: 0\ng1@ states=on\n" {
		t.Fatalf("FinishAPI wrote %q", buf.String())
	}
}

func TestFinishAPISummarySkipsWrongTypeComponents(t *testing.T) {
	env := component.Envelope{Data: map[string]any{"count": 1, "components": 42}}
	var buf bytes.Buffer
	if err := component.FinishAPI(&buf, false, env, nil); err != nil {
		t.Fatalf("FinishAPI = %v", err)
	}
	if buf.String() != "Component extensions: 1\n" {
		t.Fatalf("FinishAPI wrote %q", buf.String())
	}
}

func TestFinishAPIRowWithoutPolicyReason(t *testing.T) {
	env := component.Envelope{Data: map[string]any{
		"components": []any{
			map[string]any{"id": "c2", "diagnostics": map[string]any{"other": 1}},
		},
	}}
	var buf bytes.Buffer
	if err := component.FinishAPI(&buf, false, env, nil); err != nil {
		t.Fatalf("FinishAPI = %v", err)
	}
	if buf.String() != "Component extensions: 0\nc2@ states=\n" {
		t.Fatalf("FinishAPI wrote %q", buf.String())
	}
}

func TestFinishAPITextWriteErrors(t *testing.T) {
	writeErr := errors.New("broken pipe")
	withErr := component.Envelope{Error: &component.EnvelopeError{Code: "x"}}
	if err := component.FinishAPI(errWriter{writeErr}, false, withErr, errors.New("exit 1")); err != writeErr {
		t.Fatalf("FinishAPI error path = %v, want write error", err)
	}
	summary := component.Envelope{Truth: map[string]any{
		"freshness": map[string]any{"state": "live"},
	}}
	if err := component.FinishAPI(errWriter{writeErr}, false, summary, nil); err != writeErr {
		t.Fatalf("FinishAPI summary path = %v, want write error", err)
	}
	header := component.Envelope{Data: map[string]any{"total_count": 9}}
	if err := component.FinishAPI(errWriter{writeErr}, false, header, nil); err != writeErr {
		t.Fatalf("FinishAPI header path = %v, want write error", err)
	}
	bare := component.Envelope{Data: map[string]any{}}
	if err := component.FinishAPI(errWriter{writeErr}, false, bare, nil); err != writeErr {
		t.Fatalf("FinishAPI bare-count path = %v, want write error", err)
	}
	listed := component.Envelope{Data: map[string]any{
		"components": []any{map[string]any{"id": "c9"}},
	}}
	if err := component.FinishAPI(&failAfterWriter{n: 2, err: writeErr}, false, listed, nil); err != writeErr {
		t.Fatalf("FinishAPI row path = %v, want write error", err)
	}
	row := component.Envelope{Data: map[string]any{
		"component": map[string]any{"id": "c9"},
	}}
	if err := component.FinishAPI(errWriter{writeErr}, false, row, nil); err != writeErr {
		t.Fatalf("FinishAPI row path = %v, want write error", err)
	}
}
