// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// serviceReportFixture is a canned compose response data block: one supported
// section with a summary plus a next call and an investigation, exercising
// every RenderReport projection branch.
const serviceReportFixture = `{"schema":"service-report/v1","subject":{"service_name":"checkout","service_id":"svc-1"},"supported":true,"partial":false,"truth_class":"exact","sections":[{"kind":"identity","title":"Identity","status":"supported","answer":{"summary":"checkout serves prod"}}],"recommended_next_calls":[{"tool":"get_service_story","reason":"refresh"}],"suggested_investigations":[{"basis":"missing_evidence","reason":"gap in story","next_call":{"route":"/api/v0/x"},"expected_truth_class":"derived"}]}`

func newTestServiceReportCommand(t *testing.T) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	cmd := newServiceReportCommand()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader(""))
	return cmd, &stdout, &stderr
}

// TestServiceReportComposesViaEndpoint pins the Tier-2 compose flow: the CLI
// reads the captured dossier, posts it to the S4 endpoint, and prints the
// composed report as JSON.
func TestServiceReportComposesViaEndpoint(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v0/service/report/compose" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		gotBody = raw
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":` + serviceReportFixture + `,"truth":null,"error":null}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dossierPath := dir + "/story.json"
	if err := writeTestFile(t, dossierPath, `{"data":{"service":{"name":"checkout"}},"truth":{"level":"exact","freshness":{"state":"fresh"}}}`); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ESHU_SERVICE_URL", srv.URL)
	cmd, stdout, _ := newTestServiceReportCommand(t)
	setCmdFlags(t, cmd, map[string]string{
		"from": dossierPath,
		"json": "true",
	})
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("service-report RunE error = %v", err)
	}
	var input struct {
		Dossier map[string]any `json:"dossier"`
		Truth   map[string]any `json:"truth"`
	}
	if err := json.Unmarshal(gotBody, &input); err != nil {
		t.Fatalf("compose request is not JSON: %v", err)
	}
	if input.Dossier["service"] == nil {
		t.Fatalf("compose dossier = %v, want captured service story", input.Dossier)
	}
	if input.Truth["level"] != "exact" {
		t.Fatalf("compose truth = %v, want captured truth", input.Truth)
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, []byte(serviceReportFixture), "", "  "); err != nil {
		t.Fatal(err)
	}
	if want := indented.String() + "\n"; stdout.String() != want {
		t.Fatalf("service-report stdout = %q, want %q", stdout.String(), want)
	}
}

// TestServiceReportRendersText pins the text projection against the composed
// wire shape: subject label, status line, sections, next calls, and
// investigations.
func TestServiceReportRendersText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":` + serviceReportFixture + `,"truth":null,"error":null}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dossierPath := dir + "/story.json"
	if err := writeTestFile(t, dossierPath, `{"service":{"name":"checkout"}}`); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ESHU_SERVICE_URL", srv.URL)
	cmd, stdout, _ := newTestServiceReportCommand(t)
	setCmdFlags(t, cmd, map[string]string{"from": dossierPath})
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("service-report RunE error = %v", err)
	}
	want := "Service intelligence report: checkout (svc-1)\n" +
		"  supported=true partial=false truth_class=exact\n" +
		"\n" +
		"[SUPPORTED] Identity\n" +
		"  checkout serves prod\n" +
		"\nRecommended next calls:\n" +
		"  - get_service_story (refresh)\n" +
		"\nSuggested investigations:\n" +
		"  - [missing_evidence] gap in story -> /api/v0/x (expect derived)\n"
	if stdout.String() != want {
		t.Fatalf("service-report text = %q, want %q", stdout.String(), want)
	}
}

// TestServiceReportRequiresInput pins the empty-stdin gate.
func TestServiceReportRequiresInput(t *testing.T) {
	cmd, _, _ := newTestServiceReportCommand(t)
	if err := cmd.RunE(cmd, []string{}); err == nil ||
		!strings.Contains(err.Error(), "no service-story response provided") {
		t.Fatalf("service-report empty stdin error = %v, want input gate", err)
	}
}

// TestServiceReportCommandsRegistered pins the Tier-2 wiring on the shared tree.
func TestServiceReportCommandsRegistered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"service-report"})
	if err != nil {
		t.Fatalf("find service-report: %v", err)
	}
	if cmd == nil || cmd.Name() != "service-report" {
		t.Fatal("service-report not registered")
	}
}
