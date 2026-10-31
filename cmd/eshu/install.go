// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eshu-hq/eshu-cli/internal/cli/graphinstall"
)

// newInstallCommand builds the `eshu install` family. The command owns the
// backend-tooling install workflow the inventory assigns to this CLI:
// `install nornicdb` verifies an explicit source artifact and copies it
// into the managed home the install contract pins. The `graph` sidecar
// lifecycle (including `graph upgrade`) stays in the core binary per the
// RETAIN row, so this family carries no workspace flags and no supervisor.
func newInstallCommand() *cobra.Command {
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install local graph backend tooling",
	}
	installCmd.AddCommand(newInstallNornicDBCommand())
	return installCmd
}

// newInstallNornicDBCommand builds the `eshu install nornicdb` verb with a
// fresh flag set per call, so tests exercise flag parsing without sharing
// global command state.
func newInstallNornicDBCommand() *cobra.Command {
	installNornicDBCmd := &cobra.Command{
		Use:   "nornicdb",
		Short: "Install the local NornicDB binary",
		Long: strings.TrimSpace(`
Install a verified local NornicDB executable into Eshu's managed home.

Eshu currently tracks the latest NornicDB main branch. Build or download the
NornicDB binary you want to evaluate, then install it from that explicit
source:

  eshu install nornicdb --from /absolute/path/to/nornicdb-headless
  eshu install nornicdb --from /absolute/path/to/nornicdb-headless-darwin-arm64.tar.gz
  eshu install nornicdb --from /absolute/path/to/NornicDB-main-arm64-lite.pkg
  eshu install nornicdb --from https://example.com/releases/nornicdb-headless-darwin-arm64.tar.gz --sha256 <expected-sha256>
  eshu install nornicdb --from https://example.com/releases/NornicDB-main-arm64-lite.pkg --sha256 <expected-sha256>

The no-argument installer is intentionally unavailable while this policy is in
effect because Eshu is not pinning release assets yet. Headless remains the
default laptop artifact. Use --from with a verified full-binary artifact when
you need the larger full binary.
Remote HTTP and HTTPS sources require --sha256. Obtain the expected digest
independently from a trusted release channel; Eshu checks it before extracting
an archive or asking the candidate binary for its version. Local paths and
file:// sources may omit --sha256.
Signature verification is still future work.
`),
		RunE: runInstallNornicDB,
	}
	installNornicDBCmd.Flags().String("from", "", "Install from a local NornicDB binary, local archive/package, or release URL")
	installNornicDBCmd.Flags().String("sha256", "", "Expected SHA-256 for --from (required for HTTP/HTTPS sources)")
	installNornicDBCmd.Flags().Bool("force", false, "Replace an existing managed NornicDB binary")
	installNornicDBCmd.Flags().Bool("full", false, "Reserved for future no-argument release installs; use --from for full binary artifacts today")
	return installNornicDBCmd
}

// runInstallNornicDB is the `eshu install nornicdb` RunE. It resolves the
// cobra flags and the exit-code contract -- the parts that must stay in
// package main -- and delegates verification and the actual install to
// graphinstall.Install with the CLI's own version probe, so the
// destination never repoints at core implementation handles.
func runInstallNornicDB(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("eshu install nornicdb accepts flags only, got %d argument(s)", len(args))
	}
	from, err := cmd.Flags().GetString("from")
	if err != nil {
		return err
	}
	expectedSHA, err := cmd.Flags().GetString("sha256")
	if err != nil {
		return err
	}
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return err
	}
	full, err := cmd.Flags().GetBool("full")
	if err != nil {
		return err
	}
	if full && strings.TrimSpace(from) != "" {
		return errors.New("--full is reserved for future no-argument release installs; install full NornicDB binaries with --from")
	}

	result, err := graphinstall.Install(graphinstall.Options{
		Context:     cmd.Context(),
		From:        from,
		SHA256:      expectedSHA,
		Force:       force,
		Full:        full,
		ReadVersion: graphinstall.DefaultVersionReader,
	})
	if err != nil {
		return err
	}
	printJSON(result)
	return nil
}

func init() {
	rootCmd.AddCommand(newInstallCommand())
}
