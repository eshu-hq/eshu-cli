# Tier-2 population pin (Refs eshu-hq/eshu#8874)

Owner approval for Tier-2 CLI population recorded 2026-10-29 on the eshu
release-prep commit (`drive/8874-tier2-population`: "Tier-2 owner-approved
2026-10-29"). The Tier-2 wire import first pinned the closest shippable
state, the Tier-2 pin below; eshu-hq/eshu#9631 repointed it at the published
tag, leaving the core pin (frozen-file oracle) unchanged.

## Module pin

- Core commit: `78c5b3d1a4b7f5309a4df6eed4487162634c4010` (eshu origin/main,
  carries S2 apicontract + S4 CLI-support endpoints).
- `github.com/eshu-hq/eshu-sdk/wirecontract v0.1.0-rc.1` (published tag;
  `go.sum` pins hashes). Refs eshu-hq/eshu#9631, part of eshu-hq/eshu#6707.
  Supersedes `github.com/eshu-hq/eshu/sdk/go/apicontract
  v0.0.0-20261029223206-78c5b3d1a4b7` (toolchain-derived pseudo-version at
  the pin, retired by this repoint).
- Provenance: `go mod verify` clean; the module cache's `envelope.go`,
  `tool.go`, `profile.go`, and `freshness.go` are byte-identical to the
  retired pin tree apart from the package clause, and `env.go` differs only
  by the package clause, the `wirecontract:` error prefix, comment-only
  profile wording, and additive alias-shadow hardening the shipped client
  table does not trigger. No `replace`, no `go.work`.

## Verbatim copies (decision 7 moved-package freeze)

Import repoint `eshu/go/internal` -> `eshu-cli/internal` only, except the
recorded deltas below. `internal/cli/resultshape` mirrors
`go/internal/mcp/result/shape` (renamed: the census forbids `internal/mcp/*`
paths; the rename is expressed by the nofork mapping, content identical so no
patch file). `internal/urlredact` copies only the Authority seam (`authority.go`,
`doc.go`, `escape.go`): `query.go` and `free_text.go` need the published
`eshu-sdk/collector` module, which this CLI does not take, and
`boundary_cases.go` / `differential_cases.go` are unreferenced differential
test vectors (nothing in the copied set reads `BoundaryCases` /
`DifferentialCases`; dropping them also keeps the credential scan clean).

Destination-authored tests (coverage ratchet, eshu-hq/eshu-cli#6): new
`*_cli_test.go` files testing frozen sources are named exclusions in
`scripts/check-nofork.sh`, never pin rows — Tier-2 currently names
`internal/cli/resultshape/shape_cli_test.go`,
`internal/cli/evidredact/evidredact_cli_test.go`,
`internal/cli/playbooks/playbooks_cli_test.go`,
`internal/cli/reposelector/reposelector_cli_test.go`, and
`internal/cli/component/component_cli_test.go`, per the ratchet standing
ruling (eshu-hq/eshu-cli#6); Tier-1 names its own in `docs/tier1-pin.md`.
Exclusions retire at re-pin if core gains the test.

| `dest` | sha256 |
|---|---|
| `internal/cli/opdigest/artifact.go` | 23d4e6e11e72b1a93bca33b379f3f518086a97333ec67d710ee2eb67ecabd6d2 |
| `internal/cli/opdigest/digest.go` | 508fa70ce122862259523874eba492416e269584d7aabc06b40115588106ecef |
| `internal/cli/opdigest/doc.go` | ee85ffe43287c9b782a700033503de26278ec36e95bd32a43dfd0d1829dd78ed |
| `internal/cli/component/api.go` | b3599d1a9c4c6e33c23f08a78d5ba7ddd856bbccac448a3db561ef538f62d8ad |
| `internal/cli/component/doc.go` | 9ed4004677c2fb346068aa8d74fb79c49c6c48b2af8b94ffe3fe73cab969a381 |
| `internal/cli/component/flags.go` | 5116c2b92458df5350fe02833b8d9d879e8db20c46c6bdd7aa206bf68f8b8b03 |
| `internal/cli/component/values.go` | 216b0fbfc2b17016cdfad56de27be4d434040edecd08c262b33cc7e51584b565 |
| `internal/cli/component/write.go` | 32dec8bf4e661d2ac45383107e35ebd497fe0ef73169e38d8cacbe10e84262e6 |
| `internal/cli/mcpsetup/doc.go` | d0d925bb11a4bcc3c3bc2a8e22a1842f310cca2f370a9ef81897a310573fad6f |
| `internal/cli/mcpsetup/posture.go` | 36d1a1219462f01b2e99ed012a4e5ad70d14948070bab8cfb45642eb2d49b65f |
| `internal/cli/mcpsetup/setup.go` | 18432c848c7db0a76634c92246f91bf03cfaecd57b45616c785906e8df5fa4e8 |
| `internal/cli/mcpsetup/snippet.go` | 280268989813acfc560c3ff58719a3f90a41a22fe74680fd77603f72925a158f |
| `internal/cli/mcpsetup/verify.go` | 3fd4f47293e4921ec1970ea54eca5159e9b0a978f3f35c4302f8ec8942495e10 |
| `internal/cli/mcpsetup/write.go` | cfb2974a4b7931619c79559d93b8c3c50845138c45f1315d23db5259678c21fd |
| `internal/cli/resultshape/doc.go` | ec57c94a65c38c9dcafe6c7d92cb2d7609414d134c087b2eadb4effe186576e7 |
| `internal/cli/resultshape/shape.go` | e2ca3bf593694d87aaa3207b5236f199f4ba42b2537ab3a199658d83140fd82b |
| `internal/cli/evidredact/doc.go` | 05808ee065be941a3db6e03682e80f6ca4c7af38821200dcd4bf49cc1fea0532 |
| `internal/cli/evidredact/token.go` | e3364245c4f1e81752739964d6f83d1788c2aa5d5a2faade18f29c0462e621c0 |
| `internal/urlredact/authority.go` | bc4b8f16749445d10d3d06e80d440a6f173789c4d4ce18fa92124704c4461058 |
| `internal/urlredact/doc.go` | fcb8678c2f7f74522321c781284879c7d3dc08f2fa2c7b94c5ad44f975e570d7 |
| `internal/urlredact/escape.go` | 5ec19d917823bd8c7086dda17dba7d379268774ebc214194c56b76d31368c607 |
| `internal/cli/config/validate.go` | 585dcf8beedc76a2cec9cdfc30a227623fce0667fb9babfd7de7d2915493ec15 |
| `internal/cli/config/doc.go` | 49632c472da719d977cd3b3fcec7c8c1fe08997cbd01ef892d4b5db4962b18ce |

Closures (`go list -deps`, internal packages): opdigest 1, component 1,
mcpsetup 3 (itself + resultshape + evidredact), evidredact 1, urlredact 1,
resultshape 1. No query/mcp/serviceintel/extension-host imports.

Retired: `internal/envregistry` (the superseded full-table copy; `config
validate` now validates against the published client table). Tier-1 pin rows
for it are historical.

## Recorded deltas

- `internal/cli/mcpsetup/verify.go`: `mcp.ToolDefinition` ->
  `wirecontract.ToolDefinition` (the published wire type since eshu-hq/eshu#9631;
  before that `apicontract.ToolDefinition`; the 497-closure `mcp` root stays
  out); `toolLister`/comment and `toolCountDetail` reworded to the
  remote-tools source. Patch:
  `testdata/parity/deltas/internal/cli/mcpsetup/verify.go.patch`.
- `internal/cli/config/validate.go`: `envregistry` -> `wirecontract`
  (registry + Finding types; logic identical, messages byte-identical for
  client-table variables; `apicontract` before eshu-hq/eshu#9631). Patch:
  `testdata/parity/deltas/internal/cli/config/validate.go.patch`.
- `internal/cli/config/doc.go`: prose follows the import. Patch:
  `testdata/parity/deltas/internal/cli/config/doc.go.patch`.
- `cmd/eshu/root.go`: root Long routes MCP setup at this CLI now. Patch:
  `testdata/parity/deltas/cmd/eshu/root.go.patch` (extends the Tier-1
  doctor-removal hunk).
- `cmd/eshu/output.go`: Tier-1 `nolint:unused` guards removed (printTable,
  printSuccess, printWarning all have Tier-2 callers). The Tier-1 patch file
  is deleted: live diff vs the pin is empty.
- `internal/cli/resultshape`: path rename only (see above); content
  identical, no patch file. Its two importers record the rename:
  `testdata/parity/deltas/internal/cli/mcpsetup/setup.go.patch` and
  `snippet.go.patch`.

## Tier-2 surface decisions

- `investigation export` stays stubbed. Its packet build
  (`query.BuildSupplyChainImpactPacket` and siblings) maps reducer-owned
  domain types inside the 452-closure query root with no build endpoint; only
  the render step has one. Copying the builders would be a second copy of
  truth assembly (verdict decision 1), so the family waits on a server-side
  packet-build endpoint (follow-up, core side).
- `report validate`, `service-report`, and `answer-quality-scorecard` add no
  remote flags: they were offline commands in core, so the endpoint resolves
  from ambient `ESHU_SERVICE_URL`/profile and `--help` stays byte-identical.
- Local `mcp setup --verify` fails the tools-visible stage closed ("no tool
  surface available"): the thin CLI carries no embedded tool surface. Hosted
  verify with a bearer lists the deployment's tools remotely.
- `config validate --reference` prints the published client table, not the
  full platform table (by S2 design: the thin CLI validates exactly the
  variables it owns). Dest-pinned golden.
- `component` local verbs keep core Use/Short/Args/flags with stub bodies
  (extension-host-bound, not wire-blocked), so parent and per-verb help match
  core byte-for-byte.
- `internal/cli/component/api.go` keeps its local envelope mirror and does
  NOT adopt `wirecontract.ResponseEnvelope` (eshu-hq/eshu#9631 decision).
  The mirror's `Data`/`Truth map[string]any` plus `--json` passthrough lets a
  server-side field addition reach operators without a CLI release; the typed
  `ResponseEnvelope` would drop unknown fields on the decode/re-marshal
  round-trip and force rewriting every map accessor for zero wire gain. The
  package still imports no core module.
- Live `data_digest` parity: the capture `bundle_id` covers `created_at`, so
  the parity canon masks both; `data_digest` and the full response echo are
  compared unmasked.

## Parity extension

- Stub serves S4 fixtures byte-for-byte (`testdata/parity/fixtures/`),
  generated from real core logic at the pin by
  `go/scratch-tier2-fixtures` (throwaway, lives in the parity worktree, not
  committed here): capture/validate/compose/score responses plus component
  inventory/diagnostics rows and a tools/list answer.
- New core-compared cases: Tier-2 help (all new commands + component/mcp
  children), server (capture/validate/compose/score/component), local
  (refusals, digest, bad inputs, setup print, tools), config clean-room
  cases. Tier-1 goldens are unchanged at the new pin (no core drift).
- Dest-pinned (`cli-`): shrunk E3 set, root help, hosted verify (embedded vs
  remote tool count), config reference, E4.
