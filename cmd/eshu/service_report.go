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

// serviceReportComposeRequest mirrors the S4 compose endpoint's request
// shape: the captured dossier, its truth envelope, and the optional captured
// supply-chain inventory plus its truth. Composition runs server-side; the
// CLI only carries bytes it already holds.
type serviceReportComposeRequest struct {
	Dossier          map[string]any  `json:"dossier"`
	Truth            json.RawMessage `json:"truth"`
	SupplyChain      map[string]any  `json:"supply_chain,omitempty"`
	SupplyChainTruth json.RawMessage `json:"supply_chain_truth,omitempty"`
}

// serviceReportView is the text-mode projection of the composed wire report.
// It classifies the same fixed subset the offline renderer prints, so a
// field visible under --json and missing from the text is a bug only when it
// is in the printed subset below.
type serviceReportView struct {
	Subject struct {
		ServiceName string `json:"service_name"`
		ServiceID   string `json:"service_id"`
	} `json:"subject"`
	Supported  bool   `json:"supported"`
	Partial    bool   `json:"partial"`
	TruthClass string `json:"truth_class"`
	Sections   []struct {
		Status string `json:"status"`
		Title  string `json:"title"`
		Answer struct {
			Summary            string   `json:"summary"`
			UnsupportedReasons []string `json:"unsupported_reasons"`
			Limitations        []string `json:"limitations"`
		} `json:"answer"`
	} `json:"sections"`
	NextCalls []struct {
		Tool     string `json:"tool"`
		Route    string `json:"route"`
		Playbook string `json:"playbook"`
		Reason   string `json:"reason"`
	} `json:"recommended_next_calls"`
	Investigations []struct {
		Basis    string `json:"basis"`
		Reason   string `json:"reason"`
		NextCall struct {
			Tool     string `json:"tool"`
			Route    string `json:"route"`
			Playbook string `json:"playbook"`
		} `json:"next_call"`
		ExpectedTruthClass string `json:"expected_truth_class"`
	} `json:"suggested_investigations"`
}

func newServiceReportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service-report",
		Short: "Compose a service intelligence report from a captured service-story response",
		Long: `service-report composes an operator-ready service intelligence report from a
captured get_service_story response.

It reads JSON from --from or stdin. The input is the response from the service
story route (the standard {"data": ..., "truth": ...} envelope, or a bare
dossier object). The command maps the dossier into the identity,
code_to_runtime, and deployment_config sections, then composes the report. It
runs no query, store, or LLM path: each section's truth is the captured truth,
and sections with no supporting evidence stay visible as partial or unsupported
with explicit limitations and bounded next calls.

Supply-chain and incident sections are emitted unsupported unless their evidence
is wired in; this slice sources only the service story.`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runServiceReport,
	}
	cmd.Flags().String("from", "", "Path to a captured service-story response JSON file (default: read stdin)")
	cmd.Flags().String("supply-chain-from", "", "Optional path to a captured supply-chain impact inventory response JSON file")
	cmd.Flags().Bool("json", false, "Emit the composed report as JSON")
	// No remote flags: like the offline command, the endpoint comes from the
	// ambient ESHU_SERVICE_URL / profile resolution, keeping --help identical.
	return cmd
}

func init() {
	rootCmd.AddCommand(newServiceReportCommand())
}

func runServiceReport(cmd *cobra.Command, _ []string) error {
	path, _ := cmd.Flags().GetString("from")
	supplyChainPath, _ := cmd.Flags().GetString("supply-chain-from")
	jsonOut, _ := cmd.Flags().GetBool("json")

	raw, err := readServiceReportInput(cmd.InOrStdin(), path)
	if err != nil {
		return err
	}
	dossier, truth, err := parseServiceStoryResponse(raw)
	if err != nil {
		return err
	}
	input := serviceReportComposeRequest{Dossier: dossier, Truth: truth}
	if strings.TrimSpace(supplyChainPath) != "" {
		supplyRaw, err := readServiceReportInput(cmd.InOrStdin(), supplyChainPath)
		if err != nil {
			return err
		}
		inventory, inventoryTruth, err := parseServiceStoryResponse(supplyRaw)
		if err != nil {
			return fmt.Errorf("decode supply-chain inventory: %w", err)
		}
		input.SupplyChain = inventory
		input.SupplyChainTruth = inventoryTruth
	}

	var rawEnvelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := apiClient().PostEnvelope("/api/v0/service/report/compose", input, &rawEnvelope); err != nil {
		return err
	}

	if jsonOut {
		indented := bytes.Buffer{}
		if err := json.Indent(&indented, rawEnvelope.Data, "", "  "); err != nil {
			return fmt.Errorf("write service report JSON: %w", err)
		}
		indented.WriteByte('\n')
		_, _ = cmd.OutOrStdout().Write(indented.Bytes())
		return nil
	}
	var view serviceReportView
	if err := json.Unmarshal(rawEnvelope.Data, &view); err != nil {
		return fmt.Errorf("decode service report: %w", err)
	}
	renderServiceReport(cmd.OutOrStdout(), view)
	return nil
}

