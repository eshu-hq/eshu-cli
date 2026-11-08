# Contributing to eshu-cli

Thanks for helping. This page is the friendly front porch; the binding
rules live in the root [CONTRIBUTING.md](../../CONTRIBUTING.md). If the
two ever disagree, the root file wins.

## Setup

You'll need Go 1.26.10 (see `go.mod`). Then:

```bash
go build -o /tmp/eshu ./cmd/eshu
make ci
```

## Make targets

`make ci` is the local contract for the `lint`, `test`, and `race`
lanes (`make lint` includes the 500-line `filecap` check; `make race`
also enforces the coverage floor). Run it before every push. CI picks
lanes by changed paths — the root
[CONTRIBUTING.md](../../CONTRIBUTING.md) maps paths to lanes — and
`bash scripts/cli-seed-matrix.sh` proves each lane can go red and
green locally.

## PR rules that don't bend

- Go files stay at or under 500 lines.
- No gate compares the tree against a committed count, list, or
  artifact (the 500-line cap is the only exception).
- Lint runs on the whole repo with no baseline. Any `.golangci.yml`
  exclusion needs a reason and a linked open issue on the same lines.
- The coverage floor (`COVERAGE_FLOOR` in the `Makefile`, equal to the
  `ci.yml` input — a test fails when they differ) may only rise.
- Don't rename the `ci-complete (required aggregator)` check; the
  ruleset requires that exact context.

## Security and license

See [SECURITY.md](../../SECURITY.md) to report a vulnerability
(please don't use a public issue). There's no `LICENSE` file; sources
carry `SPDX-License-Identifier: MIT`. And please never commit
credentials or `.env` files.
