# Boundaries: HTTP for queries, local disk only where named

Older wording in this repo said the CLI goes through HTTP for
everything. That's true for queries and false as a blanket claim, so
here's the honest version — this page is its home.

## The rule

**Query commands go through HTTP only.** They never open the graph
store, a database table, or a queue — and neither does anything else
here. We hold no server storage handles at all. The picture is in
[Diagrams](../diagrams/README.md).

## We own

- The commands and their API clients: turning `eshu list`, `eshu find`,
  `eshu trace`, and friends into HTTP calls, then printing JSON.
- Command output, flags, and exit codes.
- The local setup surface listed below — and nothing beyond it.

## The four things that act on your local machine

By design, and only these:

- `eshu install nornicdb` verifies a NornicDB binary you point it at
  (with `--from`, plus `--sha256` for downloads) and copies it into
  Eshu's managed home. See `cmd/eshu/install.go`.
- `eshu index [path]` doesn't index anything itself. It finds
  `eshu-bootstrap-index` in your `PATH` and hands over to it with
  `syscall.Exec`, passing `--force` and an optional discovery-report
  path through. No bootstrap binary, no indexing. That's `runIndex`
  in `cmd/eshu/basic.go`.
- `eshu config set <key> <value>` writes to `~/.eshu/.env` (mode
  `0600`, because it holds API keys). `config show` reads it back and
  `config reset` clears it. The write lives in
  `internal/cli/config/env.go`.
- `eshu mcp setup` prints an MCP client snippet by default. Add
  `--write` and it merges that snippet into your editor's config file
  instead, atomically, via `internal/cli/mcpsetup/write.go`. Hosted
  snippets point at `<service>/mcp/message`; local ones describe a
  stdio launch. Either way the snippet references an env var, never
  your raw secret. Note that `eshu mcp start` itself just redirects
  you to the core binary. Server launch didn't move here.

## We never touch

- The graph, the server's tables, or its queues. Storing the graph and
  running the server stay in the core (or `eshu-api` for the HTTP
  front door).
- Admin repair work. Two families never call the API at all: removed
  commands (say, `delete` or `clean`) print what to use instead, and
  stub commands, marked `STUB:` in `--help`, mean the work still
  lives in the core binary. Follow the message rather than guessing
  around it.

The [command inventory](../contract/cli_extraction_inventory.md)
classifies every command if you want the complete picture, and the
decision behind this line is recorded in
[ADR-0001](../decisions/0001-query-path-http-only.md).
