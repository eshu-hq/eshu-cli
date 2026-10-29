// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package main runs the eshu-cli binary, the extracted user-facing CLI for
// Eshu (Refs eshu-hq/eshu#8874, part of eshu-hq/eshu#6707).
//
// The binary carries the Tier-1 population only: pure-remote and mechanical
// commands with small, core-free closures (analyze, find, query, list/ls,
// stats, index-status, change, map, trace, freshness, playbooks, ecosystem
// overview, completion, help, version, neo4j guidance, the index
// binary-discovery entrypoint, and the pre-existing removed-command STUBs).
// Every Tier-2/Tier-3 family ships as an E3 STUB that points at the core
// binary instead of guessing its behavior.
//
// Wire parity with the core CLI is the contract: the envelope MIME
// application/eshu.envelope+json, the shared exit table (0 success, 1
// generic, 2 invalid argument, 3 ambiguous, 4 index-building/stale, 5
// degraded/partial, 6 unsupported capability), and byte-identical help and
// --json output on Tier-1 families. The only sanctioned diffs are the E3
// stubs, the E4 index binary-discovery contract, the E5 per-family help
// deltas, and the absence of the RUNTIME launchers (which follow their
// runtime repos) recorded in the population PR.
package main
