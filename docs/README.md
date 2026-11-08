# eshu-cli docs

Welcome in. `eshu-cli` is how you talk to Eshu from a terminal: you run
a command, we call the Eshu API, and we print JSON you can read or pipe
onward. New here? Start with
[Getting started](getting-started/README.md).

## Start here

| Page | Read it when… |
| --- | --- |
| [Getting started](getting-started/README.md) | You want the binary built and your first command answered |
| [Guides](guides/README.md) | You need step-by-step help: daily use, credentials and profiles, troubleshooting |
| [Concepts](concepts/README.md) | You want the mental model — above all, which commands stay on HTTP and which touch local files |
| [Reference](reference/README.md) | You need an exact flag, env var, profile rule, or exit code |
| [Architecture](architecture/README.md) | You want our place in Eshu, the components, and the stack |
| [Diagrams](diagrams/README.md) | You think in pictures (every diagram lives here, with a text version) |
| [Decisions](decisions/README.md) | You want the recorded architecture decisions (ADRs) |
| [Contributing](contributing/README.md) | You're changing code and want setup, `make`, and PR rules |

## Standing references in this folder

These pages predate the tree above and stay where they are:

- [Release](release.md) — versioned binaries, checksums, and upgrades.
- [CLI contract](contract/8874-eshu-cli-contract.md) — supported
  version pairs, credentials, and boundaries.
- [Command inventory](contract/cli_extraction_inventory.md) — every
  command, flag, and exit behaviour, classified.

## Maintainer history

Extraction records, kept for provenance rather than day-to-day reading:

- [Provenance](contract/PROVENANCE.md) and `contract/PARITY.sha256`.
- Pin notes: [install-pin.md](install-pin.md),
  [tier1-pin.md](tier1-pin.md), [tier2-pin.md](tier2-pin.md), and
  [tier3-pin.md](tier3-pin.md) — source pins for each tier.

The repo [README](../README.md) has the one-screen version.
