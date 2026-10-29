// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eshu-hq/eshu-cli/internal/cli/opdigest"
	"github.com/eshu-hq/eshu-cli/internal/urlredact"
	"github.com/eshu-hq/eshu/sdk/go/apicontract"
)

// reportCaptureInput mirrors the S4 capture endpoint's request shape
// (reportbundle.CaptureInput): the fields the CLI sets, in the same order.
// The server decodes it into its own struct, so this stays a wire mirror,
// never a logic copy: redaction, digestion, and validation run server-side.
type reportCaptureInput struct {
	Surface         string                       `json:"surface"`
	Target          string                       `json:"target"`
	Method          string                       `json:"method,omitempty"`
	Params          map[string]any               `json:"params,omitempty"`
	Profile         string                       `json:"profile,omitempty"`
	ReporterNote    string                       `json:"reporter_note,omitempty"`
	Envelope        apicontract.ResponseEnvelope `json:"envelope"`
	Truncated       bool                         `json:"truncated,omitempty"`
	IncludePayloads bool                         `json:"include_payloads,omitempty"`
}

// reportValidateVerdict mirrors the S4 validate endpoint's verdict.
type reportValidateVerdict struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message,omitempty"`
}

// reportTargetCredentialError reports a reporter-supplied target carrying a
// credential. It is a distinct type so the CLI maps it to the usage exit
// code without matching on message text. The reason strings match the core
// refusal texts, which the S4 endpoint enforces with the same predicate.
type reportTargetCredentialError struct {
	Flag   string
	Reason string
}

func (e *reportTargetCredentialError) Error() string {
	return e.Flag + " " + e.Reason
}

// newReportCommand builds the `eshu report` family: the offline operator
// digest renderer plus the wrong-answer report bundle verbs, which run
// through the S4 capture/validate endpoints.
func newReportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Render a deterministic operator digest for an explicit scope",
		Long: `report renders the operator_digest.v1 model for an explicit scope.

This first CLI implementation is an offline presentation path. It validates
share-safe input, emits deterministic unsupported sections, and points operators
to bounded follow-up routes without reading graph state, writing graph state,
claiming reducer work, or calling providers.`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runReportDigest,
	}
	cmd.Flags().String("scope", "", "Share-safe scope such as repo:owner/name, service:name, workload:name, environment:name, or project:name")
	cmd.Flags().String("profile", opdigest.DefaultProfile, "Runtime profile used to derive the digest")
	cmd.Flags().Int("question-limit", opdigest.DefaultQuestionMax, "Maximum suggested questions to emit (0-25)")
	cmd.Flags().Bool("json", false, "Emit the digest as JSON")
	cmd.Flags().String("artifact-out", "", "Write a shareable operator_digest_artifact.v1 JSON file")
	cmd.AddCommand(newReportCaptureCommand())
	cmd.AddCommand(newReportValidateCommand())
	return cmd
}

func init() {
	rootCmd.AddCommand(newReportCommand())
}

func runReportDigest(cmd *cobra.Command, _ []string) error {
	rawScope, _ := cmd.Flags().GetString("scope")
	rawProfile, _ := cmd.Flags().GetString("profile")
	questionLimit, _ := cmd.Flags().GetInt("question-limit")
	jsonOut, _ := cmd.Flags().GetBool("json")
	artifactOut, _ := cmd.Flags().GetString("artifact-out")

	options, err := opdigest.OptionsFromFlags(rawScope, rawProfile, questionLimit)
	if err != nil {
		return err
	}
	digest := opdigest.BuildDigest(options)
	if strings.TrimSpace(artifactOut) != "" {
		if err := opdigest.WriteArtifact(artifactOut, digest); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "wrote operator digest artifact to %s\n", artifactOut)
	}
	if jsonOut {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if err := enc.Encode(digest); err != nil {
			return fmt.Errorf("write operator digest JSON: %w", err)
		}
		return nil
	}
	opdigest.RenderText(cmd.OutOrStdout(), digest)
	return nil
}

func newReportCaptureCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "capture",
		Short:         "Capture a share-safe wrong_answer_report.v1 bundle from a query",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runReportCapture,
	}
	cmd.Flags().String("endpoint", "", "API path to query (required)")
	cmd.Flags().String("method", "GET", "HTTP method to issue: GET or POST")
	cmd.Flags().String("params", "", "JSON object of query/body parameters as issued")
	cmd.Flags().String("note", "", "What you expected instead of the captured answer")
	cmd.Flags().String("out", "", "Path to write the report bundle JSON; stdout when omitted")
	cmd.Flags().Bool("include-payloads", false, "PRIVATE TRIAGE ONLY: attach raw fact payloads and citation excerpts (never attach to a public issue)")
	cmd.Flags().String("tool", "", "MCP tool name this query originated from, recorded as the surface; --endpoint still resolves the answer (Slice 1 records MCP capture, it does not invoke MCP itself)")
	addRemoteFlags(cmd)
	return cmd
}

func newReportValidateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "validate",
		Short:         "Validate a wrong_answer_report.v1 bundle",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runReportValidate,
	}
	cmd.Flags().String("from", "", "Path to a report bundle JSON file; stdin when omitted")
	cmd.Flags().Bool("require-public", false, "Fail if the bundle's redaction profile is not public (share-safe)")
	// No remote flags: like the offline command, the endpoint comes from the
	// ambient ESHU_SERVICE_URL / profile resolution, keeping --help identical.
	return cmd
}

func runReportCapture(cmd *cobra.Command, _ []string) error {
	endpoint, _ := cmd.Flags().GetString("endpoint")
	if strings.TrimSpace(endpoint) == "" {
		return commandExitError{message: "--endpoint is required", code: 2}
	}
	method, _ := cmd.Flags().GetString("method")
	paramsRaw, _ := cmd.Flags().GetString("params")
	note, _ := cmd.Flags().GetString("note")
	outPath, _ := cmd.Flags().GetString("out")
	includePayloads, _ := cmd.Flags().GetBool("include-payloads")
	tool, _ := cmd.Flags().GetString("tool")

	if err := checkReportTargetCredentials("--endpoint", endpoint); err != nil {
		return commandExitError{message: err.Error(), code: 2}
	}
	if err := checkReportTargetCredentials("--tool", tool); err != nil {
		return commandExitError{message: err.Error(), code: 2}
	}

	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))
	if normalizedMethod == "" {
		normalizedMethod = "GET"
	}
	params := map[string]any{}
	if strings.TrimSpace(paramsRaw) != "" {
		if err := json.Unmarshal([]byte(paramsRaw), &params); err != nil {
			return fmt.Errorf("--params must be a JSON object: %w", err)
		}
	}

	surface := "api"
	target := endpoint
	if strings.TrimSpace(tool) != "" {
		surface = "mcp"
		target = strings.TrimSpace(tool)
	}

	envelope, err := fetchReportEnvelope(apiClientFromCmd(cmd), endpoint, normalizedMethod, params)
	if err != nil {
		return fmt.Errorf("fetch query envelope: %w", err)
	}

	input := reportCaptureInput{
		Surface:         surface,
		Target:          target,
		Method:          normalizedMethod,
		Params:          params,
		Profile:         reportEnvelopeProfile(envelope),
		ReporterNote:    note,
		Envelope:        envelope,
		Truncated:       reportObservedTruncation(envelope),
		IncludePayloads: includePayloads,
	}
	var rawEnvelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := apiClientFromCmd(cmd).PostEnvelope("/api/v0/report/bundle/capture", input, &rawEnvelope); err != nil {
		return err
	}
	if len(bytes.TrimSpace(rawEnvelope.Data)) == 0 {
		return fmt.Errorf("capture report bundle: endpoint answered without a bundle")
	}
	indented := bytes.Buffer{}
	if err := json.Indent(&indented, rawEnvelope.Data, "", "  "); err != nil {
		return fmt.Errorf("capture report bundle: %w", err)
	}
	indented.WriteByte('\n')

	if includePayloads {
		_, _ = fmt.Fprint(cmd.ErrOrStderr(), reportIncludePayloadsWarning)
	}
	if strings.TrimSpace(outPath) != "" {
		//nolint:gosec // operator-supplied CLI output path, not an HTTP request param
		if err := os.WriteFile(outPath, indented.Bytes(), 0o600); err != nil {
			return fmt.Errorf("write report bundle: %w", err)
		}
		return nil
	}
	if _, err := cmd.OutOrStdout().Write(indented.Bytes()); err != nil {
		return fmt.Errorf("write report bundle: %w", err)
	}
	return nil
}

