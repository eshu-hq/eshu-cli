# Contributing to eshu-cli

## CI in one minute

`make ci` is the whole local contract. It runs the `lint`, `test`, and
`race` lane commands from `.github/workflows/ci.yml`. Go lanes skip
with success while the repo holds no `go.mod`.

Push and PR runs pick lanes by changed paths (the `changes` job):

| PR changes | Lanes that run |
|---|---|
| Docs only (`**/*.md`, `LICENSE`) | none; `ci-complete` succeeds |
| Go sources, `go.mod`, `go.sum` | `lint`, `test`, `race` |
| Fixtures or config tests read | `test` only |
| Shell scripts (`**/*.sh`) | `shell` only |
| `Dockerfile`, `.dockerignore` | `image` (lands with the file) |
| `.github/workflows/**` | `workflow-lint` plus `lint`, `test`, `race` |

Two rules never change: no gate compares the tree against a committed
count, list, or artifact (the only exception is the 500-line cap on
`.go` files, enforced with plain `wc -l`), and lint runs on the whole
repo with no baseline. Any `.golangci.yml` exclusion needs a reason
and a linked open issue on the same lines.

Part of `eshu-hq/eshu-cli#1`.
