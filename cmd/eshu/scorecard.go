// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// scorecardVerdictView is the text-mode projection of a scorecard verdict.
// It renders the header, the run and score lines, one line per scored
// criterion, and the titles and labels of any follow-up issues.
type scorecardVerdictView struct {
	RunID    string `json:"run_id"`
	Pass     bool   `json:"pass"`
	Score    int    `json:"score"`
	Criteria []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Detail string `json:"detail"`
	} `json:"criteria"`
	FollowUpIssues []struct {
		Title  string   `json:"title"`
		Labels []string `json:"labels"`
	} `json:"follow_up_issues"`
}

func newAnswerQualityScorecardCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "answer-quality-scorecard",
		Short: "Score captured answer evidence across API, MCP, CLI, and hosted surfaces",
		Long: `answer-quality-scorecard scores a redacted answer-quality evidence
artifact against the dogfood criteria from issue #1935.

It reads JSON from --from or stdin. The evidence must be captured from real API,
MCP, CLI, or hosted runs before scoring, then redacted so it contains no private
paths, hostnames, credentials, raw addresses, or sensitive excerpts. The command
exits non-zero when usefulness, truth honesty, citation coverage, boundedness,
narration fallback preservation, parity, follow-up usefulness, family coverage,
or publish safety fails.`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runAnswerQualityScorecard,
	}
	cmd.Flags().String("from", "", "Path to a redacted answer-quality scorecard evidence JSON file (default: read stdin)")
	cmd.Flags().Bool("json", false, "Emit the scorecard verdict as JSON")
	// No remote flags: like the offline command, the endpoint comes from the
	// ambient ESHU_SERVICE_URL / profile resolution, keeping --help identical.
	return cmd
}

func init() {
	rootCmd.AddCommand(newAnswerQualityScorecardCommand())
}

func runAnswerQualityScorecard(cmd *cobra.Command, _ []string) error {
	path, _ := cmd.Flags().GetString("from")
	jsonOut, _ := cmd.Flags().GetBool("json")

	var raw []byte
	if strings.TrimSpace(path) == "" {
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return fmt.Errorf("read answer-quality evidence from stdin: %w", err)
		}
		raw = data
	} else {
		// #nosec G304 -- operator-supplied local evidence path, not an HTTP request param
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read answer-quality evidence file %q: %w", path, err)
		}
		raw = data
	}

	// The evidence travels untouched: the S4 endpoint runs the strict
	// structural check and scoring the offline command ran, so re-encoding
	// client-side could only normalize away a distinction the scorer judges.
	// A syntax check up front keeps malformed input on the offline command's
	// error ("decode answer-quality evidence: ...") instead of leaking a
	// request-marshal failure; well-formed JSON of the wrong shape still
	// goes to the server and comes back a 400.
	var syntaxProbe any
	if err := json.Unmarshal(raw, &syntaxProbe); err != nil {
		return fmt.Errorf("decode answer-quality evidence: %w", err)
	}
	var rawEnvelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := apiClient().Post("/api/v0/answer-quality/score", json.RawMessage(raw), &rawEnvelope); err != nil {
		return err
	}

	var view scorecardVerdictView
	if err := json.Unmarshal(rawEnvelope.Data, &view); err != nil {
		return fmt.Errorf("decode answer-quality scorecard: %w", err)
	}
	if jsonOut {
		indented := bytes.Buffer{}
		if err := json.Indent(&indented, rawEnvelope.Data, "", "  "); err != nil {
			return fmt.Errorf("write answer-quality scorecard JSON: %w", err)
		}
		indented.WriteByte('\n')
		_, _ = cmd.OutOrStdout().Write(indented.Bytes())
	} else {
		renderScorecardVerdict(cmd.OutOrStdout(), view)
	}
	if !view.Pass {
		return fmt.Errorf("answer-quality scorecard FAILED: %s", scorecardFailureSummary(view))
	}
	return nil
}

// renderScorecardVerdict writes the compact, human-readable view of a
// scorecard verdict. Statuses the scorer does not classify as pass or fail
// render as not-measured rather than as a pass.
func renderScorecardVerdict(w io.Writer, verdict scorecardVerdictView) {
	header := "Answer-quality scorecard PASSED"
	if !verdict.Pass {
		header = "Answer-quality scorecard FAILED"
	}
	_, _ = fmt.Fprintln(w, header)
	_, _ = fmt.Fprintf(w, "  run   : %s\n", scorecardQuoteIfEmpty(verdict.RunID))
	_, _ = fmt.Fprintf(w, "  score : %d\n", verdict.Score)
	_, _ = fmt.Fprintln(w, strings.Repeat("-", 44))
	for _, criterion := range verdict.Criteria {
		_, _ = fmt.Fprintf(w, "  %s %s: %s\n", scorecardMarker(criterion.Status), criterion.Name, criterion.Detail)
	}
	if len(verdict.FollowUpIssues) > 0 {
		_, _ = fmt.Fprintln(w, "follow-up issues:")
		for _, issue := range verdict.FollowUpIssues {
			_, _ = fmt.Fprintf(w, "  - %s [%s]\n", issue.Title, strings.Join(issue.Labels, ", "))
		}
	}
}

// scorecardFailureSummary joins every failed criterion into one line for the
// error the command returns when the scorecard rejects its evidence.
func scorecardFailureSummary(verdict scorecardVerdictView) string {
	var failures []string
	for _, criterion := range verdict.Criteria {
		if criterion.Status == "fail" {
			failures = append(failures, fmt.Sprintf("%s: %s", criterion.Name, criterion.Detail))
		}
	}
	if len(failures) == 0 {
		return "unknown failure"
	}
	return strings.Join(failures, "; ")
}

// scorecardMarker is the fixed-width status glyph each criterion line opens
// with.
func scorecardMarker(status string) string {
	switch status {
	case "pass":
		return "[ok]"
	case "fail":
		return "[!!]"
	default:
		return "[--]"
	}
}

// scorecardQuoteIfEmpty renders a placeholder for an empty run id.
func scorecardQuoteIfEmpty(value string) string {
	if strings.TrimSpace(value) == "" {
		return "<repo>"
	}
	return value
}
