// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// writeTestFile stores content at path with owner-only permissions, mirroring
// the CLI's bundle-artifact write posture.
func writeTestFile(t *testing.T, path, content string) error {
	t.Helper()
	//nolint:gosec // test fixture under t.TempDir, never a request param
	return os.WriteFile(path, []byte(content), 0o600)
}

// newTestReportCommand builds a fresh report subcommand for a test run.
// Fresh instances (not the shared rootCmd tree) keep flag state isolated
// between tests; registration on the shared tree is pinned separately by
// TestReportCommandsRegistered.
func newTestReportCommand(t *testing.T, sub string) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var cmd *cobra.Command
	switch sub {
	case "capture":
		cmd = newReportCaptureCommand()
	case "validate":
		cmd = newReportValidateCommand()
	default:
		t.Fatalf("unknown report subcommand %q", sub)
	}
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader(""))
	return cmd, &stdout, &stderr
}

// TestReportCommandsRegistered pins the Tier-2 report wiring: the digest
// parent plus the thin capture/validate verbs resolve on the shared tree.
func TestReportCommandsRegistered(t *testing.T) {
	for _, args := range [][]string{{"report"}, {"report", "capture"}, {"report", "validate"}} {
		t.Run(strings.Join(args, "-"), func(t *testing.T) {
			cmd, _, err := rootCmd.Find(args)
			if err != nil {
				t.Fatalf("find %v: %v", args, err)
			}
			if cmd == nil || cmd.Name() != args[len(args)-1] {
				t.Fatalf("command %v not registered", args)
			}
		})
	}
}

func setCmdFlags(t *testing.T, cmd *cobra.Command, flags map[string]string) {
	t.Helper()
	for name, value := range flags {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("set --%s=%q: %v", name, value, err)
		}
	}
}

