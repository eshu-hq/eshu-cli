// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"github.com/spf13/cobra"
)

// stubGuidance is the single redirect every E3 STUB prints: the family stays
// in the core eshu binary until its Tier-2 wire contract (S2) or core
// endpoint (S4) lands, so an installed-CLI user gets a supported-contract
// pointer instead of cobra's "unknown command".
const stubGuidance = "This family stays in the core eshu binary until its " +
	"Tier-2 contract or core endpoint lands. Run the core binary instead. " +
	"Refs eshu-hq/eshu#8874."

// tierStubFamilies is the E3 set: every Tier-2/Tier-3 family ships in the
// first population as a STUB via the existing removedCommandError pattern
// (exit 1 through the uncoded-error path, pointer to the core binary).
// The population-scope verdict enumerates Tier-1 membership; anything not
// Tier-1 is here, including the RETAIN graph/watch/w/admin families per
// the local-host decision's STUB class.
var tierStubFamilies = []struct {
	use   string
	short string
}{
	{"investigation", "STUB: evidence-packet export stays in core"},
	{"scan", "STUB: local-owner scan stays in core"},
	{"vuln-scan", "STUB: vulnerability evidence client stays in core"},
	{"docs", "STUB: docs verification stays in core"},
	{"demo", "STUB: demo lifecycle stays in core"},
	{"demo-benchmark", "STUB: demo scorer stays in core"},
	{"first-run", "STUB: guided onboarding stays in core"},
	{"first-run-benchmark", "STUB: first-run scorer stays in core"},
	{"hosted-setup", "STUB: first-connection flow stays in core"},
	{"hosted-onboard", "STUB: team onboarding stays in core"},
	{"assistant", "STUB: project guidance stays in core"},
	{"competitive-parity", "STUB: shipped-surface validator stays in core"},
	{"evidence", "STUB: portable bundle client stays in core"},
	{"evidence-packet-dogfood", "STUB: packet benchmark stays in core"},
	{"workspace", "STUB: workspace operations stay in core"},
	{"graph", "STUB: sidecar lifecycle stays in core"},
	{"watch", "STUB: watcher lifecycle stays in core"},
	{"w", "STUB: shortcut for watcher lifecycle stays in core"},
	{"admin", "STUB: core-only repair stays in core"},
	{"doctor", "STUB: diagnostics reporter stays in core"},
	{"install", "STUB: backend tooling install stays in core"},
}

func init() {
	for _, s := range tierStubFamilies {
		s := s
		rootCmd.AddCommand(&cobra.Command{
			Use:   s.use,
			Short: s.short,
			RunE: func(cmd *cobra.Command, args []string) error {
				return removedCommandError("eshu "+cmd.Name(), stubGuidance)
			},
		})
	}
}
