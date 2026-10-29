# Provenance: day-one contract seed (`#8874`)

Refs `eshu-hq/eshu#8874`, part of `eshu-hq/eshu#6707`.
Status: preparation only. This repository holds no cutover authority
until its reviewed cutover.

## Source

- Source repository: `eshu-hq/eshu`, ref `origin/main`.
- Source SHA at re-seed time: `37171bafa22edcdded62dbcb7c7ce7bdaf37756a`
  (merge of `eshu-hq/eshu#9504`).
- Step-1 census: PR `eshu-hq/eshu#9361` (inventory + pinning tests).
- Step-2 contract: PR `eshu-hq/eshu#9441` (day-one contract + pinning tests).
- Install/filesystem seam: PR `eshu-hq/eshu#9484` (fail-closed source
  contract + managed-home filesystem pins).
- Open-questions verdict: PR `eshu-hq/eshu#9504` (arbiter ruling
  recorded, census Open questions replaced with Decided questions,
  `go/cmd/eshu/` re-pinned at 146 entries).

## Verbatim files (byte-parity)

These files are copied byte for byte from the source SHA above.
`scripts/check-parity.sh` (and the `parity` CI lane) re-verifies them
against `docs/contract/PARITY.sha256`.

| Destination path | Source path at the re-seed SHA | sha256 |
| --- | --- | --- |
| `docs/contract/8874-eshu-cli-contract.md` | `docs/internal/evidence/8874-eshu-cli-contract.md` | `abf8a2d088f398019e5f0dba21865ab3a8dc21af0271fc6140b77fd726abd5ca` |
| `docs/contract/cli_extraction_inventory.md` | `go/cmd/eshu/cli_extraction_inventory.md` | `d8af0cb32981d35113e69e3973039e4ed550ed3254a9776722c8102db9c8f043` |

Proof: `git show <source-SHA>:<source-path> | sha256sum` equals the
table entry, and `diff` of source bytes vs destination file is empty.

## What this seed does not move

No Go code moves in this seed. `go/cmd/eshu` (the cobra tree),
`go/internal/cli/*`, `client.go`, `output.go`, `contract.go`, and the
env registry stay in `eshu`. RUNTIME launchers (`api start`, `mcp
start`, `serve start`, hidden `local-host`) and the RETAIN families
(`admin`, `graph`, `watch`, `w`) stay out of scope per the inventory
and the recorded arbiter ruling. The cutover PR repoints this
repository at the contract figures, proves parity per the census plan,
and removes the in-tree originals only after independent build,
release, deploy, and rollback rehearsal with production acceptance
recorded.

## Pinning tests (stay in `eshu`)

- `go/cmd/eshu/cli_extraction_contract_test.go`: pins the contract
  record anchors, demo seam, envelope MIME, exit table, Bearer
  credential flow, compose-seam semantics, the hidden `local-host`
  boundary, and version reporting.
- `go/cmd/eshu/cli_extraction_inventory_test.go`: pins inventory
  coverage of the cobra tree, the RETAIN `admin` rule, the contract
  surfaces, and the `#8874` arbiter ruling.

## NOT_CHECKED

Independent release, QA EKS deployment, rollback rehearsal, production
acceptance, and vulnerability-response ownership for the extracted
stream. See `README.md` for the carried open questions.
