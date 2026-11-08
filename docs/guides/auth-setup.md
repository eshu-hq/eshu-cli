# Set up credentials for eshu-cli

Your API key, when set, travels as `Authorization: Bearer <key>` and
nowhere else. This guide sets it (and the service URL) the way that
suits how you work. Exact names and defaults are in
[Reference: configuration](../reference/configuration.md).

## Step 1: know the resolution order

Settings resolve in a fixed order, and once you know it, most "why is
it talking to the wrong service?" mysteries solve themselves:

1. Flags (`--service-url`, `--api-key` where a command takes them)
2. Saved config in `~/.eshu/.env`
3. Env vars (`ESHU_SERVICE_URL`, `ESHU_API_KEY`)
4. The built-in default (`http://localhost:8080`)

Yes, saved config beats env. The comment in the code says otherwise;
the code wins. Every time.

## Step 2: pick where your settings live

Quickest for a shell session:

```bash
export ESHU_SERVICE_URL=http://localhost:8080
export ESHU_API_KEY='change-me'
```

Better for everyday use — saved settings, managed by the CLI itself
and written `0600` because API keys end up there:

```bash
eshu config set ESHU_SERVICE_URL http://localhost:8080
eshu config set ESHU_API_KEY 'change-me'
eshu config show
eshu list
```

One caution: `--api-key` on the command line can show up in shell
history and process listings. In scripts, prefer the env var or saved
config.

## Step 3 (optional): use profiles for several setups

With `--profile staging`, lookups try `ESHU_SERVICE_URL_STAGING`
before `ESHU_SERVICE_URL` (the profile name is upper-cased). An empty
profile value falls through to the shared one, so you can leave a
profile entry blank without blanking your default.

## Step 4 (optional): wire up an MCP client

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
raw secret. Keep it that way when you edit them by hand.

Key set but still refused? See [Troubleshoot](troubleshoot.md).
