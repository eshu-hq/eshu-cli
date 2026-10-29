// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"bytes"
	"strings"
	"testing"

	cliconfig "github.com/eshu-hq/eshu-cli/internal/cli/config"
	"github.com/eshu-hq/eshu/sdk/go/apicontract"
)

// validateTestRegistry builds the published client-table registry the thin
// command validates against.
func validateTestRegistry(t *testing.T) *apicontract.Registry {
	t.Helper()
	registry, err := apicontract.New(apicontract.ClientEntries())
	if err != nil {
		t.Fatalf("New(ClientEntries()) error = %v", err)
	}
	return registry
}

// TestConfigValidateOK pins the clean-environment report.
func TestConfigValidateOK(t *testing.T) {
	var stdout bytes.Buffer
	err := cliconfig.ValidateEnv(&stdout, validateTestRegistry(t), map[string]string{
		"ESHU_MCP_TRANSPORT": "http",
		"ESHU_QUERY_PROFILE": "production",
	}, false)
	if err != nil {
		t.Fatalf("ValidateEnv error = %v, want nil", err)
	}
	if want := "config validate: OK"; !strings.Contains(stdout.String(), want) {
		t.Fatalf("validate stdout = %q, want %q", stdout.String(), want)
	}
}

// TestConfigValidateRejectsBadEnum pins the error mapping: an invalid value
// for a known variable is an error finding and fails the check.
func TestConfigValidateRejectsBadEnum(t *testing.T) {
	var stdout bytes.Buffer
	err := cliconfig.ValidateEnv(&stdout, validateTestRegistry(t), map[string]string{
		"ESHU_MCP_TRANSPORT": "smtp",
	}, false)
	if err == nil || !strings.Contains(err.Error(), "configuration invalid") {
		t.Fatalf("ValidateEnv error = %v, want configuration invalid", err)
	}
	if !strings.Contains(stdout.String(), "ERROR ESHU_MCP_TRANSPORT=\"smtp\" is not one of") {
		t.Fatalf("validate stdout = %q, want enum refusal", stdout.String())
	}
}

// TestConfigValidateSuggestsTypos pins the typo hint: a near-miss of a
// registered name is reported even outside strict mode.
func TestConfigValidateSuggestsTypos(t *testing.T) {
	var stdout bytes.Buffer
	err := cliconfig.ValidateEnv(&stdout, validateTestRegistry(t), map[string]string{
		"ESHU_MCP_TRANSPOR": "http",
	}, false)
	if err != nil {
		t.Fatalf("ValidateEnv error = %v, want nil (typo is a warning)", err)
	}
	if !strings.Contains(stdout.String(), "did you mean ESHU_MCP_TRANSPORT?") {
		t.Fatalf("validate stdout = %q, want typo hint", stdout.String())
	}
}

// TestConfigValidateStrictFlagsUnknown pins strict mode: an unrecognized
// ESHU_* variable is silent by default and a finding under --strict.
func TestConfigValidateStrictFlagsUnknown(t *testing.T) {
	env := map[string]string{"ESHU_SITE_LOCAL_EXPERIMENT": "1"}
	var quiet bytes.Buffer
	if err := cliconfig.ValidateEnv(&quiet, validateTestRegistry(t), env, false); err != nil {
		t.Fatalf("ValidateEnv error = %v, want nil", err)
	}
	if !strings.Contains(quiet.String(), "config validate: OK") {
		t.Fatalf("non-strict stdout = %q, want OK", quiet.String())
	}
	var strictOut bytes.Buffer
	if err := cliconfig.ValidateEnv(&strictOut, validateTestRegistry(t), env, true); err != nil {
		t.Fatalf("ValidateEnv strict error = %v, want nil (unknown is a warning)", err)
	}
	if !strings.Contains(strictOut.String(), "WARN  ESHU_SITE_LOCAL_EXPERIMENT is not a registered variable") {
		t.Fatalf("strict stdout = %q, want unknown warning", strictOut.String())
	}
}

// TestConfigValidateReferenceRendersClientTable pins the --reference output:
// the published client table renders deterministically.
func TestConfigValidateReferenceRendersClientTable(t *testing.T) {
	first := renderConfigReference()
	second := renderConfigReference()
	if first != second || first == "" {
		t.Fatal("reference output is not deterministic")
	}
	for _, want := range []string{"ESHU_API_KEY", "ESHU_MCP_TRANSPORT", "ESHU_QUERY_PROFILE"} {
		if !strings.Contains(first, want) {
			t.Fatalf("reference output missing %s:\n%s", want, first)
		}
	}
}

// TestConfigCommandsRegistered pins the Tier-2 wiring: the config family
// minus the Tier-1 neo4j block, which stays where Tier-1 put it.
func TestConfigCommandsRegistered(t *testing.T) {
	for _, args := range [][]string{
		{"config"},
		{"config", "show"},
		{"config", "set"},
		{"config", "reset"},
		{"config", "db"},
		{"config", "validate"},
	} {
		t.Run(strings.Join(args, "-"), func(t *testing.T) {
			cmd, _, err := rootCmd.Find(args)
			if err != nil {
				t.Fatalf("find %v: %v", args, err)
			}
			if cmd == nil || cmd.Name() != args[len(args)-1] {
				t.Fatalf("command %v not registered", args)
			}
		})
	}
}
