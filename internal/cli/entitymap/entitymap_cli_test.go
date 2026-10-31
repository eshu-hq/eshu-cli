// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package entitymap

import (
	"errors"
	"strings"
	"testing"
)

// statusErr is a test EnvelopePoster-side error carrying an HTTP status,
// exercising the apierr.StatusCode branch of the transport classifier.
type statusErr struct {
	status int
	msg    string
}

func (e statusErr) Error() string       { return e.msg }
func (e statusErr) HTTPStatusCode() int { return e.status }

// fakePoster serves a canned envelope or error and records what Fetch sent,
// so the tests pin the request shape without a network.
type fakePoster struct {
	envelope Envelope
	err      error
	path     string
	body     map[string]any
}

func (p *fakePoster) PostEnvelope(path string, body, result any) error {
	p.path = path
	if typed, ok := body.(map[string]any); ok {
		p.body = typed
	}
	if p.err != nil {
		return p.err
	}
	if envelope, ok := result.(*Envelope); ok {
		*envelope = p.envelope
	}
	return nil
}

func TestFailureErrorText(t *testing.T) {
	f := &Failure{Kind: FailureNoMatch, Code: "no_match", Message: "entity map selector did not match a supported entity"}
	if f.Error() != "entity map selector did not match a supported entity" {
		t.Errorf("Failure.Error() = %q, want the message", f.Error())
	}
}

func TestFetchPostsRequestShape(t *testing.T) {
	poster := &fakePoster{envelope: Envelope{Data: map[string]any{"status": "mapped"}}}
	opts := Options{
		From: "terraform/aws_lb.main", FromType: "service", Repo: "repo-a",
		Environment: "prod", Relationship: "DEPENDS_ON", Depth: 2, Limit: 10,
	}
	envelope, err := Fetch(poster, opts)
	if err != nil {
		t.Fatalf("Fetch() error = %v, want nil", err)
	}
	if poster.path != route {
		t.Errorf("Fetch() path = %q, want route %q", poster.path, route)
	}
	for key, want := range map[string]any{
		"from": "terraform/aws_lb.main", "from_type": "service", "repo_id": "repo-a",
		"environment": "prod", "relationship": "DEPENDS_ON", "depth": 2, "limit": 10,
	} {
		if poster.body[key] != want {
			t.Errorf("Fetch() body[%q] = %v, want %v", key, poster.body[key], want)
		}
	}
	if stringField(envelope.Data, "status") != "mapped" {
		t.Errorf("Fetch() status = %q, want mapped", stringField(envelope.Data, "status"))
	}
}

func TestFetchTransportError(t *testing.T) {
	poster := &fakePoster{err: errors.New("dial tcp: connection refused")}
	envelope, err := Fetch(poster, Options{From: "x"})
	if err == nil {
		t.Fatal("Fetch(failing client) = nil, want the client error")
	}
	if envelope.Error != nil || len(envelope.Data) != 0 {
		t.Errorf("Fetch(failing client) envelope = %+v, want zero (Resolve builds the synthetic one)", envelope)
	}
}

func TestResolveTransportError(t *testing.T) {
	fetchErr := errors.New("dial tcp: connection refused")
	envelope, failure := Resolve(Envelope{}, fetchErr)
	if failure == nil {
		t.Fatal("Resolve(transport fail) failure = nil, want failure")
	}
	if failure.Kind != FailureEnvelope {
		t.Errorf("failure.Kind = %d, want FailureEnvelope", failure.Kind)
	}
	if envelope.Error == nil || envelope.Error.Code != "backend_unavailable" {
		t.Errorf("synthetic envelope code = %+v, want backend_unavailable", envelope.Error)
	}
	if envelope.Error.Message != fetchErr.Error() {
		t.Errorf("synthetic envelope message = %q, want the transport text", envelope.Error.Message)
	}
	if failure.Message != fetchErr.Error() {
		t.Errorf("failure.Message = %q, want the transport text", failure.Message)
	}
}

