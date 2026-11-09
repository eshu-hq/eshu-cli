# One command's story

Most of what this tool does is translation. You speak in commands, the
Eshu service speaks HTTP, and we sit in the middle. Here's one trip
end to end; the drawn version is in
[Diagrams](../diagrams/README.md).

## The trip

1. You run `eshu list`.
2. We work out where your service lives and which key to send: flags,
   then saved config, then env, then the default (see
   [Auth setup](../guides/auth-setup.md)).
3. We make one HTTP call — `GET /api/v0/repositories` for `list`.
4. We print what comes back as JSON. If the service answers with an
   envelope error, the error code decides the process exit code, so
   your scripts can branch on "index still building" separately from
   "you typed that wrong".
   [Reference: configuration](../reference/configuration.md) lists
   every code.

## It repeats across families

`eshu list` (and `eshu ls`) reads the repository list.
`eshu index-status` reads the latest indexing status.
`eshu query <query>` posts a language query. `eshu stats [path]` reads
indexing statistics. The `analyze`, `find`, `change`, `map`, `trace`,
`freshness`, `playbooks`, `report`, and `component` families all work
the same way. Where a command uses Eshu's envelope shape, it asks for
`application/eshu.envelope+json` explicitly.

Run `eshu --help` for the full list and `eshu <command> --help` for
any command's flags. That help output is generated from the code, so
trust it over any doc, including this one.

## What goes over the wire

Your API key, when set, travels as `Authorization: Bearer <key>` and
nowhere else. Requests time out after 30 seconds unless you change
`ESHU_REMOTE_TIMEOUT_SECONDS`. Replies print as JSON, and transport
trouble surfaces as messages like `request failed` or `API error 401`
rather than a stack trace. [Troubleshoot](../guides/troubleshoot.md)
covers what to do about each one.

And the commands that don't take this trip at all — the four local
setup paths, plus removed and stub families — are drawn exactly in
[Boundaries](boundaries.md).
