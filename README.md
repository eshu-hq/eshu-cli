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

## Open questions (NOT_CHECKED)

Carried from the source inventory and contract; each blocks the reviewed cutover, not this seed.

- Oldest/newest supported core/CLI version pairs for an independently released CLI, plus credential handling and vulnerability-response ownership for the extracted stream.
- Distribution of the demo Compose overlay and question manifest to an installed `eshu-cli` outside a checkout (embed, pinned fetch, or deploy-repo checkout), decided with `#8871`.
- Name, versioning, and discovery of the future core `local-host` entrypoint that would let the retained `graph`/`watch` verbs appear in `eshu-cli`.

Independent release, QA EKS deployment, rollback rehearsal, and production acceptance are also NOT_CHECKED.
PROBE-DOCS: append one line