// reportIncludePayloadsWarning matches the offline command's stderr warning
// for --include-payloads, so a terminal user sees it even when stdout carries
// only the bundle JSON.
const reportIncludePayloadsWarning = `
!!! PRIVATE TRIAGE ONLY !!!
This bundle includes raw fact payloads and citation excerpts because
--include-payloads was set. Do NOT attach this bundle to a public GitHub
issue or share it outside your own local triage workflow. Run without
--include-payloads for a share-safe bundle, or run
"eshu report validate --require-public" to confirm before sharing.
`

func runReportValidate(cmd *cobra.Command, _ []string) error {
	from, _ := cmd.Flags().GetString("from")
	requirePublic, _ := cmd.Flags().GetBool("require-public")

	var raw []byte
	if strings.TrimSpace(from) != "" {
		// #nosec G304 -- operator-supplied local bundle path, not an HTTP request param
		data, err := os.ReadFile(from)
		if err != nil {
			return fmt.Errorf("read report bundle: %w", err)
		}
		raw = data
	} else {
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return fmt.Errorf("read report bundle stdin: %w", err)
		}
		raw = data
	}
	var bundle json.RawMessage
	if err := json.Unmarshal(raw, &bundle); err != nil {
		return fmt.Errorf("decode report bundle: %w", err)
	}
	var rawEnvelope struct {
		Data reportValidateVerdict `json:"data"`
	}
	if err := apiClient().PostEnvelope("/api/v0/report/bundle/validate",
		map[string]any{"bundle": bundle, "require_public": requirePublic}, &rawEnvelope); err != nil {
		return err
	}
	if rawEnvelope.Data.Valid {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "report bundle validation: passed")
		return nil
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "report bundle validation: failed")
	if message := strings.TrimSpace(rawEnvelope.Data.Message); message != "" {
		return errors.New(message)
	}
	return errors.New("report bundle validation failed")
}

// checkReportTargetCredentials refuses a reporter-supplied target carrying
// URL userinfo, with the same predicate and reason texts the S4 capture
// endpoint enforces, so the refusal lands before any request goes out.
func checkReportTargetCredentials(flag, value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, _, err := urlredact.Authority(strings.TrimSpace(value))
	if err != nil {
		return &reportTargetCredentialError{Flag: flag, Reason: "cannot be parsed as a URL, so it cannot be checked for an embedded credential; pass a plain path or tool name"}
	}
	if parsed.User != nil {
		return &reportTargetCredentialError{Flag: flag, Reason: "carries a credential in its URL userinfo (user:password@host); remove it and rerun, because the captured bundle is meant to be attached to a public issue"}
	}
	return nil
}

