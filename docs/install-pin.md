# Install population pin (Refs eshu-hq/eshu#8874)

Fourth-population slice: `install nornicdb` lands as destination-authored
code pinned to the install contract, not as a moved copy. The core package
(`eshu/go/internal/cli/graphinstall`) closes over the `query` root for one
backend-name constant and over `eshulocal` for home resolution; both
packages fail the destination census by construction, so a byte-identical
move is impossible. The #9484 contract record ("Install filesystem and
source contract") plus the parity gates below are the freeze instead:
user-visible bytes match the core binary exactly, and the census plus the
no-fork oracle keep the destination free of core implementation handles.

`graph upgrade` accepts the same `--from`/`--sha256` flags but stays in core
under the census RETAIN `graph` row (sidecar lifecycle); it does not move in
this cutover.

## Module pin

- Core behavior basis: `78c5b3d1a4b7f5309a4df6eed4487162634c4010` (eshu
  origin/main, the Tier-2 pin). Help bytes and the no-source guidance error
  are byte-identical between the pin binary and this CLI.
- `github.com/eshu-hq/eshu-sdk/wirecontract v0.1.0-rc.1` (unchanged: the
  install path performs no wire calls and takes no SDK dependency).
- No `replace`, no `go.work`.

## Destination-authored files (parity goldens, not the freeze)

- `cmd/eshu/install.go`: `install` parent plus `nornicdb` child with
  core-identical Use/Short/Long/flags. The one intentional structural
  difference is a `newInstallNornicDBCommand` constructor (fresh flag set
  per call); help and behavior bytes are unaffected.
- `internal/cli/graphinstall/`: the install pipeline in four files plus
  the embedded empty release manifest:
  - `home.go`: managed-home resolution (`ESHU_HOME` verbatim with leading
    `~` expansion, else per-platform default) implementing the filesystem
    seam the contract record pins.
  - `source.go`: source classification, fail-closed `--sha256` policy,
    download, archive/package extraction, version probe staging.
  - `pipeline.go`: `Install`/`Result`/manifest shapes with the stable
    JSON tags the CLI prints, reuse/exists/force semantics, atomic
    binary and manifest writes.
  - `version.go` + `manifest.go`: `<binary> version` probe parsing the
    `NornicDB <version>` protocol and the fail-closed empty-manifest
    guidance error.
- `cmd/eshu/install_test.go`, `internal/cli/graphinstall/*_test.go`:
  unit proof for the contract table (home branches, sha policy, version
  protocol, reuse/exists/force, manifest shape, download mismatch,
  archive allowlist, darwin-only packages).

## Contract figures this pin enforces

- Managed binary `<home>/bin/nornicdb-headless`; manifest
  `<home>/graph-backends/nornicdb/manifest.json` (mode 0600).
- Fail-closed: remote `http`/`https` without `--sha256` refuses with
  `remote nornicdb sources require --sha256`; checksum mismatch refuses
  with `sha256 mismatch`; `--full` with `--from` refuses with the
  reservation text; empty `--from` prints the no-asset guidance naming
  the CLI version, platform, and `--from`.
- Version probe `<binary> version` accepting only the `NornicDB <version>`
  line; anything else fails closed before any install.
- Existing managed binary: same version and checksum reuses without
  rewrite; different content refuses naming `--force`; `--force` replaces.

## Census

- `internal/cli/graphinstall` closes over 2 internal packages (itself +
  `buildinfo` for the manifest guidance version); no `query`, `mcp`,
  `serviceintel`, `doctruth`, `component`, `storage`, `eshulocal`, or
  `localsupervisor` anywhere in the destination closure; zero core
  `go/internal` imports. Pinned by `scripts/check-census.sh`.
- `internal/cli/graphinstall/*`, `cmd/eshu/install.go`, and
  `cmd/eshu/install_test.go` are excluded from the no-fork oracle as
  destination-authored files, exactly like the Tier-2/3 thin commands.

## Parity extension

- New core-compared cases: `install` + `install nornicdb` help; install
  from a staged fake binary (fixed `NornicDB 1.0.0-parity` version line),
  reuse on repeat, exists-different without `--force`, `--force`
  replacement, no-source guidance, remote-without-sha refusal,
  `--full`-with-`--from` refusal, flags-only refusal. `ESHU_HOME` and the
  `installed_at` stamp are canonicalized (`canon_install`).
- Dest-pinned: root help loses the install STUB line (one-line golden
  change); the `cli-stub-install` goldens are deleted with the stub.
- Mutation proof: disabling the checksum comparison fails the
  mismatch test; restoring it passes.
