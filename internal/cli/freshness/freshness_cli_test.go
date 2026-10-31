// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package freshness

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// statusErr is a test EnvelopeFetcher-side error carrying an HTTP status,
// exercising the apierr.StatusCode branch of ErrorCodeFromTransport.
type statusErr struct {
	status int
	msg    string
}

func (e statusErr) Error() string       { return e.msg }
func (e statusErr) HTTPStatusCode() int { return e.status }

func TestEnvelopeFailureNil(t *testing.T) {
	if got := EnvelopeFailure(nil); got != nil {
		t.Fatalf("EnvelopeFailure(nil) = %+v, want nil", got)
	}
}

func TestEnvelopeFailureMessage(t *testing.T) {
	got := EnvelopeFailure(&EnvelopeError{Code: "stale", Message: "scope is stale"})
	if got == nil {
		t.Fatal("EnvelopeFailure() = nil, want failure")
	}
	if got.Message != "scope is stale" {
		t.Errorf("Message = %q, want %q", got.Message, "scope is stale")
	}
	if got.Code != 4 {
		t.Errorf("Code = %d, want 4", got.Code)
	}
}

func TestEnvelopeFailureCodeFallback(t *testing.T) {
	got := EnvelopeFailure(&EnvelopeError{Code: "ambiguous"})
	if got == nil {
		t.Fatal("EnvelopeFailure() = nil, want failure")
	}
	if got.Message != "ambiguous" {
		t.Errorf("Message = %q, want code fallback %q", got.Message, "ambiguous")
	}
	if got.Code != 3 {
		t.Errorf("Code = %d, want 3", got.Code)
	}
}

func TestEnvelopeFailureWhitespaceFallsBack(t *testing.T) {
	got := EnvelopeFailure(&EnvelopeError{Code: "  ", Message: "   "})
	if got == nil {
		t.Fatal("EnvelopeFailure() = nil, want failure")
	}
	if got.Message != "generation lifecycle request failed" {
		t.Errorf("Message = %q, want fixed sentence", got.Message)
	}
}

func TestExitCodeTable(t *testing.T) {
	cases := map[string]int{
		"ambiguous":              3,
		"index_building":         4,
		"stale":                  4,
		"capability_degraded":    5,
		"partial":                5,
		"unsupported_capability": 6,
		"invalid_argument":       2,
		"not_found":              2,
		"scope_not_found":        2,
		"something_else":         1,
		"":                       1,
		"  stale  ":              4,
	}
	for code, want := range cases {
		if got := ExitCodeForErrorCode(code); got != want {
			t.Errorf("ExitCodeForErrorCode(%q) = %d, want %d", code, got, want)
		}
	}
}

func TestExitCodeForErrorCodeRejectsStateSpellings(t *testing.T) {
	// Freshness states are not error codes: they exit 0 via Render paths,
	// never through this table.
	for _, state := range []string{"building", "fresh", "unknown"} {
		if got := ExitCodeForErrorCode(state); got != 1 {
			t.Errorf("ExitCodeForErrorCode(%q) = %d, want 1 (not a code)", state, got)
		}
	}
}

