# eshu-cli

**Eshu CLI user commands and API clients, seeded as a preparation-only contract surface. Pre-release: holds no cutover authority until its reviewed cutover.**

Refs `eshu-hq/eshu#8874`, part of `eshu-hq/eshu#6707`.

## What lives here

Day-one seed only: the versioned contract `eshu-cli` will publish, copied verbatim from `eshu` so the destination starts byte-identical to the reviewed source.

- `docs/contract/8874-eshu-cli-contract.md`: supported core/CLI version basis, credential handling, demo asset split, and the `local-host` entrypoint boundary.
- `docs/contract/cli_extraction_inventory.md`: command/flag/env/output/exit census classifying every visible command as EXTRACT, RETAIN, RUNTIME, or STUB.
- `docs/contract/PROVENANCE.md`: source SHA, per-file hashes, what stays in `eshu`, and the pinning tests that guard the contract.
- `scripts/check-parity.sh` plus the `parity` CI lane: re-verify the verbatim files against `docs/contract/PARITY.sha256`.

No Go code moves in this seed. The cobra tree, `go/internal/cli/*`, and the env registry stay in `eshu` until the cutover PR proves parity and records production acceptance.

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
