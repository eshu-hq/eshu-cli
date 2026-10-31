# First versioned release (Refs eshu-hq/eshu#9738)

`v0.1.0` is the first versioned `eshu-cli` release: the first artifact the
install/upgrade proof in `eshu-hq/eshu#9671` can install. It carries the
Tier-1/2/3 populations plus `install nornicdb` on the `v0.1.x` line, matching
the SDK modules' initial-version convention. Part of `eshu-hq/eshu#6707`,
Refs `eshu-hq/eshu#8874`.

## Cutting the release (maintainer)

Tags are maintainer-controlled; the PR that prepares a release states the
exact commands and a maintainer runs them after merge:

```bash
git fetch origin
git checkout main
git pull --ff-only
git rev-parse HEAD          # confirm the intended source commit
git tag v0.1.0 <sha>
git push origin v0.1.0
```

The tag push starts `.github/workflows/release.yml`, which builds
`eshu-linux-x86_64` (ubuntu-24.04) and `eshu-macos-arm64` (macos-latest)
with `buildinfo.Version` stamped to the tag, records `govulncheck ./...`,
and publishes the GitHub Release with SLSA build provenance plus an SPDX
SBOM attestation. Asset names match the core `eshu` release assets on
purpose: the satellite binary is a drop-in upgrade for the core-shipped
binary. Branch pushes never publish.

## Verifying a published release

```bash
TAG=v0.1.0
gh release download "$TAG" --repo eshu-hq/eshu-cli --dir /tmp/eshu-cli-release
(cd /tmp/eshu-cli-release && sha256sum -c SHA256SUMS.txt)
gh attestation verify /tmp/eshu-cli-release/eshu-linux-x86_64 -R eshu-hq/eshu-cli
```

`govulncheck.txt` on the release names the scan the artifact passed.

## Install and upgrade proof

Clean install on a supported platform (linux-x86_64 shown):

```bash
/tmp/eshu-cli-release/eshu-linux-x86_64 version   # Eshu v0.1.0
/tmp/eshu-cli-release/eshu-linux-x86_64 --help
/tmp/eshu-cli-release/eshu-linux-x86_64 install nornicdb --help
```

Upgrade from the core-shipped binary: put the satellite earlier in `PATH`
(or replace the core binary in place; both are a single `eshu` binary with
the same Tier-1 surface — see the parity goldens under
`testdata/parity/goldens/`). Rollback is the reverse: reinstall the prior
core binary; the CLI keeps no live state (`eshu-hq/eshu#9671` records this
disposition).

## Evidence

- Source commit, tag, and workflow run: linked from the release page.
- Provenance: `gh attestation verify` output (see above).
- Vulnerability scan: `govulncheck.txt` attached to the release.
- Install/upgrade runs: recorded on `eshu-hq/eshu#9671`.