// TestReportCapturePostsEnvelopeAndPrintsBundle pins the Tier-2 capture flow:
// the CLI fetches the query envelope itself, posts the capture input to the
// S4 endpoint, and prints the returned bundle bytes indented with a trailing
// newline, like the offline command's canonical encoding.
func TestReportCapturePostsEnvelopeAndPrintsBundle(t *testing.T) {
	const envelopeBody = `{"data":{"answer":"ok"},"truth":{"level":"exact","freshness":{"state":"fresh"}},"error":null}`
	const bundleBody = `{"schema":"wrong_answer_report.v1","query":{"target":"/api/v0/q"}}`
	var captureBody []byte
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v0/q"):
			gotQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(envelopeBody))
		case r.Method == "POST" && r.URL.Path == "/api/v0/report/bundle/capture":
			raw, _ := io.ReadAll(r.Body)
			captureBody = raw
			_, _ = w.Write([]byte(`{"data":` + bundleBody + `,"truth":{"level":"exact","freshness":{"state":"fresh"}},"error":null}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	cmd, stdout, _ := newTestReportCommand(t, "capture")
	setCmdFlags(t, cmd, map[string]string{
		"service-url": srv.URL,
		"endpoint":    "/api/v0/q",
		"params":      `{"x":"1"}`,
		"note":        "expected a different answer",
	})
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("capture RunE error = %v", err)
	}
	if want := "x=1"; gotQuery != want {
		t.Fatalf("capture GET query = %q, want %q", gotQuery, want)
	}
	var input struct {
		Surface      string         `json:"surface"`
		Target       string         `json:"target"`
		Method       string         `json:"method"`
		Params       map[string]any `json:"params"`
		ReporterNote string         `json:"reporter_note"`
		Envelope     map[string]any `json:"envelope"`
	}
	if err := json.Unmarshal(captureBody, &input); err != nil {
		t.Fatalf("capture request is not JSON: %v", err)
	}
	if input.Surface != "api" || input.Target != "/api/v0/q" || input.Method != "GET" {
		t.Fatalf("capture input = %+v, want surface=api target=/api/v0/q method=GET", input)
	}
	if input.Params["x"] != "1" {
		t.Fatalf("capture params = %v, want x=1", input.Params)
	}
	if input.ReporterNote != "expected a different answer" {
		t.Fatalf("capture note = %q", input.ReporterNote)
	}
	if input.Envelope["data"] == nil {
		t.Fatalf("capture envelope missing data: %v", input.Envelope)
	}
	wantOut := "{\n  \"schema\": \"wrong_answer_report.v1\",\n  \"query\": {\n    \"target\": \"/api/v0/q\"\n  }\n}\n"
	if stdout.String() != wantOut {
		t.Fatalf("capture stdout = %q, want %q", stdout.String(), wantOut)
	}
}

// TestReportCaptureExplicitParamWins pins the merge rule: an explicit --params
// entry replaces a same-named endpoint query parameter.
func TestReportCaptureExplicitParamWins(t *testing.T) {
	const envelopeBody = `{"data":{"answer":"ok"},"truth":null,"error":null}`
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			gotQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(envelopeBody))
			return
		}
		_, _ = w.Write([]byte(`{"data":{},"truth":null,"error":null}`))
	}))
	defer srv.Close()

	cmd, _, _ := newTestReportCommand(t, "capture")
	setCmdFlags(t, cmd, map[string]string{
		"service-url": srv.URL,
		"endpoint":    "/api/v0/q?a=1",
		"params":      `{"a":"2","b":"3"}`,
	})
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("capture RunE error = %v", err)
	}
	if want := "a=2&b=3"; gotQuery != want {
		t.Fatalf("capture GET query = %q, want %q (explicit --params wins)", gotQuery, want)
	}
}

// TestReportCapturePostPreservesEndpointQuery pins the POST flow: the query
// goes to the full endpoint (query string included) with the --params object
// as the body, and the S4 input records the reporter surface.
func TestReportCapturePostPreservesEndpointQuery(t *testing.T) {
	var gotPath, gotQuery string
	var gotBody []byte
	var captureInput struct {
		Surface string         `json:"surface"`
		Target  string         `json:"target"`
		Method  string         `json:"method"`
		Params  map[string]any `json:"params"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/api/v0/q") {
			gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
			raw, _ := io.ReadAll(r.Body)
			gotBody = raw
			_, _ = w.Write([]byte(`{"data":{"answer":"ok"},"truth":null,"error":null}`))
			return
		}
		if r.URL.Path == "/api/v0/report/bundle/capture" {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &captureInput)
			_, _ = w.Write([]byte(`{"data":{"bundle":"ok"},"truth":null,"error":null}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	cmd, stdout, stderr := newTestReportCommand(t, "capture")
	setCmdFlags(t, cmd, map[string]string{
		"service-url":      srv.URL,
		"endpoint":         "/api/v0/q?x=1",
		"method":           "post",
		"params":           `{"y":"2"}`,
		"tool":             "find_tool",
		"include-payloads": "true",
		"out":              "",
	})
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("capture RunE error = %v", err)
	}
	if gotPath != "/api/v0/q" || gotQuery != "x=1" {
		t.Fatalf("POST went to %s?%s, want /api/v0/q?x=1", gotPath, gotQuery)
	}
	var fetchParams map[string]any
	if err := json.Unmarshal(gotBody, &fetchParams); err != nil || fetchParams["y"] != "2" {
		t.Fatalf("POST body = %s, want params object", string(gotBody))
	}
	if captureInput.Surface != "mcp" || captureInput.Target != "find_tool" || captureInput.Method != "POST" {
		t.Fatalf("capture input = %+v, want mcp/find_tool/POST", captureInput)
	}
	if !strings.Contains(stderr.String(), "PRIVATE TRIAGE ONLY") {
		t.Fatalf("capture stderr = %q, want payloads warning", stderr.String())
	}
	if !strings.Contains(stdout.String(), `"bundle": "ok"`) {
		t.Fatalf("capture stdout = %q, want bundle", stdout.String())
	}
}

// TestReportCaptureWritesOutFile pins --out: the indented bundle lands on
// disk with owner-only permissions.
func TestReportCaptureWritesOutFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/api/v0/q") {
			_, _ = w.Write([]byte(`{"data":{},"truth":null,"error":null}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"bundle":"ok"},"truth":null,"error":null}`))
	}))
	defer srv.Close()

	outPath := t.TempDir() + "/bundle.json"
	cmd, _, _ := newTestReportCommand(t, "capture")
	setCmdFlags(t, cmd, map[string]string{
		"service-url": srv.URL,
		"endpoint":    "/api/v0/q",
		"out":         outPath,
	})
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("capture RunE error = %v", err)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read out file: %v", err)
	}
	if want := "{\n  \"bundle\": \"ok\"\n}\n"; string(raw) != want {
		t.Fatalf("out file = %q, want %q", string(raw), want)
	}
}

// TestReportValidateReadsStdin pins the stdin bundle path.
func TestReportValidateReadsStdin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"valid":true},"truth":null,"error":null}`))
	}))
	defer srv.Close()
	t.Setenv("ESHU_SERVICE_URL", srv.URL)

	cmd, stdout, _ := newTestReportCommand(t, "validate")
	cmd.SetIn(strings.NewReader(`{"schema":"wrong_answer_report.v1"}`))
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("validate RunE error = %v, want nil", err)
	}
	if stdout.String() != "report bundle validation: passed\n" {
		t.Fatalf("validate stdout = %q", stdout.String())
	}
}

