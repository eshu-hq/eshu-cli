// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/eshu-hq/eshu-cli/internal/cli/mcpsetup"
	"github.com/eshu-hq/eshu/sdk/go/apicontract"
	"github.com/spf13/cobra"
)

// mcpSetupLongHelp is the `eshu mcp setup` (and `eshu m`) long help text. It
// describes the three auth postures (issue #5169, F-8) rather than
// advertising the shared ${ESHU_API_KEY} as the hosted default: that
// credential is now the shared-key posture, opt-in only.
const mcpSetupLongHelp = "Print platform-specific MCP client config and optionally install it.\n\n" +
	"By default this prints a safe snippet and writes nothing. Use --write to\n" +
	"merge the eshu server entry into the platform config, preserving existing\n" +
	"servers and keys. Use --hosted with --service-url for an HTTP endpoint.\n\n" +
	"--auth selects the credential story for hosted setup: auto (default)\n" +
	"probes the server's RFC 9728 discovery route and resolves per-user token\n" +
	"or SSO; sso forces the OAuth flow; token forces the per-user\n" +
	"${ESHU_MCP_TOKEN} reference; shared-key (or --shared-key) forces the\n" +
	"legacy admin/dev ${ESHU_API_KEY} credential, never emitted by default.\n" +
	"No posture ever prints a raw secret."

// mcpStartGuidance redirects the RUNTIME server launchers: starting a local
// server is local execution, not a query, and stays in the core binary.
const mcpStartGuidance = "Local server launchers stay in the core eshu binary. Run the core binary instead. Refs eshu-hq/eshu#8874."

// addMCPSetupFlags registers the flags shared by `eshu mcp setup` and its `eshu
// m` alias on cmd. Both commands must expose the identical flag set: a command
// that forgets one of these silently reads a zero value for it instead of
// failing, which is why registration is centralized here rather than repeated
// per command.
func addMCPSetupFlags(cmd *cobra.Command) {
	cmd.Flags().String("platform", "generic", "Target MCP client: "+strings.Join(mcpsetup.SupportedPlatformNames(), ", "))
	cmd.Flags().Bool("hosted", false, "Generate hosted HTTP setup instead of local stdio")
	cmd.Flags().Bool("write", false, "Merge the config into the platform's file instead of printing it")
	cmd.Flags().String("target", "", "Override the file path used by --write")
	cmd.Flags().Bool("verify", false, "Run staged verification (config, reachable, tools, first query)")
	cmd.Flags().String("auth", "auto", "MCP auth posture for hosted setup: auto, sso, token, or shared-key")
	cmd.Flags().Bool("shared-key", false, "Force the legacy shared ESHU_API_KEY credential (admin/dev only)")
	addRemoteFlags(cmd)
}

// newMCPParentCommand builds the `eshu mcp` family: the RUNTIME start
// launcher stays stubbed, setup runs thin, and tools prints its static
// pointer.
func newMCPParentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server commands",
	}
	mcpStartCmd := &cobra.Command{
		Use:   "start",
		Short: "Start the Eshu MCP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return removedCommandError("eshu mcp start", mcpStartGuidance)
		},
	}
	mcpStartCmd.Flags().StringP("transport", "t", "stdio", "Transport mode: stdio, http, or sse")
	mcpStartCmd.Flags().String("host", "0.0.0.0", "Host to bind HTTP MCP server")
	mcpStartCmd.Flags().IntP("port", "p", 8080, "Port for HTTP MCP server")
	mcpStartCmd.Flags().String("workspace-root", "", "Explicit workspace root for the local Eshu owner")
	mcpStartCmd.Flags().String("profile", "", "Runtime profile for a new local owner: local_authoritative (default; embedded NornicDB graph for call-graph and Cypher answers) or local_lightweight (Postgres only, no graph). For a Neo4j-backed authoritative owner, set ESHU_QUERY_PROFILE and ESHU_GRAPH_BACKEND instead.")
	cmd.AddCommand(mcpStartCmd)
	cmd.AddCommand(newMCPSetupCmd())
	mcpToolsCmd := &cobra.Command{
		Use:   "tools",
		Short: "List available MCP tools",
		Run: func(cmd *cobra.Command, args []string) {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "MCP tools are served by the Go MCP server.")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Start the server with 'eshu mcp start' and connect via your IDE.")
		},
	}
	cmd.AddCommand(mcpToolsCmd)
	return cmd
}

