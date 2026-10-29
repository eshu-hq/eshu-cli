// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"fmt"
	"os"
	"strings"

	cliconfig "github.com/eshu-hq/eshu-cli/internal/cli/config"
	"github.com/eshu-hq/eshu/sdk/go/apicontract"
	"github.com/spf13/cobra"
)

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration settings",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Display current configuration settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings := cliconfig.Load()
			if len(settings) == 0 {
				fmt.Printf("No configuration found at %s\n", cliconfig.EnvFilePath())
				return nil
			}

			headers := []string{"Key", "Value"}
			var rows [][]string
			for k, v := range settings {
				rows = append(rows, []string{k, v})
			}
			printTable(headers, rows)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set one configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cliconfig.SetValue(args[0], args[1]); err != nil {
				return err
			}
			printSuccess(fmt.Sprintf("Set %s", args[0]))
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "reset",
		Short: "Reset all configuration to defaults",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Print("Reset all configuration to defaults? [y/N] ")
			var answer string
			if _, err := fmt.Scanln(&answer); err != nil {
				return err
			}
			if strings.ToLower(answer) != "y" {
				fmt.Println("Reset cancelled")
				return nil
			}
			return cliconfig.Reset()
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "db <backend>",
		Short: "Switch the default database backend",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			backend, err := cliconfig.ConfigureDatabaseBackend(args[0])
			if err != nil {
				return err
			}
			printSuccess(fmt.Sprintf("Default database switched to %s", backend))
			return nil
		},
	})

	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate ESHU_* environment variables against the central registry",
		Long: "Validate checks the current environment against the code-owned ESHU_*\n" +
			"registry. It reports invalid values for known variables (errors),\n" +
			"deprecated variables (warnings), and unknown variables that resemble a\n" +
			"known name (likely typos). Use --strict to also flag every unrecognized\n" +
			"ESHU_* variable, and --reference to print the generated reference doc.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runConfigValidate,
	}
	validateCmd.Flags().Bool("strict", false, "Treat every unrecognized ESHU_* variable as a finding")
	validateCmd.Flags().Bool("reference", false, "Print the generated environment-variable reference and exit")
	cmd.AddCommand(validateCmd)

	return cmd
}

func init() {
	rootCmd.AddCommand(newConfigCommand())
}

// runConfigValidate is the process-wiring half of `eshu config validate`: it
// reads the two cobra flags, resolves the command's output stream, and
// snapshots the real process environment. The check itself runs against the
// published apicontract client table, so the thin CLI validates exactly the
// variables it owns; the full platform table stays core-side.
func runConfigValidate(cmd *cobra.Command, _ []string) error {
	if reference, _ := cmd.Flags().GetBool("reference"); reference {
		_, _ = fmt.Fprint(cmd.OutOrStdout(), renderConfigReference())
		return nil
	}

	registry, err := apicontract.New(apicontract.ClientEntries())
	if err != nil {
		return err
	}
	strict, _ := cmd.Flags().GetBool("strict")
	return cliconfig.ValidateEnv(cmd.OutOrStdout(), registry, cliconfig.EnvironMap(os.Environ()), strict)
}

// renderConfigReference prints the published client variable table in
// Registry.Entries order (subsystem, then name), the same order the registry
// round-trips through.
func renderConfigReference() string {
	registry, err := apicontract.New(apicontract.ClientEntries())
	if err != nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("# ESHU_* environment reference (thin CLI client table)\n\n")
	for _, entry := range registry.Entries() {
		fmt.Fprintf(&b, "## %s\n\n", entry.Name)
		fmt.Fprintf(&b, "- Type: %s\n", entry.Type)
		if entry.Default != "" {
			fmt.Fprintf(&b, "- Default: %s\n", entry.Default)
		}
		if len(entry.Allowed) > 0 {
			fmt.Fprintf(&b, "- Allowed: %s\n", strings.Join(entry.Allowed, ", "))
		}
		if len(entry.Aliases) > 0 {
			fmt.Fprintf(&b, "- Aliases: %s\n", strings.Join(entry.Aliases, ", "))
		}
		if entry.Deprecated {
			fmt.Fprintf(&b, "- Deprecated: use %s\n", entry.ReplacedBy)
		}
		fmt.Fprintf(&b, "- Subsystem: %s\n\n", entry.Subsystem)
		fmt.Fprintf(&b, "%s\n\n", entry.Description)
	}
	return b.String()
}
