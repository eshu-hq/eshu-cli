# eshu-cli

**Eshu CLI user commands and API clients, seeded as a preparation-only contract surface. Pre-release: holds no cutover authority until its reviewed cutover.**

Refs `eshu-hq/eshu#8874`, part of `eshu-hq/eshu#6707`.

## What lives here

Day-one seed only: the versioned contract `eshu-cli` will publish, copied verbatim from `eshu` so the destination starts byte-identical to the reviewed source.

- `docs/contract/8874-eshu-cli-contract.md`: supported core/CLI version basis, credential handling, demo asset split, and the `local-host` entrypoint boundary.
- `docs/contract/cli_extraction_inventory.md`: command/flag/env/output/exit census classifying every visible command as EXTRACT, RETAIN, RUNTIME, or STUB.
- `docs/contract/PROVENANCE.md`: source SHA, per-file hashes, what stays in `eshu`, and the pinning tests that guard the contract.
- `scripts/check-parity.sh` plus the `parity` CI lane: re-verify the verbatim files against `docs/contract/PARITY.sha256`.

## Tier-1 population (S3)

The first population carries the Tier-1 set from the population-scope
verdict (`eshu-hq/eshu`
`docs/internal/evidence/8874-cli-population-scope-verdict.md`): the
pure-remote/mechanical commands `analyze`, `find`, `query`, `list`/`ls`,
`stats`, `index-status`, `change`, `map`, `trace`, `freshness`,
`playbooks`, `ecosystem overview`, `completion`, `help`, `version`,
`neo4j` guidance, the `index` binary-discovery entrypoint, and the
pre-existing removed-command STUBs — plus E3 STUBs for every Tier-2/Tier-3
family, all pointing at the core binary.

- `cmd/eshu/`: the Tier-1 cobra tree (`doc.go` describes the scope).
- `internal/`: the small-closure libraries the tree owns (`apierr`,
  `trace`, `change`, `entitymap`, `freshness`, `playbooks`, `config`,
  `reposelector`, `buildinfo`; `envregistry` was retired in Tier-2),
  pinned to the core commit in `docs/tier1-pin.md` under the
  moved-package freeze.
- Proof: `scripts/check-census.sh` (closures, pins, credentials),
  `scripts/check-nofork.sh` (freeze), `scripts/check-tier1-parity.sh`
  with `testdata/parity/goldens/` (help bytes, `--json` envelopes,
  exit codes 0-6), and `TestCredentialProfileRoundTrip` plus the
  redaction canaries in `go test ./...`.

In-tree removal in `eshu` still waits for production acceptance.

## Tier-2 population

The second population carries the Tier-2 set from the same verdict: thin
clients for `report` (digest plus capture/validate through the S4
endpoints), `service-report` (compose), `answer-quality-scorecard`
(score), `config` (validate against the published client table),
`component` inventory/diagnostics, and `mcp`/`m` setup (remote tools/list
decoded via `apicontract.ToolDefinition`). `investigation export` stays
stubbed: its packet build is truth assembly with no build endpoint.
The published `sdk/go/apicontract` module is pinned in `go.mod`
(`docs/tier2-pin.md`; the `v0.1.0` tag is pending, so the pin is the
toolchain-derived pseudo-version at the Tier-2 core commit, never an
invented version). New verbatim copies and their recorded deltas live in
`docs/tier2-pin.md`; S4 fixtures and the extended parity cases live under
`testdata/parity/`.

## Open questions (decided)

Decided by the arbiter verdict recorded at
`eshu-hq/eshu` `docs/internal/evidence/8874-cli-open-questions-verdict.md`
(landed via `eshu-hq/eshu#9504`); each decision blocks the reviewed
cutover, not this seed.

- Version pairs and credentials: one tested pair at the first
  independent release, recorded only in the eshu-deploy compatibility
  matrix; no runtime version enforcement on day one; the
  `~/.eshu/.env` (0600) filesystem contract moves with the CLI.
  Vulnerability-response ownership (`SECURITY.md` here) is named
  before the first independent release.
- Demo distribution: the question manifest embeds in `eshu-cli`
  (`//go:embed`, on-disk file wins, CI byte-diff against the core
  spec); the Compose overlay stays a deploy-repo checkout owned by
  `#8871`.
- `local-host`: `eshu local-host` in the core binary, protocol
  `local-host/v1`, with a redacted v1 discovery record (never
  `OwnerRecord`); `graph`/`watch`/`w` stay RETAIN until v1 ships,
  then STUB entries (exit 1) are the sanctioned parity exception
  alongside the demo miss-text update.

Independent release, QA EKS deployment, rollback rehearsal, and production acceptance are also NOT_CHECKED.
