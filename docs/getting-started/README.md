# Getting started with eshu-cli

Fastest path from nothing to a working `eshu` command. You'll need Go
1.26.10 (see `go.mod`) and an Eshu service to talk to.

## 1. Build it

```bash
go build -o /tmp/eshu ./cmd/eshu
/tmp/eshu version
```

`eshu version` prints one line, `Eshu <version>` (source builds report
`dev`). If that works, the binary runs.

## 2. Point it at your service

```bash
export ESHU_SERVICE_URL=http://localhost:8080
export ESHU_API_KEY='change-me'
```

Prefer saved settings? `eshu config set` writes them to
`~/.eshu/.env` — [Auth setup](../guides/auth-setup.md) covers both
routes and the order they win in.

## 3. Make your first call

```bash
/tmp/eshu list
```

That calls `GET /api/v0/repositories` and prints the reply, so a JSON
response means you're connected. Run `/tmp/eshu --help` any time you
want the full command list, and `eshu <command> --help` for any
command's flags — that help is generated from the code, so trust it
over any doc, including this one.

## Where next?

- [Operate](../guides/operate.md) — install options and the three
  checks that prove it works.
- [Concepts](../concepts/README.md) — which commands only ever talk
  HTTP, and which few touch local files.
- [Troubleshoot](../guides/troubleshoot.md) — `request failed`, `401`s,
  and `STUB:` messages explained.
