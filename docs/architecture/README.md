# Architecture: eshu-cli

## Place in Eshu

`eshu-cli` is the terminal face of Eshu. You (or a script) run a
command here; we translate it into an HTTP call to the Eshu API /
core, which alone holds the graph and durable stores; the reply comes
back as printed JSON. The repo [README](../../README.md) keeps the
one-screen version, and [Diagrams](../diagrams/README.md) draws the
topology — including the one wrinkle, the local setup paths.

Sibling repos: `eshu` holds the core, `eshu-api` is the HTTP front
door, `eshu-mcp` serves tools to AI assistants, and
[eshu-deploy](https://github.com/eshu-hq/eshu-deploy) runs the full
stack. Supported version pairs live in the
[CLI contract](../contract/8874-eshu-cli-contract.md).

## Components

| Component | Job |
| --- | --- |
| `cmd/eshu/` | The binary and its command tree (`main.go`, `root.go`, one file per family: `basic.go`, `find.go`, `trace.go`, `install.go`, `config.go`, `mcp.go`, `doctor.go`, …), plus the API client (`client.go`) and output shaping (`output.go`). |
| `internal/cli/` | Family support packages: saved config (`config/`), MCP client setup (`mcpsetup/`), diagnostics (`doctor/`), freshness, entity maps, redaction, and friends. |
| `internal/buildinfo/`, `internal/urlredact/` | Version stamping for releases, and keeping secrets out of printed URLs. |
| `testdata/` | Fixtures, including the parity goldens the release proof leans on. |

How a command travels through these parts is told in
[Request story](../concepts/request-story.md); what each family may
touch is in [Boundaries](../concepts/boundaries.md) and the
[command inventory](../contract/cli_extraction_inventory.md).

## Stack

- Go 1.26.10 (see `go.mod`), module `github.com/eshu-hq/eshu-cli`.
  Commands are built on Cobra (`github.com/spf13/cobra`); wire shapes
  come from the Eshu SDK contract modules pinned in `go.mod`.
- Delivery: versioned `eshu-linux-x86_64` / `eshu-macos-arm64`
  binaries with checksums and provenance, plus a distroless image
  carrying just the binary — see [Release](../release.md).

The load-bearing decision behind this shape — queries over HTTP only,
local writes only where named — is recorded in
[ADR-0001](../decisions/0001-query-path-http-only.md).
