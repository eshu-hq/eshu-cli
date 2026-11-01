// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/eshu-hq/eshu-cli/internal/cli/trace"
)

func stubTraceFetch(t *testing.T, fn func(*APIClient, string, trace.ServiceQuery) (trace.ServiceEnvelope, error)) {
	t.Helper()
	orig := traceFetchServiceStory
	traceFetchServiceStory = fn
	t.Cleanup(func() { traceFetchServiceStory = orig })
}

// runTraceServiceCommand runs a fresh `trace service` command and returns its
// stdout and error.
func runTraceServiceCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := &cobra.Command{Use: "service <name>", Args: cobra.ExactArgs(1), RunE: runTraceService}
	addTraceServiceFlags(cmd)
	addRemoteFlags(cmd)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestTraceServiceExitCodes(t *testing.T) {
	healthy := trace.ServiceEnvelope{Data: map[string]any{"service_name": "checkout"}}
	cases := []struct {
		name     string
		args     []string
		envelope trace.ServiceEnvelope
		fetchErr error
		wantCode int // 0 means success
	}{
		{name: "blank name is a usage error", args: []string{"   "}, wantCode: 2},
		{name: "healthy trace succeeds", args: []string{"checkout"}, envelope: healthy},
		{
			name: "api error maps through the exit table", args: []string{"checkout"}, wantCode: 6,
			envelope: trace.ServiceEnvelope{Error: &trace.ServiceError{Code: "unsupported_capability", Message: "no"}},
		},
		{
			name: "ambiguous service exits 3", args: []string{"checkout"}, wantCode: 3,
			envelope: trace.ServiceEnvelope{Error: &trace.ServiceError{Code: "ambiguous", Message: "two matches"}},
		},
		{
			name: "missing data is not found", args: []string{"checkout"}, wantCode: 2,
			envelope: trace.ServiceEnvelope{},
		},
		{
			name: "stale truth exits 4", args: []string{"checkout"}, wantCode: 4,
			envelope: trace.ServiceEnvelope{
				Data:  map[string]any{"service_name": "checkout"},
				Truth: map[string]any{"freshness": map[string]any{"state": "stale"}},
			},
		},
		{
			name: "partial trace exits 5", args: []string{"checkout"}, wantCode: 5,
			envelope: trace.ServiceEnvelope{Data: map[string]any{
				"code_to_runtime_trace": map[string]any{"status": "partial"},
			}},
		},
		{
			name: "connection refused is backend unavailable", args: []string{"checkout"}, wantCode: 1,
			fetchErr: errors.New("dial tcp: connection refused"),
		},
		{
			name: "not found status exits 2", args: []string{"checkout"}, wantCode: 2,
			fetchErr: &apiHTTPError{StatusCode: 404, Body: "nope"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stubTraceFetch(t, func(*APIClient, string, trace.ServiceQuery) (trace.ServiceEnvelope, error) {
				return tc.envelope, tc.fetchErr
			})
			_, err := runTraceServiceCommand(t, tc.args...)
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

func TestTraceServicePassesSelectorsAndWritesJSON(t *testing.T) {
	var gotSelector string
	var gotQuery trace.ServiceQuery
	stubTraceFetch(t, func(_ *APIClient, selector string, query trace.ServiceQuery) (trace.ServiceEnvelope, error) {
		gotSelector, gotQuery = selector, query
		return trace.ServiceEnvelope{Data: map[string]any{"service_name": "checkout"}}, nil
	})
	out, err := runTraceServiceCommand(t, "  checkout ", "--json", "--repo", " r ", "--env", " prod ", "--service-id", " svc-1 ")
	if err != nil {
		t.Fatalf("trace service: %v", err)
	}
	if gotSelector != "checkout" || gotQuery.Repo != "r" || gotQuery.Environment != "prod" || gotQuery.ServiceID != "svc-1" {
		t.Errorf("selector=%q query=%+v, want trimmed values", gotSelector, gotQuery)
	}
	var decoded trace.ServiceEnvelope
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("--json output is not an envelope: %v\n%s", err, out)
	}
	if decoded.Data["service_name"] != "checkout" {
		t.Errorf("decoded data = %v, want the service name back", decoded.Data)
	}
}

func TestTraceServiceJSONStillReturnsTheExitError(t *testing.T) {
	stubTraceFetch(t, func(*APIClient, string, trace.ServiceQuery) (trace.ServiceEnvelope, error) {
		return trace.ServiceEnvelope{Error: &trace.ServiceError{Code: "stale", Message: "behind"}}, nil
	})
	out, err := runTraceServiceCommand(t, "checkout", "--json")
	if got := exitCodeOf(t, err); got != 4 {
		t.Errorf("exit code = %d, want 4", got)
	}
	if !strings.Contains(out, `"stale"`) {
		t.Errorf("--json output must still carry the envelope error, got %q", out)
	}
}

func TestTraceServiceRequestFromCommandRejectsFlaglessCommand(t *testing.T) {
	if _, _, err := traceServiceRequestFromCommand(&cobra.Command{Use: "bare"}); err == nil {
		t.Error("a command without the trace flags must return the flag lookup error")
	}
}

func TestTraceEnvelopeErrorFallbacks(t *testing.T) {
	if err := traceEnvelopeError(nil); err != nil {
		t.Errorf("nil envelope error = %v, want nil", err)
	}
	cases := []struct {
		in       trace.ServiceError
		wantMsg  string
		wantCode int
	}{
		{trace.ServiceError{Code: "not_found", Message: " gone "}, "gone", 2},
		{trace.ServiceError{Code: "index_building"}, "index_building", 4},
		{trace.ServiceError{}, "service trace failed", 1},
	}
	for _, tc := range cases {
		in := tc.in
		var exit commandExitError
		if err := traceEnvelopeError(&in); !errors.As(err, &exit) || exit.message != tc.wantMsg || exit.code != tc.wantCode {
			t.Errorf("traceEnvelopeError(%+v) = %#v, want message %q code %d", tc.in, err, tc.wantMsg, tc.wantCode)
		}
	}
}

func TestTraceExitCodeTable(t *testing.T) {
	want := map[string]int{
		"ambiguous": 3, "index_building": 4, "stale": 4, "capability_degraded": 5, "partial": 5,
		"unsupported_capability": 6, "invalid_argument": 2, "not_found": 2, "scope_not_found": 2,
		"": 1, "something_else": 1, " ambiguous ": 3,
	}
	for code, exit := range want {
		if got := traceExitCode(code); got != exit {
			t.Errorf("traceExitCode(%q) = %d, want %d", code, got, exit)
		}
	}
}

func TestTraceErrorCodeFromTransport(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{nil, "api_error"},
		{errors.New("dial tcp 127.0.0.1:1: connect: connection refused"), "backend_unavailable"},
		{errors.New("request failed: EOF"), "backend_unavailable"},
		{&apiHTTPError{StatusCode: 400}, "invalid_argument"},
		{&apiHTTPError{StatusCode: 404}, "not_found"},
		{&apiHTTPError{StatusCode: 501}, "unsupported_capability"},
		{&apiHTTPError{StatusCode: 503}, "backend_unavailable"},
		{&apiHTTPError{StatusCode: 418}, "api_error"},
		{fmt.Errorf("wrapped: %w", &apiHTTPError{StatusCode: 404}), "not_found"},
		{errors.New("something odd"), "api_error"},
	}
	for _, tc := range cases {
		if got := traceErrorCodeFromTransport(tc.err); got != tc.want {
			t.Errorf("traceErrorCodeFromTransport(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}