func TestResolveEnvelopeError(t *testing.T) {
	envelope := Envelope{Error: &EnvelopeError{Code: "stale", Message: "scope is stale"}}
	_, failure := Resolve(envelope, nil)
	if failure == nil {
		t.Fatal("Resolve(envelope error) failure = nil, want failure")
	}
	if failure.Kind != FailureEnvelope || failure.Code != "stale" {
		t.Errorf("failure = %+v, want envelope kind with code stale", failure)
	}
}

func TestResolveFreshnessFailures(t *testing.T) {
	for _, state := range []string{"stale", "building"} {
		envelope := Envelope{Truth: map[string]any{"freshness": map[string]any{"state": state}}}
		_, failure := Resolve(envelope, nil)
		if failure == nil {
			t.Fatalf("Resolve(freshness=%s) failure = nil, want failure", state)
		}
		if failure.Kind != FailureFreshness || failure.Code != state {
			t.Errorf("Resolve(freshness=%s) = %+v, want freshness kind with code %s", state, failure, state)
		}
	}
	// A fresh index resolves no failure on its own.
	if _, failure := Resolve(Envelope{
		Data:  map[string]any{"status": "mapped"},
		Truth: map[string]any{"freshness": map[string]any{"state": "fresh"}},
	}, nil); failure != nil {
		t.Errorf("Resolve(fresh) failure = %+v, want nil", failure)
	}
}

func TestResolveCheckOrder(t *testing.T) {
	// Envelope error outranks a stale index: the API already said no.
	staleErr := Envelope{
		Error: &EnvelopeError{Code: "stale", Message: "scope is stale"},
		Truth: map[string]any{"freshness": map[string]any{"state": "stale"}},
	}
	if _, failure := Resolve(staleErr, nil); failure == nil || failure.Kind != FailureEnvelope {
		t.Errorf("Resolve(error+stale) = %+v, want the envelope failure first", failure)
	}
	// A stale index outranks an ambiguous selector: the map would answer
	// from truth the operator has not been told is behind.
	staleAmbiguous := Envelope{
		Data:  map[string]any{"status": "ambiguous"},
		Truth: map[string]any{"freshness": map[string]any{"state": "stale"}},
	}
	if _, failure := Resolve(staleAmbiguous, nil); failure == nil || failure.Kind != FailureFreshness {
		t.Errorf("Resolve(stale+ambiguous) = %+v, want the freshness failure first", failure)
	}
	// A transport error outranks everything the envelope carries.
	_, failure := Resolve(staleErr, errors.New("boom"))
	if failure == nil || failure.Kind != FailureEnvelope || failure.Code != "api_error" {
		t.Errorf("Resolve(transport+error) = %+v, want the transport failure first", failure)
	}
}

func TestResolveResolutionStatus(t *testing.T) {
	ambiguous := Envelope{Data: map[string]any{"status": "ambiguous"}}
	if _, failure := Resolve(ambiguous, nil); failure == nil || failure.Kind != FailureAmbiguous {
		t.Errorf("Resolve(ambiguous) = %+v, want FailureAmbiguous", failure)
	}
	noMatch := Envelope{Data: map[string]any{"status": "no_match"}}
	_, failure := Resolve(noMatch, nil)
	if failure == nil || failure.Kind != FailureNoMatch || failure.Code != "no_match" {
		t.Errorf("Resolve(no_match) = %+v, want FailureNoMatch", failure)
	}
	if !strings.Contains(failure.Message, "did not match") {
		t.Errorf("Resolve(no_match) message = %q, want the no-match sentence", failure.Message)
	}
	mapped := Envelope{Data: map[string]any{"status": "mapped"}}
	if _, failure := Resolve(mapped, nil); failure != nil {
		t.Errorf("Resolve(mapped) failure = %+v, want nil", failure)
	}
	// An unknown status is success: the CLI renders what the API returned.
	unknown := Envelope{Data: map[string]any{"status": "weird"}}
	if _, failure := Resolve(unknown, nil); failure != nil {
		t.Errorf("Resolve(weird) failure = %+v, want nil", failure)
	}
}