// readServiceReportInput returns the captured response bytes: the file at
// path when path holds any non-whitespace character, otherwise everything
// readable from stdin, which must be non-empty.
func readServiceReportInput(stdin io.Reader, path string) ([]byte, error) {
	if strings.TrimSpace(path) != "" {
		data, err := os.ReadFile(path) //nolint:gosec // operator-supplied CLI input path, not an HTTP request param
		if err != nil {
			return nil, fmt.Errorf("read service-story response %s: %w", path, err)
		}
		return data, nil
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return nil, fmt.Errorf("read service-story response from stdin: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, fmt.Errorf("no service-story response provided; pass --from or pipe JSON on stdin")
	}
	return data, nil
}

// parseServiceStoryResponse extracts the dossier map and optional truth bytes
// from a captured service-story response. It accepts the standard envelope
// ({"data": ..., "truth": ...}) and falls back to treating the whole object
// as a bare dossier whenever "data" decodes to nil.
func parseServiceStoryResponse(raw []byte) (map[string]any, json.RawMessage, error) {
	var envelope struct {
		Data  map[string]any  `json:"data"`
		Truth json.RawMessage `json:"truth"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, nil, fmt.Errorf("decode service-story response: %w", err)
	}
	if envelope.Data != nil {
		return envelope.Data, envelope.Truth, nil
	}
	var bare map[string]any
	if err := json.Unmarshal(raw, &bare); err != nil {
		return nil, nil, fmt.Errorf("decode service-story dossier: %w", err)
	}
	return bare, nil, nil
}

// renderServiceReport prints the compact, human-readable view of the composed
// report: the subject label, the report-level Supported, Partial, and
// truth class, each section's Status, Title, Summary, UnsupportedReasons, and
// Limitations, each recommended next call's label and Reason, and each
// suggested investigation's Basis, Reason, next-call label, and expected
// truth class.
func renderServiceReport(w io.Writer, report serviceReportView) {
	_, _ = fmt.Fprintf(w, "Service intelligence report: %s\n", serviceReportSubjectLabel(report.Subject.ServiceName, report.Subject.ServiceID))
	_, _ = fmt.Fprintf(w, "  supported=%t partial=%t truth_class=%s\n\n", report.Supported, report.Partial, report.TruthClass)

	for _, section := range report.Sections {
		_, _ = fmt.Fprintf(w, "[%s] %s\n", strings.ToUpper(section.Status), section.Title)
		if summary := strings.TrimSpace(section.Answer.Summary); summary != "" {
			_, _ = fmt.Fprintf(w, "  %s\n", summary)
		}
		for _, reason := range section.Answer.UnsupportedReasons {
			_, _ = fmt.Fprintf(w, "  - %s\n", reason)
		}
		for _, limitation := range section.Answer.Limitations {
			_, _ = fmt.Fprintf(w, "  ! %s\n", limitation)
		}
	}

	if len(report.NextCalls) > 0 {
		_, _ = fmt.Fprintln(w, "\nRecommended next calls:")
		for _, call := range report.NextCalls {
			_, _ = fmt.Fprintf(w, "  - %s", serviceReportNextCallLabel(call.Tool, call.Route, call.Playbook))
			if reason := strings.TrimSpace(call.Reason); reason != "" {
				_, _ = fmt.Fprintf(w, " (%s)", reason)
			}
			_, _ = fmt.Fprintln(w)
		}
	}

	if len(report.Investigations) > 0 {
		_, _ = fmt.Fprintln(w, "\nSuggested investigations:")
		for _, inv := range report.Investigations {
			_, _ = fmt.Fprintf(w, "  - [%s] %s -> %s (expect %s)\n",
				inv.Basis, inv.Reason,
				serviceReportNextCallLabel(inv.NextCall.Tool, inv.NextCall.Route, inv.NextCall.Playbook),
				inv.ExpectedTruthClass)
		}
	}
}

func serviceReportSubjectLabel(name, id string) string {
	if strings.TrimSpace(name) == "" {
		return "(unknown service)"
	}
	if strings.TrimSpace(id) != "" {
		return fmt.Sprintf("%s (%s)", name, id)
	}
	return name
}

func serviceReportNextCallLabel(tool, route, playbook string) string {
	switch {
	case tool != "":
		return tool
	case route != "":
		return route
	case playbook != "":
		return "playbook:" + playbook
	default:
		return "(none)"
	}
}