// newMCPSetupCmd builds the `eshu mcp setup` command.
func newMCPSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Configure IDE and CLI MCP integrations",
		Long:  mcpSetupLongHelp,
		RunE:  runMCPSetup,
	}
	addMCPSetupFlags(cmd)
	return cmd
}

// newMCPSetupAliasCmd builds the `eshu m` shortcut for `eshu mcp setup`.
func newMCPSetupAliasCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "m",
		Short:  "Shortcut for 'eshu mcp setup'",
		Hidden: false,
		RunE:   runMCPSetup,
	}
	addMCPSetupFlags(cmd)
	return cmd
}

func init() {
	rootCmd.AddCommand(newMCPParentCommand())
	rootCmd.AddCommand(newMCPSetupAliasCmd())
}

func runMCPSetup(cmd *cobra.Command, args []string) error {
	platformName, _ := cmd.Flags().GetString("platform")
	hosted, _ := cmd.Flags().GetBool("hosted")
	write, _ := cmd.Flags().GetBool("write")
	target, _ := cmd.Flags().GetString("target")
	verify, _ := cmd.Flags().GetBool("verify")
	authFlag, _ := cmd.Flags().GetString("auth")
	sharedKey, _ := cmd.Flags().GetBool("shared-key")

	platform, err := mcpsetup.ResolvePlatform(platformName)
	if err != nil {
		return err
	}

	req := mcpsetup.SetupRequest{Mode: mcpsetup.ModeLocalStdio}
	if hosted {
		req.Mode = mcpsetup.ModeHostedHTTP
		client := apiClientFromCmd(cmd)
		req.ServiceURL = client.BaseURL
		req.APIKey = client.APIKey
	}

	posture, err := mcpsetup.ResolveAuthPosture(authFlag, sharedKey, hosted, mcpsetup.HostedPostureProbe, req.ServiceURL)
	if err != nil {
		return err
	}
	req.Posture = posture.Posture
	req.Issuers = posture.Issuers
	req.PreregisteredClientID = posture.PreregisteredClientID
	if posture.Warning != "" {
		printWarning(posture.Warning)
	}

	if write {
		return mcpSetupWrite(platform, req, target)
	}

	if verify {
		return mcpSetupVerify(cmd, platform, req)
	}

	snippet, err := mcpsetup.RenderSetupSnippet(platform, req)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprint(cmd.OutOrStdout(), snippet)
	return nil
}

// mcpSetupWrite merges the eshu entry into the platform config file and reports
// where it landed. It never prints a raw token.
func mcpSetupWrite(platform *mcpsetup.Platform, req mcpsetup.SetupRequest, target string) error {
	if !platform.Writable {
		return fmt.Errorf("platform %q does not support --write; print the snippet and add it to %s manually",
			platform.Name, platform.TargetFile)
	}
	path := strings.TrimSpace(target)
	if path == "" {
		def, err := mcpsetup.DefaultWriteTarget(platform)
		if err != nil {
			return err
		}
		path = def
	}
	if err := mcpsetup.WriteMCPServerConfig(platform, req, path); err != nil {
		return err
	}
	printSuccess(fmt.Sprintf("Merged eshu MCP server into %s", mcpsetup.DescribeWriteTarget(path)))
	return nil
}