// fetchReportEnvelope issues the reporter's query and decodes it into the
// canonical response envelope, through the same envelope calls every other
// envelope-backed verb uses.
func fetchReportEnvelope(client *APIClient, endpoint, method string, params map[string]any) (apicontract.ResponseEnvelope, error) {
	var envelope apicontract.ResponseEnvelope
	switch method {
	case "GET":
		path, targetParams, err := splitReportTarget(endpoint)
		if err != nil {
			return apicontract.ResponseEnvelope{}, err
		}
		values := url.Values{}
		for key, value := range targetParams {
			repeated, ok := value.([]any)
			if !ok {
				values.Set(key, fmt.Sprintf("%v", value))
				continue
			}
			for _, item := range repeated {
				values.Add(key, fmt.Sprintf("%v", item))
			}
		}
		for key, value := range params {
			values.Set(key, fmt.Sprintf("%v", value))
		}
		requestPath := path
		if len(values) > 0 {
			requestPath += "?" + values.Encode()
		}
		if err := client.GetEnvelope(requestPath, &envelope); err != nil {
			return apicontract.ResponseEnvelope{}, reportRequestError(err, path)
		}
	case "POST":
		postPath, _, err := splitReportTarget(endpoint)
		if err != nil {
			return apicontract.ResponseEnvelope{}, err
		}
		if err := client.PostEnvelope(endpoint, params, &envelope); err != nil {
			return apicontract.ResponseEnvelope{}, reportRequestError(err, postPath)
		}
	default:
		return apicontract.ResponseEnvelope{}, fmt.Errorf("unsupported --method %q: want GET or POST", method)
	}
	return envelope, nil
}

// splitReportTarget separates a report target's query string back into keys
// so the request URL and the recorded bundle derive from one split. An
// unparseable query string is an error: nothing can separate a credential
// from a string that cannot be taken apart.
func splitReportTarget(target string) (string, map[string]any, error) {
	path, rawQuery, found := strings.Cut(target, "?")
	if !found || strings.TrimSpace(rawQuery) == "" {
		return path, nil, nil
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return path, nil, fmt.Errorf("parse query string of query.target: %w", err)
	}
	params := make(map[string]any, len(values))
	for key, list := range values {
		switch len(list) {
		case 0:
			continue
		case 1:
			params[key] = list[0]
		default:
			repeated := make([]any, len(list))
			for i, item := range list {
				repeated[i] = item
			}
			params[key] = repeated
		}
	}
	return path, params, nil
}

// reportRequestError strips the request URL out of a transport error and puts
// the bare endpoint path in its place: the capture command issues the
// reporter's real request, and a failed request must not echo a
// credential-bearing URL to the terminal.
func reportRequestError(err error, safePath string) error {
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		return err
	}
	if urlErr.Op == "parse" {
		return fmt.Errorf("%s %s: %s", urlErr.Op, reportSafeErrorPath(safePath), urlredact.ParseErrorReason(urlErr.Err))
	}
	return fmt.Errorf("%s %s: %w", urlErr.Op, reportSafeErrorPath(safePath), urlErr.Err)
}

// reportSafeErrorPath returns path with any URL userinfo replaced by a fixed
// marker. Unlike checkReportTargetCredentials this redacts instead of
// refusing: the caller is already reporting a failure, and the host and path
// are what a reader needs to fix it.
func reportSafeErrorPath(path string) string {
	parsed, rewritten, err := urlredact.Authority(path)
	if err != nil {
		return "[unparseable endpoint]"
	}
	if parsed.User == nil {
		return path
	}
	parsed.User = url.User("redacted")
	if rewritten {
		return strings.TrimPrefix(parsed.String(), "//")
	}
	return parsed.String()
}

// reportEnvelopeProfile returns the query profile the envelope's truth
// reports, or empty when no truth envelope is present.
func reportEnvelopeProfile(envelope apicontract.ResponseEnvelope) string {
	if envelope.Truth == nil {
		return ""
	}
	return string(envelope.Truth.Profile)
}

// reportObservedTruncation looks for a top-level "truncated" boolean in the
// captured response data.
func reportObservedTruncation(envelope apicontract.ResponseEnvelope) bool {
	data, ok := envelope.Data.(map[string]any)
	if !ok {
		return false
	}
	truncated, ok := data["truncated"].(bool)
	return ok && truncated
}