func TestEnvelopeFailureFallbacks(t *testing.T) {
	if failure := envelopeFailure(nil); failure != nil {
		t.Errorf("envelopeFailure(nil) = %+v, want nil", failure)
	}
	if failure := envelopeFailure(&EnvelopeError{Code: "stale", Message: "scope is stale"}); failure.Message != "scope is stale" {
		t.Errorf("envelopeFailure() message = %q, want the message", failure.Message)
	}
	// No message falls back to the code, so the operator never sees an
	// empty failure line.
	if failure := envelopeFailure(&EnvelopeError{Code: "ambiguous"}); failure.Message != "ambiguous" {
		t.Errorf("envelopeFailure() message = %q, want the code fallback", failure.Message)
	}
	if failure := envelopeFailure(&EnvelopeError{}); failure.Message != "entity map failed" {
		t.Errorf("envelopeFailure() message = %q, want the fixed sentence", failure.Message)
	}
	if failure := envelopeFailure(&EnvelopeError{Code: "  ", Message: "   "}); failure.Message != "entity map failed" {
		t.Errorf("envelopeFailure() message = %q, want the fixed sentence for blanks", failure.Message)
	}
}

func TestFreshnessState(t *testing.T) {
	fresh := Envelope{Truth: map[string]any{"freshness": map[string]any{"state": "fresh"}}}
	if got := FreshnessState(fresh); got != "fresh" {
		t.Errorf("FreshnessState() = %q, want fresh", got)
	}
	for name, envelope := range map[string]Envelope{
		"no truth":      {},
		"wrong type":    {Truth: map[string]any{"freshness": "fresh"}},
		"missing state": {Truth: map[string]any{"freshness": map[string]any{}}},
	} {
		t.Run(name, func(t *testing.T) {
			if got := FreshnessState(envelope); got != "" {
				t.Errorf("FreshnessState() = %q, want empty", got)
			}
		})
	}
}