// mcpSetupVerify runs the staged verification, reusing the API client for hosted
// reachability and the deployment's remote tools/list (decoded via
// apicontract) for tool visibility. The thin CLI carries no embedded tool
// surface, so without an endpoint bearing a credential the tools stage fails
// closed instead of passing on a list the binary does not hold.
//
// The authenticated first-query stage must exercise the SAME credential the
// emitted snippet wires, and must be skipped -- never probed with the wrong
// key, never failed -- when the CLI holds no such credential:
//
//   - token posture: prefer the per-user ESHU_MCP_TOKEN the snippet references,
//     overriding any resolved shared ESHU_API_KEY/--api-key so --verify cannot
//     give false confidence in the admin/dev key. With no ESHU_MCP_TOKEN, run
//     only the public health stage and skip the query rather than fall back to
//     the shared key.
//   - SSO posture: OAuth is interactive and the CLI holds no bearer, so the
//     auth-enforced query would 401. Skip it (health still runs against the
//     public /health route) so --verify does not spuriously fail.
//   - shared-key posture: the shared key is the intended credential, so both
//     health and the query run against it.
//
// The public /health reachability probe runs under every hosted posture.
func mcpSetupVerify(cmd *cobra.Command, platform *mcpsetup.Platform, req mcpsetup.SetupRequest) error {
	snippet, err := mcpsetup.RenderSetupSnippet(platform, req)
	if err != nil {
		snippet = ""
	}

	var health mcpsetup.HealthProber
	var query mcpsetup.QueryProber
	var querySkipReason string
	var tools func() []apicontract.ToolDefinition
	if req.Mode == mcpsetup.ModeHostedHTTP {
		client := apiClientFromCmd(cmd)
		switch req.Posture {
		case mcpsetup.PostureSSO:
			querySkipReason = "OAuth is interactive; complete sign-in via your client (e.g. Claude Code /mcp)"
		case mcpsetup.PostureSharedKey:
			query = apiQueryProber{client: client}
			tools = remoteMCPTools(client)
		default: // mcpsetup.PostureToken
			if token := strings.TrimSpace(os.Getenv(mcpsetup.MCPTokenEnvVar)); token != "" {
				client.APIKey = token
				query = apiQueryProber{client: client}
				tools = remoteMCPTools(client)
			} else {
				querySkipReason = "set " + mcpsetup.MCPTokenEnvVar + " to verify the personal credential"
			}
		}
		// Built after any credential override so the public health probe uses
		// the same resolved client (the *APIClient pointer the query prober
		// also holds).
		health = apiHealthProber{client: client}
	}

	report := mcpsetup.RunVerification(snippet, tools, health, query, querySkipReason)
	_, _ = fmt.Fprint(cmd.OutOrStdout(), postureVerifyHeader(req)+mcpsetup.RenderVerifyReport(report))
	if !report.AllOK() {
		return fmt.Errorf("mcp setup verification failed")
	}
	return nil
}

// remoteMCPTools returns a tool lister that decodes the deployment's remote
// tools/list response through apicontract.ToolDefinition. A transport or
// protocol failure yields no definitions, which fails the tools-visible stage
// rather than passing it on stale data.
func remoteMCPTools(client *APIClient) func() []apicontract.ToolDefinition {
	return func() []apicontract.ToolDefinition {
		var response struct {
			Result *struct {
				Tools []apicontract.ToolDefinition `json:"tools"`
			} `json:"result"`
		}
		if err := client.Post("/mcp/message",
			map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"},
			&response); err != nil {
			return nil
		}
		if response.Result == nil {
			return nil
		}
		return response.Result.Tools
	}
}

// apiHealthProber probes the hosted API /health endpoint via the shared
// client. It stays in cmd/eshu because it wraps *APIClient, which reads
// process-level flags and settings that mcpsetup cannot depend on.
type apiHealthProber struct {
	client *APIClient
}

// Reachable issues a GET /health and treats any non-error response as reachable.
func (p apiHealthProber) Reachable() error {
	if err := p.client.Get("/health", nil); err != nil {
		return fmt.Errorf("health probe %s failed: %w", p.client.BaseURL, err)
	}
	return nil
}

// apiQueryProber runs one bounded index-status read as the first-query smoke.
// It stays in cmd/eshu for the same reason as apiHealthProber: it wraps
// *APIClient.
type apiQueryProber struct {
	client *APIClient
}

// Smoke fetches index status, a cheap bounded read that proves the query surface
// answers without fetching a large payload.
func (p apiQueryProber) Smoke() error {
	var result map[string]any
	if err := p.client.Get("/api/v0/index-status", &result); err != nil {
		return fmt.Errorf("index-status query failed: %w", err)
	}
	return nil
}

// postureVerifyHeader names the detected auth posture and probe outcome as a
// leading line before the staged verification report, so `--verify` output is
// self-explanatory about which credential story it exercised. Empty for local
// stdio mode, which carries no credential to name.
func postureVerifyHeader(req mcpsetup.SetupRequest) string {
	if req.Mode != mcpsetup.ModeHostedHTTP {
		return ""
	}
	switch req.Posture {
	case mcpsetup.PostureSSO:
		if len(req.Issuers) > 0 {
			return fmt.Sprintf("Auth posture: sso (issuer: %s)\n", req.Issuers[0])
		}
		return "Auth posture: sso\n"
	case mcpsetup.PostureSharedKey:
		return "Auth posture: shared-key (admin/dev " + mcpsetup.APIKeyEnvVar + ")\n"
	default: // mcpsetup.PostureToken
		return "Auth posture: token (per-user " + mcpsetup.MCPTokenEnvVar + ")\n"
	}
}
