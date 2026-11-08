# Operate eshu-cli day to day

This is a client tool, so there's no server to babysit and no health
endpoint to scrape. You install the binary where you work and run it
when you need it. "Operate" here mostly means: get it installed, then
prove it can reach your service.

## Step 1: get the binary

From source, with Go 1.26.10:

```bash
go build -o /tmp/eshu ./cmd/eshu
```

Or from the image in this repo, which ships just the binary on a
distroless base with no shell:

```bash
docker build -t eshu-cli:dev .
docker run --rm eshu-cli:dev --help
```

Tagged releases publish `eshu-linux-x86_64` and `eshu-macos-arm64`
binaries. [Release](../release.md) covers downloads and checksum
checks. If you need the server side too, use
[eshu-deploy](https://github.com/eshu-hq/eshu-deploy). This repo ships
only the client.

## Step 2: prove it works

Three quick checks, in order:

```bash
eshu version   # the binary runs; prints "Eshu <version>"
eshu doctor    # diagnostics for health and config
eshu list      # the service is reachable and accepts your key
```

If `version` prints and `list` answers with JSON, you're set. Anything
less, head to [Troubleshoot](troubleshoot.md). If your key or service
URL isn't set yet, [Auth setup](auth-setup.md) comes first.

## Releases

`v0.1.0` was the first versioned release, carrying the Tier 1/2/3
command populations plus `install nornicdb`. Tags are
maintainer-controlled, and [Release](../release.md) spells out the
exact cutting steps and how to verify what you downloaded.
