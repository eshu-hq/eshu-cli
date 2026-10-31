# Tier-3 population pin (Refs eshu-hq/eshu#8874)

Third-population slice: the census-passing deferred families. `doctor`
moves as a frozen verbatim copy (closure 3, sole CLI consumer); `workspace`
lands as destination-authored wiring (thin `status` plus removed/retained
children). The S4 render family (`investigation export`) stays stubbed:
packet assembly (`query.Build*Packet`, 452-closure) has no core endpoint on
eshu origin/main, so there is no thin-client target yet.

## Module pin

- Core commit: `78c5b3d1a4b7f5309a4df6eed4487162634c4010` (eshu origin/main,
  carries S2 apicontract + S4 CLI-support endpoints). Every Tier-3 source
  below is byte-identical between this pin and eshu origin/main `960206d657`
  (verified per file with sha256 before the copy), so the Tier-2 pin SHA
  covers Tier-3 with no new core pin.
- `github.com/eshu-hq/eshu-sdk/wirecontract v0.1.0-rc.1` (unchanged).
- `github.com/eshu-hq/eshu-sdk/collector v0.1.0-rc.5` (new): the exact
  version the core pin uses. `internal/cli/doctor` needs
  `evidredact.Endpoint`, which needs `urlredact.Query`, which predicates on
  `collector.IsSensitiveKeyName`. Tier-2 left `urlredact/query.go` out
  because no Tier-2 family needed it; doctor needs it, forking the walk is
  accuracy-forbidden (the hand-split drift is recorded in `endpoint.go`),
  and collector is a qualified published SDK module, so the CLI takes the
  same pinned version core builds against. No `replace`, no `go.work`.

## Verbatim copies (decision 7 moved-package freeze)

Import repoint `eshu/go/internal` -> `eshu-cli/internal` only, no recorded
deltas: the fetch-leg oracle diffs import-normalized core vs destination
with no patch file, so any logic drift fails the freeze. `collector` is an
external SDK import and is not repointed.

| `dest` | sha256 |
|---|---|
| `internal/cli/doctor/doctor.go` | 35d9a7c3b945045288bf047825d056c803509f70dcb512ec704657f1de5e9b13 |
| `internal/cli/doctor/doc.go` | fa6412f6468ea9e88c3f5f3dbb49c754341c59a9d1999695e7cea6bad8d40fa2 |
| `internal/cli/doctor/defaults_test.go` | bfa3970a8ef900c23b286cf235135601fc27a1b7c21a0f1cf9d78ff479d2fe3d |
| `internal/cli/doctor/doc_lockstep_test.go` | 7663236b150f11b82ab4c0a6b391443f3cbce4cce8345b02ba92375ce2d8fa37 |
| `internal/cli/doctor/doctor_redaction_test.go` | 4a24300ca0fc7bba1a5a85409f17c63174c9e534011181671a1846e7aeaea275 |
| `internal/cli/doctor/health_url_test.go` | 5ef500bb3164200d679e0e732a83407ce49ed12dd9e67cfdf43a2066b6887b4c |
| `internal/cli/evidredact/endpoint.go` | ef242affa64a760f73a4b8dfc352f6085a4460caa54f037d031fa43964658dce |
| `internal/urlredact/query.go` | 45f9b2f5867dd456f2c53584de56f3bc2543a96ccc4c804431243b89b33983b7 |

Closures (`go list -deps`, internal packages): doctor 3 (itself +
urlredact + evidredact, both already in the destination), evidredact 1,
urlredact 1. No query/mcp/serviceintel/doctruth/component/storage/eshulocal/
localsupervisor in any populated closure; no fork/vendor/replace.

## Destination-authored files (parity goldens, not the freeze)

- `cmd/eshu/doctor.go`: registration plus `runDoctor`, behavior-identical
  to the core twin (same Use/Short, no flags, same ambient resolution:
  `NEO4J_URI` env first, then settings fallback; `NewAPIClient("","","")`
  base URL; `ESHU_POSTGRES_DSN` presence only).
- `cmd/eshu/workspace.go`: parent plus five children with core-identical
  Use/Short/Args/flags. `status` is the thin client (`GET
  /api/v0/status/pipeline`, ambient resolution like core — the core command
  parses remote flags but resolves ambiently, and the copy preserves that).
  `plan`/`sync`/`index` keep the removed-command errors; `watch` keeps
  retained-lifecycle guidance pointing at the core binary (the destination
  owns no local-host supervisor).
- `cmd/eshu/doctor_test.go`, `cmd/eshu/workspace_test.go`,
  `internal/urlredact/query_tier3_test.go`: unit proof for the new surface
  (registration, request path, removed/retained contracts, query walk).
- `internal/urlredact/urlredact_cli_test.go`: unit proof for the urlredact
  decode and authority readers (escape tables, boundary depths, authority
  rewrite, fail-closed error reasons) plus the query-walk branches the
  tier-3 test leaves uncovered, under the coverage ratchet
  (eshu-hq/eshu-cli#6).

## Tier-3 surface decisions

- `doctor` output depends on the machine (PATH binaries, HOME, live probe),
  so its parity golden runs in a clean room (`env -i` with an isolated
  `ESHU_HOME`, a `/usr/bin:/bin` PATH without service binaries, and the
  stub server as the probe target): core and destination agree byte-for-byte
  there, and the copied package tests pin the redaction behavior.
- `workspace status` needs no fixture: the stub server echoes the request
  path, and both binaries print the same indented JSON.
- `workspace watch` help stays core-identical; only the body differs (core
  re-execs the retained supervisor, the CLI points at the core binary).
  Root help is re-pinned: `doctor` and `workspace` leave the E3 stub set.
- Stays stubbed with this PR recording the evidence: `investigation export`
  (no packet-build endpoint), `docs verify` (retain-forever, doctruth +
  Postgres), `scan`/`vuln-scan` local modes (eshulocal / 499-closure),
  `service` local profiles and `graph`/`watch`/`admin` (RUNTIME / retained),
  leaf scorers (`compparity` 468 via capabilitycatalog+query; `evidpacket`
  move breaks the core `compparity/exercises.go` caller;
  `vulnerabilityparity` serves retained vulnscan).

## Parity extension

- New core-compared cases: `doctor` + `workspace` help (parent and all five
  children), `workspace status` server case, `doctor` local case (clean
  room). Tier-1/2 goldens unchanged.
- Dest-pinned (`cli-`): shrunk E3 set (no `doctor`, no `workspace`), root
  help without them. The `cli-stub-doctor` / `cli-stub-workspace` goldens
  are deleted with the stubs.
- Stub serving still covers the remaining E3 set byte-for-byte; no S4
  fixture changes (no new endpoint consumed).
