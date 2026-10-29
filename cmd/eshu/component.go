// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	clicomponent "github.com/eshu-hq/eshu-cli/internal/cli/component"
)

// This file is the cobra wiring for the `eshu component` family. The thin
// inventory and diagnostics verbs fetch through the configured API via the
// extracted internal/cli/component package; the local verbs (inspect,
// verify, install, list, enable, disable, uninstall, conform, init, index,
// extraction-readiness, schema-versions, and the grant verbs) run against the
// local extension host and stay in the core binary until the
// eshu-extension-host move owns them, so they keep their core surface
// (Use/Short/Args/flags) with a stub body. Everything here resolves flags,
// environment, and streams, then passes plain values across.

// componentLocalGuidance is the redirect every extension-host-bound verb
// prints: unlike the E3 family stubs, these verbs are not waiting on a wire
// contract but on the extension-host move.
const componentLocalGuidance = "This verb runs against the local extension host and stays in the core eshu binary. Refs eshu-hq/eshu#8874."

// The component family's flag names live here, with five exceptions.
// --instance, --version, --id, --publisher, and --fact-kind are declared by
// internal/cli/component, because that package prints them in its
// "--<flag> is required" errors. Whoever prints a name owns it: a second copy
// here would let a rename on one side leave that message pointing at a flag
// this binary no longer registers. The registrations below use
// clicomponent.InstanceFlag and friends instead, so such a rename is a
// compile error.
const (
	componentHomeFlag                    = "component-home"
	componentTrustModeFlag               = "trust-mode"
	componentAllowIDFlag                 = "allow-id"
	componentAllowPublisherFlag          = "allow-publisher"
	componentRevokeIDFlag                = "revoke-id"
	componentRevokePublisherFlag         = "revoke-publisher"
	componentCosignBinaryFlag            = "cosign-binary"
	componentProvenanceIdentityFlag      = "provenance-certificate-identity"
	componentProvenanceIssuerFlag        = "provenance-oidc-issuer"
	componentProvenancePredicateTypeFlag = "provenance-predicate-type"
	componentCoreVersionFlag             = "core-version"
	componentModeFlag                    = "mode"
	componentClaimsFlag                  = "claims"
	componentConfigFlag                  = "config"
	componentJSONFlag                    = "json"
	componentDryRunFlag                  = "dry-run"
	componentFixtureFlag                 = "fixture"
	componentInitOutputFlag              = "output"
	componentSchemaCheckFlag             = "check"

	componentExtractionReadinessVerboseFlag = "verbose"
)

const componentGrantProducerFlag = "producer"

// componentTrustModeDisabledDefault is the --trust-mode default for the
// package-verifying verbs (core component.TrustModeDisabled).
const componentTrustModeDisabledDefault = "disabled"

// componentProvenancePredicateDefault is the --provenance-predicate-type
// default (core component.DefaultProvenancePredicateType).
const componentProvenancePredicateDefault = "slsaprovenance1"

func newComponentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "component",
		Short: "Manage optional Eshu components",
	}

	inspectCmd := &cobra.Command{
		Use:   "inspect <manifest>",
		Short: "Inspect a component package manifest",
		Args:  cobra.ExactArgs(1),
		RunE:  componentLocalStub("inspect"),
	}
	verifyCmd := &cobra.Command{
		Use:   "verify <manifest>",
		Short: "Verify a component package manifest against local trust policy",
		Args:  cobra.ExactArgs(1),
		RunE:  componentLocalStub("verify"),
	}
	installCmd := &cobra.Command{
		Use:   "install <manifest>",
		Short: "Install a verified local component package manifest",
		Args:  cobra.ExactArgs(1),
		RunE:  componentLocalStub("install"),
	}
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List installed components",
		Args:  cobra.NoArgs,
		RunE:  componentLocalStub("list"),
	}
	enableCmd := &cobra.Command{
		Use:   "enable <component-id>",
		Short: "Enable an installed component instance",
		Args:  cobra.ExactArgs(1),
		RunE:  componentLocalStub("enable"),
	}
	disableCmd := &cobra.Command{
		Use:   "disable <component-id>",
		Short: "Disable an installed component instance",
		Args:  cobra.ExactArgs(1),
		RunE:  componentLocalStub("disable"),
	}
	uninstallCmd := &cobra.Command{
		Use:   "uninstall <component-id>",
		Short: "Uninstall an inactive component package",
		Args:  cobra.ExactArgs(1),
		RunE:  componentLocalStub("uninstall"),
	}
	conformCmd := &cobra.Command{
		Use:   "conform <manifest>",
		Short: "Run component extension conformance fixtures",
		Args:  cobra.ExactArgs(1),
		RunE:  componentLocalStub("conform"),
	}
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold a new component package",
	}
	initCollectorCmd := &cobra.Command{
		Use:   "collector",
		Short: "Scaffold a collector component package",
		Args:  cobra.NoArgs,
		RunE:  componentLocalStub("init collector"),
	}

	addComponentHomeFlag(inspectCmd)
	addComponentHomeFlag(verifyCmd)
	addComponentHomeFlag(installCmd)
	addComponentHomeFlag(listCmd)
	addComponentHomeFlag(enableCmd)
	addComponentHomeFlag(disableCmd)
	addComponentHomeFlag(uninstallCmd)
	addComponentHomeFlag(conformCmd)
	addTrustFlags(verifyCmd)
	addTrustFlags(installCmd)
	addOptionalTrustFlags(listCmd)
	for _, c := range []*cobra.Command{inspectCmd, verifyCmd, installCmd, listCmd, enableCmd, disableCmd, uninstallCmd, conformCmd} {
		addComponentJSONFlag(c)
	}
	addComponentJSONFlag(initCollectorCmd)
	installCmd.Flags().Bool(componentDryRunFlag, false, "Verify install and render the planned result without writing component state")
	enableCmd.Flags().String(clicomponent.InstanceFlag, "", "Collector instance ID to enable")
	enableCmd.Flags().String(componentModeFlag, "manual", "Collector activation mode")
	enableCmd.Flags().Bool(componentClaimsFlag, false, "Enable workflow claims for this component instance")
	enableCmd.Flags().String(componentConfigFlag, "", "Path to component instance configuration")
	enableCmd.Flags().Bool(componentDryRunFlag, false, "Validate activation and render the planned result without writing component state")
	disableCmd.Flags().String(clicomponent.InstanceFlag, "", "Collector instance ID to disable")
	uninstallCmd.Flags().String(clicomponent.VersionFlag, "", "Component version to uninstall")
	conformCmd.Flags().StringSlice(componentFixtureFlag, nil, "Collector SDK result fixture JSON file; repeat for multiple fixtures")
	conformCmd.Flags().String(componentModeFlag, "fixture", "Conformance mode: fixture or compose")
	initCollectorCmd.Flags().String(clicomponent.InitIDFlag, "", "Component ID, for example dev.example.collector.demo")
	initCollectorCmd.Flags().String(clicomponent.InitPublisherFlag, "", "Component publisher allowlist identity")
	initCollectorCmd.Flags().String(clicomponent.InitFactKindFlag, "", "Namespaced fact kind emitted by the scaffold")
	initCollectorCmd.Flags().String(componentInitOutputFlag, "", "Output directory; defaults to ./<component-id>")

	initCmd.AddCommand(initCollectorCmd)

	indexCmd := &cobra.Command{
		Use:   "index",
		Short: "Manage component extension index metadata",
	}
	indexVerifyCmd := &cobra.Command{
		Use:   "verify <index>",
		Short: "Verify a component extension index for local or CI publication gates",
		Args:  cobra.ExactArgs(1),
		RunE:  componentLocalStub("index verify"),
	}
	addComponentJSONFlag(indexVerifyCmd)
	indexCmd.AddCommand(indexVerifyCmd)

	readinessCmd := &cobra.Command{
		Use:   "extraction-readiness [collector-family]",
		Short: "Explain whether a collector is keep-in-tree, an extraction candidate, blocked, or external-ready",
		Long: "Report the advisory collector extraction readiness checklist. The output is " +
			"informational: it never moves code or changes runtime behavior. With no argument it " +
			"lists every collector family the extraction policy tracks; with a family argument it " +
			"explains that single family's per-criterion checklist.",
		Args: cobra.MaximumNArgs(1),
		RunE: componentLocalStub("extraction-readiness"),
	}
	readinessCmd.Flags().Bool(componentJSONFlag, false, "Emit machine-readable JSON")
	readinessCmd.Flags().Bool(componentExtractionReadinessVerboseFlag, false, "Show every criterion, not just blockers")

	versionsCmd := &cobra.Command{
		Use:   "schema-versions",
		Short: "List core fact-kind schema versions or classify a collector fact version",
		Long: "Report the schema version each core reducer or query consumer currently " +
			"supports for every core fact kind. With --check fact_kind=version it classifies " +
			"a collector's fact schema version as supported, unsupported_major, " +
			"unsupported_minor, or unknown_kind, and exits non-zero when the version is not " +
			"supported. The command is read-only and never changes runtime behavior.",
		Args: cobra.NoArgs,
		RunE: componentLocalStub("schema-versions"),
	}
	versionsCmd.Flags().Bool(componentJSONFlag, false, "Emit machine-readable JSON")
	versionsCmd.Flags().String(componentSchemaCheckFlag, "", "Classify one fact version as fact_kind=schema_version")

	grantCmd := &cobra.Command{
		Use:   "grant <producer-id>",
		Short: "Record a core-issued producer grant for a core-owned fact kind",
		Args:  cobra.ExactArgs(1),
		RunE:  componentLocalStub("grant"),
	}
	revokeGrantCmd := &cobra.Command{
		Use:   "revoke-grant <producer-id>",
		Short: "Revoke a recorded producer grant so emissions fail closed",
		Args:  cobra.ExactArgs(1),
		RunE:  componentLocalStub("revoke-grant"),
	}
	grantsCmd := &cobra.Command{
		Use:   "grants",
		Short: "List recorded producer grants",
		Args:  cobra.NoArgs,
		RunE:  componentLocalStub("grants"),
	}
	addComponentHomeFlag(grantCmd)
	addComponentHomeFlag(revokeGrantCmd)
	addComponentHomeFlag(grantsCmd)
	addComponentJSONFlag(grantCmd)
	addComponentJSONFlag(revokeGrantCmd)
	addComponentJSONFlag(grantsCmd)
	grantCmd.Flags().String(clicomponent.VersionFlag, "", "Granted manifest version to bind")
	grantCmd.Flags().String(clicomponent.GrantKindFlag, "", "Core-owned fact kind the producer may emit")
	grantCmd.Flags().StringSlice(clicomponent.GrantSchemaVersionFlag, nil, "Covered fact schema version; repeat for multiple versions")
	grantCmd.Flags().String(clicomponent.GrantScopeFlag, "", "Source scope the grant is valid for")
	grantCmd.Flags().Duration(clicomponent.GrantExpiresInFlag, 0, "Grant lifetime from issuance, for example 720h")
	revokeGrantCmd.Flags().String(clicomponent.VersionFlag, "", "Granted manifest version to revoke")
	revokeGrantCmd.Flags().String(clicomponent.GrantKindFlag, "", "Core-owned fact kind to revoke")
	revokeGrantCmd.Flags().String(clicomponent.GrantScopeFlag, "", "Source scope to revoke")
	grantsCmd.Flags().String(componentGrantProducerFlag, "", "Show only grants for one producer ID")

	cmd.AddCommand(
		initCmd, inspectCmd, verifyCmd, installCmd, listCmd, enableCmd,
		disableCmd, uninstallCmd, conformCmd, indexCmd, readinessCmd,
		versionsCmd, grantCmd, revokeGrantCmd, grantsCmd,
		newComponentInventoryCommand(), newComponentDiagnosticsCommand(),
	)
	return cmd
}