func TestErrorCodeFromTransport(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"conflict", statusErr{status: 409, msg: "conflict"}, "ambiguous"},
		{"refused", errors.New("dial tcp: connection refused"), "backend_unavailable"},
		{"failed", errors.New("request failed after retries"), "backend_unavailable"},
		{"bad-request", statusErr{status: 400, msg: "bad"}, "invalid_argument"},
		{"not-found", statusErr{status: 404, msg: "missing"}, "not_found"},
		{"not-implemented", statusErr{status: 501, msg: "nope"}, "unsupported_capability"},
		{"unavailable", statusErr{status: 503, msg: "down"}, "backend_unavailable"},
		{"teapot", statusErr{status: 418, msg: "tea"}, "api_error"},
		{"plain", errors.New("boom"), "api_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ErrorCodeFromTransport(tc.err); got != tc.want {
				t.Errorf("ErrorCodeFromTransport(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

// TestTransportClassifierPrecedence pins the two orderings the classifier
// depends on: a 409 wins over a refused message (only the map uses 409, and
// it means the selector was ambiguous), while a refused/failed message wins
// over any other status (a broken connection, not a bad selector).
func TestTransportClassifierPrecedence(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"conflict-beats-refused", statusErr{status: 409, msg: "connection refused"}, "ambiguous"},
		{"refused-beats-400", statusErr{status: 400, msg: "dial: connection refused"}, "backend_unavailable"},
		{"failed-beats-404", statusErr{status: 404, msg: "request failed: reset"}, "backend_unavailable"},
		{"refused-beats-418", statusErr{status: 418, msg: "connection refused"}, "backend_unavailable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ErrorCodeFromTransport(tc.err); got != tc.want {
				t.Errorf("ErrorCodeFromTransport(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

func TestMapField(t *testing.T) {
	parent := map[string]any{"freshness": map[string]any{"state": "fresh"}}
	if got := mapField(parent, "freshness"); got["state"] != "fresh" {
		t.Errorf("mapField() = %v, want the nested object", got)
	}
	if got := mapField(parent, "missing"); got != nil {
		t.Errorf("mapField(missing) = %v, want nil", got)
	}
	if got := mapField(map[string]any{"freshness": "fresh"}, "freshness"); got != nil {
		t.Errorf("mapField(string) = %v, want nil", got)
	}
	if got := mapField(nil, "freshness"); got != nil {
		t.Errorf("mapField(nil parent) = %v, want nil", got)
	}
}

func TestSliceField(t *testing.T) {
	anyRows := map[string]any{"rows": []any{map[string]any{"a": "b"}}}
	if got := sliceField(anyRows, "rows"); len(got) != 1 {
		t.Errorf("sliceField([]any) len = %d, want 1", len(got))
	}
	// A Go-built envelope spells rows as []map; it renders the same way.
	mapRows := map[string]any{"rows": []map[string]any{{"a": "b"}, {"c": "d"}}}
	if got := sliceField(mapRows, "rows"); len(got) != 2 {
		t.Errorf("sliceField([]map) len = %d, want 2", len(got))
	}
	if got := sliceField(map[string]any{"rows": "nope"}, "rows"); got != nil {
		t.Errorf("sliceField(string) = %v, want nil", got)
	}
	if got := sliceField(nil, "rows"); got != nil {
		t.Errorf("sliceField(nil parent) = %v, want nil", got)
	}
}

func TestStringField(t *testing.T) {
	if got := stringField(map[string]any{"id": "  svc-a "}, "id"); got != "svc-a" {
		t.Errorf("stringField() = %q, want trimmed svc-a", got)
	}
	if got := stringField(map[string]any{"id": 7}, "id"); got != "" {
		t.Errorf("stringField(int) = %q, want empty", got)
	}
	if got := stringField(map[string]any{}, "id"); got != "" {
		t.Errorf("stringField(missing) = %q, want empty", got)
	}
	if got := stringField(nil, "id"); got != "" {
		t.Errorf("stringField(nil parent) = %q, want empty", got)
	}
}

func TestIntField(t *testing.T) {
	if got := intField(map[string]any{"n": 7}, "n"); got != 7 {
		t.Errorf("intField(int) = %d, want 7", got)
	}
	if got := intField(map[string]any{"n": float64(4)}, "n"); got != 4 {
		t.Errorf("intField(float64) = %d, want 4", got)
	}
	if got := intField(map[string]any{"n": "seven"}, "n"); got != 0 {
		t.Errorf("intField(string) = %d, want 0", got)
	}
	if got := intField(nil, "n"); got != 0 {
		t.Errorf("intField(nil parent) = %d, want 0", got)
	}
}

func TestStringList(t *testing.T) {
	if got := stringList([]string{"a", "b"}); len(got) != 2 {
		t.Errorf("stringList([]string) = %v, want both entries", got)
	}
	// Blanks and non-strings drop out; survivors trim.
	got := stringList([]any{" a ", "  ", 7, nil, "b"})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("stringList([]any) = %q, want [a b]", got)
	}
	if got := stringList("nope"); got != nil {
		t.Errorf("stringList(string) = %v, want nil", got)
	}
	if got := stringList(nil); got != nil {
		t.Errorf("stringList(nil) = %v, want nil", got)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "  ", "svc-a", "other"); got != "svc-a" {
		t.Errorf("firstNonEmpty() = %q, want svc-a", got)
	}
	if got := firstNonEmpty("", "   "); got != "" {
		t.Errorf("firstNonEmpty(blanks) = %q, want empty", got)
	}
	if got := firstNonEmpty(); got != "" {
		t.Errorf("firstNonEmpty() = %q, want empty", got)
	}
}
