// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// scorecardPassFixture is a canned score verdict envelope data block.
const scorecardPassFixture = `{"version":"answer-quality-scorecard/v1","run_id":"run-1","pass":true,"score":9,"criteria":[{"name":"usefulness","status":"pass","detail":"useful"}],"follow_up_issues":[]}`

// scorecardFailFixture is a failing verdict with one failed criterion and one
// follow-up issue, exercising the failure summary and issue rendering.
const scorecardFailFixture = `{"version":"answer-quality-scorecard/v1","run_id":"","pass":false,"score":3,"criteria":[{"name":"usefulness","status":"fail","detail":"not useful"},{"name":"honesty","status":"pass","detail":"honest"}],"follow_up_issues":[{"title":"add citations","labels":["content","followup"]}]}`

func newTestScorecardCommand(t *testing.T) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	cmd := newAnswerQualityScorecardCommand()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader(""))
	return cmd, &stdout, &stderr
}

func scorecardStubServer(t *testing.T, verdict string, gotBody *[]byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v0/answer-quality/score" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		*gotBody = raw
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":` + verdict + `,"truth":null,"error":null}`))
	}))
}

// TestAnswerQualityScorecardPassesVerdictThrough pins the Tier-2 score flow:
// raw evidence bytes go to the S4 endpoint untouched and the verdict drives
// the exit.
func TestAnswerQualityScorecardPassesVerdictThrough(t *testing.T) {
	var gotBody []byte
	srv := scorecardStubServer(t, scorecardPassFixture, &gotBody)
	defer srv.Close()
	t.Setenv("ESHU_SERVICE_URL", srv.URL)

	dir := t.TempDir()
	evidencePath := dir + "/evidence.json"
	const evidence = `{"version":"answer-quality/v1","run_id":"run-1","prompts":[]}`
	if err := writeTestFile(t, evidencePath, evidence); err != nil {
		t.Fatal(err)
	}
	cmd, stdout, _ := newTestScorecardCommand(t)
	setCmdFlags(t, cmd, map[string]string{"from": evidencePath})
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("scorecard RunE error = %v, want nil", err)
	}
	if strings.TrimSpace(string(gotBody)) != evidence {
		t.Fatalf("score request body = %q, want evidence bytes untouched", string(gotBody))
	}
	if want := "Answer-quality scorecard PASSED\n  run   : run-1\n  score : 9\n" +
		"--------------------------------------------\n" +
		"  [ok] usefulness: useful\n"; stdout.String() != want {
		t.Fatalf("scorecard stdout = %q, want %q", stdout.String(), want)
	}
}

// TestAnswerQualityScorecardFailureSummary pins the failing verdict mapping:
// the FAILED header, failed-criterion summary in the returned error, and the
// follow-up issue rendering.
func TestAnswerQualityScorecardFailureSummary(t *testing.T) {
	var gotBody []byte
	srv := scorecardStubServer(t, scorecardFailFixture, &gotBody)
	defer srv.Close()
	t.Setenv("ESHU_SERVICE_URL", srv.URL)

	cmd, stdout, _ := newTestScorecardCommand(t)
	cmd.SetIn(strings.NewReader(`{"version":"answer-quality/v1","prompts":[]}`))
	if err := cmd.RunE(cmd, []string{}); err == nil ||
		!strings.Contains(err.Error(), "answer-quality scorecard FAILED: usefulness: not useful") {
		t.Fatalf("scorecard RunE error = %v, want FAILED summary", err)
	}
	want := "Answer-quality scorecard FAILED\n  run   : <repo>\n  score : 3\n" +
		"--------------------------------------------\n" +
		"  [!!] usefulness: not useful\n" +
		"  [ok] honesty: honest\n" +
		"follow-up issues:\n" +
		"  - add citations [content, followup]\n"
	if stdout.String() != want {
		t.Fatalf("scorecard stdout = %q, want %q", stdout.String(), want)
	}
}

// TestAnswerQualityScorecardCommandsRegistered pins the Tier-2 wiring.
func TestAnswerQualityScorecardCommandsRegistered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"answer-quality-scorecard"})
	if err != nil {
		t.Fatalf("find answer-quality-scorecard: %v", err)
	}
	if cmd == nil || cmd.Name() != "answer-quality-scorecard" {
		t.Fatal("answer-quality-scorecard not registered")
	}
}

// TestAnswerQualityScorecardMalformedEvidence pins the syntax gate:
// malformed JSON fails locally with the offline command's decode error,
// never reaching the server.
func TestAnswerQualityScorecardMalformedEvidence(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	t.Setenv("ESHU_SERVICE_URL", srv.URL)

	cmd, _, _ := newTestScorecardCommand(t)
	cmd.SetIn(strings.NewReader(`{oops`))
	err := cmd.RunE(cmd, []string{})
	if err == nil || !strings.Contains(err.Error(), "decode answer-quality evidence:") {
		t.Fatalf("scorecard RunE error = %v, want decode error", err)
	}
	if hit {
		t.Fatal("malformed evidence reached the server, want local refusal")
	}
}

// TestAnswerQualityScorecardServerRefusal pins the hostile-evidence mapping:
// well-formed JSON of the wrong shape goes to the server, whose 400
// surfaces as the transport error and exits non-zero, never as a
// fabricated verdict.
func TestAnswerQualityScorecardServerRefusal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"bad_request","message":"decode answer-quality evidence: bad"}}`))
	}))
	defer srv.Close()
	t.Setenv("ESHU_SERVICE_URL", srv.URL)

	cmd, _, _ := newTestScorecardCommand(t)
	cmd.SetIn(strings.NewReader(`[1, 2, 3]`))
	err := cmd.RunE(cmd, []string{})
	if err == nil || !strings.Contains(err.Error(), "API error 400") {
		t.Fatalf("scorecard RunE error = %v, want API error 400", err)
	}
}