func TestErrorCodeFromTransport(t *testing.T) {
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
			if got := ErrorCodeFromTransport(tc.err); got != tc.want {
				t.Errorf("ErrorCodeFromTransport(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

// TestErrorCodeFromTransportPrefersMessageOverStatus pins the load-bearing
// ordering in ErrorCodeFromTransport (owner advisory, #48): the message
// checks run before the status switch, so an error carrying BOTH an HTTP
// status AND a refused/failed message classifies as backend_unavailable
// rather than by its status.
func TestErrorCodeFromTransportPrefersMessageOverStatus(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"refused-beats-400", statusErr{status: 400, msg: "dial tcp: connection refused"}, "backend_unavailable"},
		{"failed-beats-404", statusErr{status: 404, msg: "request failed after retries"}, "backend_unavailable"},
		{"refused-beats-418", statusErr{status: 418, msg: "connection refused by peer"}, "backend_unavailable"},
		{"failed-beats-501", statusErr{status: 501, msg: "request failed: upstream reset"}, "backend_unavailable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ErrorCodeFromTransport(tc.err); got != tc.want {
				t.Errorf("ErrorCodeFromTransport(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

// TestExitCodeForTransportProducedCodes pins the transport-produced codes
// through ExitCodeForErrorCode (owner advisory, #48): backend_unavailable
// and api_error both exit 1.
func TestExitCodeForTransportProducedCodes(t *testing.T) {
	for _, code := range []string{"backend_unavailable", "api_error"} {
		if got := ExitCodeForErrorCode(code); got != 1 {
			t.Errorf("ExitCodeForErrorCode(%q) = %d, want 1", code, got)
		}
	}
}

func TestFinishJSONModes(t *testing.T) {
	env := Envelope{Data: map[string]any{"count": float64(1)}}
	// JSON mode writes the envelope and returns the command error untouched.
	var out bytes.Buffer
	sentinel := errors.New("cmd failed")
	if err := finish(&out, true, env, RenderGenerationsSummary, sentinel); err != sentinel {
		t.Errorf("finish(json, cmdErr) = %v, want the command error", err)
	}
	if !strings.Contains(out.String(), "count") {
		t.Errorf("finish(json) wrote %q, want the envelope", out.String())
	}
	// JSON mode with no command error returns nil.
	out.Reset()
	if err := finish(&out, true, env, RenderGenerationsSummary, nil); err != nil {
		t.Errorf("finish(json, nil) = %v, want nil", err)
	}
}

func TestFinishHumanModes(t *testing.T) {
	// Human mode with no error renders the summary.
	var out bytes.Buffer
	env := Envelope{Data: map[string]any{"count": float64(2)}}
	if err := finish(&out, false, env, RenderGenerationsSummary, nil); err != nil {
		t.Fatalf("finish(human, nil) error = %v", err)
	}
	if !strings.Contains(out.String(), "Generations: 2") {
		t.Errorf("finish(human) wrote %q, want the summary", out.String())
	}
	// Human mode with a command error renders the envelope error and
	// returns the command error.
	out.Reset()
	failEnv := Envelope{Error: &EnvelopeError{Code: "stale", Message: "scope is stale"}}
	cmdErr := &Failure{Message: "scope is stale", Code: 4}
	if err := finish(&out, false, failEnv, RenderGenerationsSummary, cmdErr); err != cmdErr {
		t.Errorf("finish(human, cmdErr) = %v, want the command error", err)
	}
	if !strings.Contains(out.String(), "scope is stale") {
		t.Errorf("finish(human, cmdErr) wrote %q, want the error line", out.String())
	}
}

func TestGenerationsPath(t *testing.T) {
	if got := GenerationsPath(GenerationsOptions{}); got != GenerationsRoute {
		t.Errorf("GenerationsPath(zero) = %q, want bare route %q", got, GenerationsRoute)
	}
	got := GenerationsPath(GenerationsOptions{
		ScopeID: "  scope-a ", Repository: "repo-a", Limit: 10,
	})
	for _, want := range []string{"scope_id=scope-a", "repository=repo-a", "limit=10"} {
		if !strings.Contains(got, want) {
			t.Errorf("GenerationsPath() = %q, want %q", got, want)
		}
	}
	// Non-positive limits are omitted so the server default applies.
	if got := GenerationsPath(GenerationsOptions{Limit: -1}); got != GenerationsRoute {
		t.Errorf("GenerationsPath(limit=-1) = %q, want bare route", got)
	}
}

func TestChangedSincePath(t *testing.T) {
	if got := ChangedSincePath(ChangedSinceOptions{}); got != ChangedSinceRoute {
		t.Errorf("ChangedSincePath(zero) = %q, want bare route %q", got, ChangedSinceRoute)
	}
	got := ChangedSincePath(ChangedSinceOptions{
		ScopeID: "scope-a", SinceGenerationID: "gen-1",
		SinceObservedAt: "2026-01-01T00:00:00Z", SampleLimit: 5,
	})
	for _, want := range []string{
		"scope_id=scope-a", "since_generation_id=gen-1",
		"since_observed_at=2026-01-01T00%3A00%3A00Z", "sample_limit=5",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("ChangedSincePath() = %q, want %q", got, want)
		}
	}
}

func TestServiceChangedSincePath(t *testing.T) {
	if got := ServiceChangedSincePath(ServiceChangedSinceOptions{}); got != ServiceChangedSinceRoute {
		t.Errorf("ServiceChangedSincePath(zero) = %q, want bare route %q", got, ServiceChangedSinceRoute)
	}
	got := ServiceChangedSincePath(ServiceChangedSinceOptions{
		ServiceID: "svc-a", ScopeID: "scope-a",
		SinceGenerationID: "gen-1", SampleLimit: 3,
	})
	for _, want := range []string{
		"service_id=svc-a", "scope_id=scope-a",
		"since_generation_id=gen-1", "sample_limit=3",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("ServiceChangedSincePath() = %q, want %q", got, want)
		}
	}
}

func TestRenderEnvelopeError(t *testing.T) {
	var out bytes.Buffer
	if err := RenderEnvelopeError(&out, Envelope{}); err != nil {
		t.Fatalf("RenderEnvelopeError() error = %v", err)
	}
	if out.String() != "" {
		t.Errorf("RenderEnvelopeError() wrote %q for nil error, want nothing", out.String())
	}
	out.Reset()
	env := Envelope{Error: &EnvelopeError{Code: "stale", Message: "scope is stale"}}
	if err := RenderEnvelopeError(&out, env); err != nil {
		t.Fatalf("RenderEnvelopeError() error = %v", err)
	}
	if !strings.Contains(out.String(), "stale") || !strings.Contains(out.String(), "scope is stale") {
		t.Errorf("RenderEnvelopeError() = %q, want code and message", out.String())
	}
}

func TestChangedSinceBaselineLabel(t *testing.T) {
	cases := []struct {
		name string
		data map[string]any
		want string
	}{
		{"generation", map[string]any{"since_generation_id": "gen-9"}, "gen-9"},
		{"observed-at", map[string]any{"since_observed_at": "2026-01-01"}, "2026-01-01"},
		{"generation-wins", map[string]any{"since_generation_id": "gen-9", "since_observed_at": "2026-01-01"}, "gen-9"},
		{"unknown", map[string]any{}, "(unknown)"},
		{"nil", nil, "(unknown)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ChangedSinceBaselineLabel(tc.data); got != tc.want {
				t.Errorf("ChangedSinceBaselineLabel() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValueReaderGaps(t *testing.T) {
	// sliceValue accepts the []map spelling a Go caller may construct.
	got := sliceValue(map[string]any{"rows": []map[string]any{{"a": "b"}}}, "rows")
	if len(got) != 1 {
		t.Errorf("sliceValue([]map) len = %d, want 1", len(got))
	}
	if got := sliceValue(map[string]any{"rows": "nope"}, "rows"); got != nil {
		t.Errorf("sliceValue(string) = %v, want nil", got)
	}
	if got := sliceValue(nil, "rows"); got != nil {
		t.Errorf("sliceValue(nil) = %v, want nil", got)
	}
	// intValue accepts a Go int as well as the JSON float64.
	if got := intValue(map[string]any{"n": 7}, "n"); got != 7 {
		t.Errorf("intValue(int) = %d, want 7", got)
	}
	if got := intValue(map[string]any{"n": "seven"}, "n"); got != 0 {
		t.Errorf("intValue(string) = %d, want 0", got)
	}
	// boolValue true arm.
	if got := boolValue(map[string]any{"b": true}, "b"); !got {
		t.Error("boolValue(true) = false, want true")
	}
	// failureOf returns nil for a clean envelope.
	if err := failureOf(Envelope{}); err != nil {
		t.Errorf("failureOf(clean) = %v, want nil", err)
	}
	if err := failureOf(Envelope{Error: &EnvelopeError{Code: "stale"}}); err == nil {
		t.Error("failureOf(error) = nil, want failure")
	}
}

func TestFailureErrorText(t *testing.T) {
	f := &Failure{Message: "scope is stale", Code: 4}
	if f.Error() != "scope is stale" {
		t.Errorf("Failure.Error() = %q, want the message", f.Error())
	}
}

func TestBoolValueWrongType(t *testing.T) {
	if got := boolValue(map[string]any{"b": "yes"}, "b"); got {
		t.Error("boolValue(string) = true, want false")
	}
	if got := boolValue(nil, "b"); got {
		t.Error("boolValue(nil parent) = true, want false")
	}
}