func init() {
	rootCmd.AddCommand(newComponentCommand())
}

// componentLocalStub returns the RunE for an extension-host-bound verb: the
// core surface with a stub body pointing at the core binary.
func componentLocalStub(name string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		return removedCommandError("eshu component "+name, componentLocalGuidance)
	}
}

// The fetch indirection points at the extracted package; it keeps the call
// sites in the run functions swappable in one place.
var (
	componentFetchInventory   = clicomponent.FetchInventory
	componentFetchDiagnostics = clicomponent.FetchDiagnostics
)

func newComponentInventoryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inventory",
		Short: "List component extensions through the configured API",
		Args:  cobra.NoArgs,
		RunE:  runComponentInventory,
	}
	addComponentAPIFlags(cmd)
	cmd.Flags().Int("limit", clicomponent.InventoryDefaultLimit, "Maximum number of component rows to return")
	return cmd
}

func newComponentDiagnosticsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diagnostics <component-id>",
		Short: "Read component extension diagnostics through the configured API",
		Args:  cobra.ExactArgs(1),
		RunE:  runComponentDiagnostics,
	}
	addComponentAPIFlags(cmd)
	return cmd
}

func addComponentAPIFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("json", false, "Write the canonical component extension envelope as JSON")
	addRemoteFlags(cmd)
}

func runComponentInventory(cmd *cobra.Command, _ []string) error {
	limit, err := componentAPILimit(cmd)
	if err != nil {
		return err
	}
	jsonOutput, _ := cmd.Flags().GetBool("json")
	envelope, err := componentFetchInventory(apiClientFromCmd(cmd), limit)
	if err != nil {
		envelope = clicomponent.Envelope{Error: &clicomponent.EnvelopeError{
			Code:    traceErrorCodeFromTransport(err),
			Message: err.Error(),
		}}
		return finishComponentAPI(cmd, jsonOutput, envelope, componentAPIEnvelopeError(envelope.Error))
	}
	if envelope.Error != nil {
		return finishComponentAPI(cmd, jsonOutput, envelope, componentAPIEnvelopeError(envelope.Error))
	}
	return finishComponentAPI(cmd, jsonOutput, envelope, nil)
}