// TestReportValidateUnreadableFile pins the file-read gate.
func TestReportValidateUnreadableFile(t *testing.T) {
	cmd, _, _ := newTestReportCommand(t, "validate")
	setCmdFlags(t, cmd, map[string]string{"from": "/nonexistent/bundle.json"})
	err := cmd.RunE(cmd, []string{})
	if err == nil || !strings.Contains(err.Error(), "read report bundle") {
		t.Fatalf("validate RunE error = %v, want read report bundle", err)
	}
}

// TestReportCaptureRefusesCredentialTarget pins the fail-closed credential
// gate: a userinfo-bearing endpoint is refused before any request goes out.
func TestReportCaptureRefusesCredentialTarget(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cmd, _, _ := newTestReportCommand(t, "capture")
	setCmdFlags(t, cmd, map[string]string{
		"service-url": srv.URL,
		"endpoint":    "https://svc:C4N4RY@example.com/api",
	})
	err := cmd.RunE(cmd, []string{})
	if err == nil {
		t.Fatal("capture with credential endpoint returned nil, want refusal")
	}
	if !strings.Contains(err.Error(), "--endpoint") || !strings.Contains(err.Error(), "credential") {
		t.Fatalf("capture refusal = %q, want --endpoint credential refusal", err.Error())
	}
	if coded, ok := err.(commandExitError); !ok || coded.ExitCode() != 2 {
		t.Fatalf("capture refusal exit = %v, want code 2", err)
	}
	if hit {
		t.Fatal("credential endpoint reached the server, want no request")
	}
}

// TestReportCaptureRequiresEndpoint pins the --endpoint required gate.
func TestReportCaptureRequiresEndpoint(t *testing.T) {
	cmd, _, _ := newTestReportCommand(t, "capture")
	err := cmd.RunE(cmd, []string{})
	if err == nil || !strings.Contains(err.Error(), "--endpoint is required") {
		t.Fatalf("capture without endpoint error = %v, want --endpoint is required", err)
	}
}

// TestReportValidatePassAndFail pins the validate mapping: the S4 verdict
// drives the printed line and the exit, with the server's message as the
// failure text.
func TestReportValidatePassAndFail(t *testing.T) {
	for _, tc := range []struct {
		name    string
		verdict string
		wantOut string
		wantErr string
	}{
		{"pass", `{"valid":true}`, "report bundle validation: passed\n", ""},
		{"fail", `{"valid":false,"message":"bundle fails the share-safe gate"}`, "report bundle validation: failed\n", "bundle fails the share-safe gate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				gotBody = raw
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":` + tc.verdict + `,"truth":null,"error":null}`))
			}))
			defer srv.Close()

			dir := t.TempDir()
			bundlePath := dir + "/bundle.json"
			if err := writeTestFile(t, bundlePath, `{"schema":"wrong_answer_report.v1"}`); err != nil {
				t.Fatal(err)
			}
			t.Setenv("ESHU_SERVICE_URL", srv.URL)
			cmd, stdout, _ := newTestReportCommand(t, "validate")
			setCmdFlags(t, cmd, map[string]string{"from": bundlePath})
			err := cmd.RunE(cmd, []string{})
			if tc.wantErr == "" && err != nil {
				t.Fatalf("validate RunE error = %v, want nil", err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("validate RunE error = %v, want %q", err, tc.wantErr)
			}
			if stdout.String() != tc.wantOut {
				t.Fatalf("validate stdout = %q, want %q", stdout.String(), tc.wantOut)
			}
			var input struct {
				Bundle        map[string]any `json:"bundle"`
				RequirePublic bool           `json:"require_public"`
			}
			if err := json.Unmarshal(gotBody, &input); err != nil {
				t.Fatalf("validate request is not JSON: %v", err)
			}
			if input.Bundle["schema"] != "wrong_answer_report.v1" {
				t.Fatalf("validate bundle = %v", input.Bundle)
			}
		})
	}
}
