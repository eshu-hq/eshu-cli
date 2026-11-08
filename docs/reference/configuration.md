# Reference: configuring eshu-cli

Settings resolve in a fixed order: flags first, then your saved config
in `~/.eshu/.env`, then env vars, then the built-in default. (The
comment at the top of `NewAPIClient` says "flags to env to config",
but the code checks saved config before env. The code wins. Every
time.) For a walkthrough, see [Auth setup](../guides/auth-setup.md).

| Setting | Required | Default | What it does |
| --- | --- | --- | --- |
| `--database` (global) | No | empty | Temporarily override the database backend for this run |
| `--visual` / `-V` (global) | No | `false` | Show results as an interactive graph where supported |
| `--version` / `-v` | No | `false` | Print the version and exit |
| `--service-url` (some commands) | No | from config/env | API service URL for this run |
| `--api-key` (some commands) | No | from config/env | API key for this run |
| `--profile` (some commands) | No | empty | Named config profile, for example `staging` |
| `ESHU_SERVICE_URL` (env) | No | `http://localhost:8080` | Base URL of the Eshu service |
| `ESHU_API_KEY` (env) | No | empty | Bearer token, sent only when set |
| `ESHU_REMOTE_TIMEOUT_SECONDS` (env) | No | `30` | HTTP timeout in seconds |
| `ESHU_HOME` (env) | No | `~/.eshu` | Config directory. A leading `~` expands |

Per-command flags vary a lot, so treat `eshu <command> --help` as the
authority and this table as the map. A couple worth calling out:
`eshu index` takes `--force` / `-f` and `--discovery-report <path>`,
and `eshu install nornicdb` takes `--from`, `--sha256` (required for
HTTP sources), `--force`, and `--full` (reserved; it errors today and
tells you to use `--from`).

## Saved settings and profiles

Saved settings live in `~/.eshu/.env`, written `0600` because API keys
end up there. You manage them with the CLI itself:

```bash
eshu config set ESHU_SERVICE_URL http://localhost:8080
eshu config set ESHU_API_KEY 'change-me'
eshu config show
eshu list
```

Profiles let one file serve several setups. With `--profile staging`,
lookups try `ESHU_SERVICE_URL_STAGING` before `ESHU_SERVICE_URL` (the
profile name is upper-cased). An empty profile value falls through to
the shared one, so you can leave a profile entry blank without
blanking your default. One caution: `--api-key` on the command line
can show up in shell history and process listings. In scripts, prefer
the env var or saved config.

## Setting up an MCP client

`eshu mcp setup` (shortcut: `eshu m`) generates config for AI editors
and other MCP clients. MCP is the Model Context Protocol, the standard
way assistants call tools. The flags you'll actually use:

- `--platform` picks the client (default `generic`).
- `--hosted` generates hosted HTTP setup pointing at
  `<service>/mcp/message`. Without it you get a local stdio snippet.
- `--auth` chooses the hosted credential story: `auto` (default),
  `sso`, `token`, or `shared-key`. `--shared-key` forces the legacy
  shared key and is meant for admin or dev use.
- `--write` merges the snippet into the platform's config file instead
  of printing it. `--target` overrides which file.
- `--verify` runs staged checks: config, reachable, tools, first query.

Snippets always use an env-var reference for the credential, never the
raw secret. Keep it that way when you edit them by hand. Which of
these touch local files (and which never do) is drawn in
[Boundaries](../concepts/boundaries.md).

## Exit codes

Scripts can rely on these. Envelope error codes map onto exit codes in
`ExitCodeForErrorCode`, and a parity test keeps the copies of that
table from drifting apart.

| Code | When you get it |
| --- | --- |
| 0 | Success |
| 1 | Generic failure, and the default for any error code we don't recognise |
| 2 | `invalid_argument`, `not_found`, or `scope_not_found` |
| 3 | `ambiguous` |
| 4 | `index_building` or `stale` |
| 5 | `capability_degraded` or `partial` |
| 6 | `unsupported_capability` |

One wrinkle: freshness states like `building` or `stale` reported in
output aren't error codes. The freshness family prints the state and
still exits `0`. Commands like `trace`, `change impact`, and `map` do
exit `4` on a non-fresh state. If a pipeline branches oddly, check
which family produced the code before you blame the table.