func runComponentDiagnostics(cmd *cobra.Command, args []string) error {
	jsonOutput, _ := cmd.Flags().GetBool("json")
	componentID := strings.TrimSpace(args[0])
	if componentID == "" {
		return commandExitError{message: "component id is required", code: 2}
	}
	envelope, err := componentFetchDiagnostics(apiClientFromCmd(cmd), componentID)
	if err != nil {
		envelope = clicomponent.Envelope{Error: &clicomponent.EnvelopeError{
			Code:    traceErrorCodeFromTransport(err),
			Message: err.Error(),
		}}
		return finishComponentAPI(cmd, jsonOutput, envelope, componentAPIEnvelopeError(envelope.Error))
	}
	if envelope.Error != nil {
		return finishComponentAPI(cmd, jsonOutput, envelope, componentAPIEnvelopeError(envelope.Error))
	}
	return finishComponentAPI(cmd, jsonOutput, envelope, nil)
}

func componentAPILimit(cmd *cobra.Command) (int, error) {
	if cmd.Flags().Lookup("limit") == nil {
		return 0, nil
	}
	limit, err := cmd.Flags().GetInt("limit")
	if err != nil {
		return 0, err
	}
	if limit < 1 || limit > clicomponent.InventoryMaxLimit {
		return 0, commandExitError{
			message: fmt.Sprintf("limit must be between 1 and %d", clicomponent.InventoryMaxLimit),
			code:    2,
		}
	}
	return limit, nil
}

// finishComponentAPI resolves the command's stream and hands terminal output
// to the extracted package.
func finishComponentAPI(cmd *cobra.Command, jsonOutput bool, envelope clicomponent.Envelope, err error) error {
	return clicomponent.FinishAPI(cmd.OutOrStdout(), jsonOutput, envelope, err)
}

// componentAPIEnvelopeError maps an envelope error onto the CLI's exit-code
// contract. The mapping table is traceExitCode, shared with `eshu trace` and
// `eshu map`; commandExitError is defined in this package, so the conversion
// stays here rather than moving out with the component logic.
func componentAPIEnvelopeError(e *clicomponent.EnvelopeError) error {
	if e == nil {
		return nil
	}
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = strings.TrimSpace(e.Code)
	}
	if message == "" {
		message = "component extension request failed"
	}
	return commandExitError{message: message, code: traceExitCode(e.Code)}
}

func addComponentHomeFlag(cmd *cobra.Command) {
	cmd.Flags().String(componentHomeFlag, "", "Component registry home directory")
}

func addComponentJSONFlag(cmd *cobra.Command) {
	cmd.Flags().Bool(componentJSONFlag, false, "Write stable JSON output")
}

func addTrustFlags(cmd *cobra.Command) {
	addTrustFlagsWithDefault(cmd, componentTrustModeDisabledDefault)
}

func addOptionalTrustFlags(cmd *cobra.Command) {
	addTrustFlagsWithDefault(cmd, "")
}

func addTrustFlagsWithDefault(cmd *cobra.Command, defaultMode string) {
	cmd.Flags().String(componentTrustModeFlag, defaultMode, "Component trust mode: disabled, allowlist, or strict")
	cmd.Flags().StringSlice(componentAllowIDFlag, nil, "Allowed component ID")
	cmd.Flags().StringSlice(componentAllowPublisherFlag, nil, "Allowed component publisher")
	cmd.Flags().StringSlice(componentRevokeIDFlag, nil, "Revoked component ID")
	cmd.Flags().StringSlice(componentRevokePublisherFlag, nil, "Revoked component publisher")
	cmd.Flags().String(componentCosignBinaryFlag, "", "Cosign verifier binary for strict component trust")
	cmd.Flags().String(componentProvenanceIdentityFlag, "", "Expected Sigstore certificate identity for strict component trust")
	cmd.Flags().String(componentProvenanceIssuerFlag, "", "Expected Sigstore OIDC issuer for strict component trust")
	cmd.Flags().String(
		componentCoreVersionFlag,
		"",
		"Eshu core version the operator asserts for compatibleCore checks (default: this build's version)",
	)
	cmd.Flags().String(
		componentProvenancePredicateTypeFlag,
		componentProvenancePredicateDefault,
		"Cosign attestation predicate type for strict component trust",
	)
}
