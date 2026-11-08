# ADR-0001: Query path goes over HTTP only

## Status

Accepted. Recorded from the
[CLI contract](../contract/8874-eshu-cli-contract.md) and the
[command inventory](../contract/cli_extraction_inventory.md), which
classify every command by what it touches.

## Context

The CLI was extracted from the core binary. In the old home it could
reach server internals directly; as its own repo it must not. At the
same time, a terminal tool legitimately does a few local jobs
(installing a managed binary, saving settings, writing editor config),
so a blanket "HTTP for everything" claim would be false — and was, in
older wording.

## Decision

Query commands go through HTTP only. They never open the graph store,
a database table, or a queue, and the CLI holds no server storage
handles at all. Exactly four things act locally, by design:
`install nornicdb`, the `index` hand-off to `eshu-bootstrap-index`,
`config set`, and `mcp setup --write`. Removed and stub families call
nothing and point at the core binary instead.

## Consequences

- The boundary is auditable per command — see
  [Boundaries](../concepts/boundaries.md), which names the code behind
  each local path.
- Scripts can trust that a query command's only side effect is an HTTP
  call and printed JSON (plus an exit code).
- The cost: anything the API doesn't expose stays a stub until the
  core grows an endpoint. That's deliberate friction, not a bug.
