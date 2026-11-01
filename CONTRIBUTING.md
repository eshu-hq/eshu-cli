# Contributing to eshu-cli

## CI in one minute

`make ci` is the local contract for the `lint`, `test`, and `race`
lanes. `.github/workflows/ci.yml` is a thin caller: its `lanes` job runs
the lane bodies from the pinned `eshu-hq/.github` `go-collector-ci.yml`
reusable (shown as `lanes / <lane>`), and `ci-complete (required
aggregator)` is the only context the repo ruleset requires. Do not
rename it. Tool pins, the coverage floor, and every lane gate are
inputs in `ci.yml`; `ci_workflow_test.go` pins them, and the floor must
equal `COVERAGE_FLOOR` in the `Makefile`. The accepted differences from
the old inline lanes are listed in the `ci.yml` header.

Push and PR runs pick lanes by changed paths (the `changes` job):

| PR changes | Lanes that run |
|---|---|
| Docs only (paths no filter class matches: `**/*.md`, `LICENSE`) | none; `ci-complete` succeeds |
| Go sources, `go.mod`, `go.sum` | `lint`, `test`, `race`, `gosec` |
| `go.mod`, `go.sum`, `.golangci.yml` | `govulncheck` plus the Go rows above |
| Fixtures or config tests read | `test` only |
| Shell scripts (`**/*.sh`) | `shell` only |
| Gate inputs (`scripts/coverage-gate.sh`, `Makefile`) | `race` (the script additionally matches `shell`) |
| `Dockerfile`, `.dockerignore` | `image` (lands with the file) |
| `.github/workflows/**` | `actionlint` plus `lint`, `test`, `race`, `shell`, `image` |
| Weekly schedule (Mondays 06:00 UTC) | `govulncheck`, `gosec` |

Two rules never change: no gate compares the tree against a committed
count, list, or artifact (the only exception is the 500-line cap on
`.go` files, enforced with plain `wc -l`), and lint runs on the whole
repo with no baseline. Any `.golangci.yml` exclusion needs a reason
and a linked open issue on the same lines.

Part of `eshu-hq/eshu-cli#1`.
