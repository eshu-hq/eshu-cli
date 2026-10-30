# Tier-1 moved-package freeze pin (Refs eshu-hq/eshu#8874)

Pinned core commit: `73418463ed48d086047d40b82a89f4eef28f2b2f`
(`origin/main` with S1 merged as #9512).

Tier-2 amendment: the freeze re-pins at `78c5b3d1a4b7f5309a4df6eed4487162634c4010`
(see `docs/tier2-pin.md`); every Tier-1 source below is byte-identical across
both SHAs, so the Tier-1 rows stay valid at the new pin. Four rows changed
content in Tier-2 (`cmd/eshu/output.go`: nolint guards removed, callers
landed; `cmd/eshu/root.go`: Long routes MCP setup at this CLI;
`internal/cli/config/validate.go` + `doc.go`: wirecontract repoint, via
`apicontract` at Tier-2 and the published `wirecontract` tag since
eshu-hq/eshu#9631) and carry their new hashes; `internal/envregistry` rows
are historical (package retired in Tier-2). Every file below is a pinned copy:
byte-identical to the core path at the pin except the module import-path
repointing `github.com/eshu-hq/eshu/go/internal` ->
`github.com/eshu-hq/eshu-cli/internal`, plus three recorded deltas
(`cmd/eshu/basic.go`: watch registration removed, now an E3 STUB;
`cmd/eshu/root.go`: doctor registration removed, now an E3 STUB, plus the
MCP-onboarding `Long` reword so root help no longer routes through the
stubbed `mcp` family (eshu-hq/eshu-cli#23);
`cmd/eshu/output.go`: `//nolint:unused` freeze annotations on the three
helpers whose Tier-2/3 callers land with later tiers). The exact recorded
diffs live in `testdata/parity/deltas/<path>.patch` (stable `core/` /
`dest/` labels); `check-nofork.sh --fetch` requires the live diff to equal
the committed patch byte-for-byte, so any unrecorded hunk fails closed.
The CI parity lane runs `--fetch --require-fetch`, so a fetch failure is a
hard FAIL there instead of a SKIP.

Divergent evolution in either tree is a fork under decision 1 of the
population-scope verdict and blocks release. `scripts/check-nofork.sh`
enforces this offline against the hashes below; `--fetch` re-diffs against
a sparse checkout of the pin SHA in CI.

Destination-authored files outside the freeze: `cmd/eshu/doc.go`,
`cmd/eshu/neo4j.go`, `cmd/eshu/stubs.go`, `cmd/eshu/stubs_test.go`,
`cmd/eshu/client_bearer_test.go` (httptest provenance for the
Proof-table bearer-carriage figure, eshu-hq/eshu-cli#23),
`internal/cli/change/redaction_helper_test.go` (canary scaffolding mirroring
core's `render_test.go` helper), and
`internal/cli/config/credential_roundtrip_test.go` (decision-7 proof).

Refresh procedure: re-copy from the new pin, repoint imports, regenerate
this table, re-capture Tier-1 parity goldens, and re-run the full review.

| Destination path | Core path at pin | sha256 (destination bytes) |
| --- | --- | --- |
| `cmd/eshu/analyze.go` | `go/cmd/eshu/analyze.go` | 080289033de6ddb7ca522a206b7238256248b6ef5c529f3a49779efb42e815c7 |
| `cmd/eshu/basic.go` | `go/cmd/eshu/basic.go` | 8e3b833d17f337dce6ce40d2aa9b08ef5bc0b0a6067abbad8c26ac933689c391 |
| `cmd/eshu/change_impact.go` | `go/cmd/eshu/change_impact.go` | 595c4f2aed7ea8ec1003fb00f22cdf7efb349c8b133306df4ba59e50d4f662e3 |
| `cmd/eshu/client.go` | `go/cmd/eshu/client.go` | fe8a75e504753c3ab2fcfadd4dba0c3673357c03542ab96b7355e6aaf58e1f51 |
| `cmd/eshu/contract.go` | `go/cmd/eshu/contract.go` | 8220b72fb675f17f1e31e022fa9327542c184b324eafb386796ed63480f89132 |
| `cmd/eshu/ecosystem.go` | `go/cmd/eshu/ecosystem.go` | 0aaf2874cef526f3b55a3de569ab6ce4cfe502c5cade860c34cac27d55913119 |
| `cmd/eshu/find.go` | `go/cmd/eshu/find.go` | bd22fa01b9a084a0c7fd1fbe865ae29a6dcab7a55e6fb32094ac442efe2a7f8c |
| `cmd/eshu/freshness.go` | `go/cmd/eshu/freshness.go` | 80b6ea7a78866b413e3d542823ff16e5bce80b60f91b90667c07c118a0fa6b1b |
| `cmd/eshu/main.go` | `go/cmd/eshu/main.go` | 618af6e1c7b1151c0390a2f37676a2ce1d57c26cc67fd9512ff9988ef54a31b8 |
| `cmd/eshu/map.go` | `go/cmd/eshu/map.go` | 99083e55f98fb8e5d4a448c4a401c86e36b41c1e35cd94bed35ed5549d67dc2c |
| `cmd/eshu/output.go` | `go/cmd/eshu/output.go` | a44f56e73ee81552fef7fe7edd0f78ea104f52e0300c9154a2265edbfb601322 |
| `cmd/eshu/playbooks.go` | `go/cmd/eshu/playbooks.go` | df92e661ae0e71e6b62d5c356abef309aab0b01faa6d8b3b3e5a597059ec09f0 |
| `cmd/eshu/repository_selector.go` | `go/cmd/eshu/repository_selector.go` | e4cebe6dcb14e8a9c53d481d34a90c39c71330b0db767a81b6fe67a310e31b25 |
| `cmd/eshu/root.go` | `go/cmd/eshu/root.go` | e53b331eb57b22498eae4446fbb40e6c016be2bbc8b673793a7c31c9ec06420f |
| `cmd/eshu/trace.go` | `go/cmd/eshu/trace.go` | af66fe8e36aed58589fce91ae5d1ed5228bd13785a3606eeee18094be79203bf |
| `internal/buildinfo/buildinfo.go` | `go/internal/buildinfo/buildinfo.go` | cdb1de4e5c9f219c721cedd2c90bb6e7068922bf3649221dcfe125b846c72f83 |
| `internal/buildinfo/cli.go` | `go/internal/buildinfo/cli.go` | db77f0f7927a37e0aa0aa5bb3b4c79d8aaae2a9727e2e19138be5ddb2b4d1a13 |
| `internal/buildinfo/doc.go` | `go/internal/buildinfo/doc.go` | 2eb3601930c85c3a31aed3ce3aea23199d64547894816c96797b11b17d00b88b |
| `internal/cli/apierr/doc.go` | `go/internal/cli/apierr/doc.go` | 18265f07f94e19fe64ebd56c18bd8297ade470380741c5c2e336438d2919d3be |
| `internal/cli/apierr/httpstatus.go` | `go/internal/cli/apierr/httpstatus.go` | dca3d7559b9d20cfd0dfb50394c0e0b2553581e992f1a924af728981a4917b96 |
| `internal/cli/change/diff.go` | `go/internal/cli/change/diff.go` | 6520dab9257ff2a2534aea559e7d978c8848716b353e975e202341cb5961f125 |
| `internal/cli/change/doc.go` | `go/internal/cli/change/doc.go` | c69d7bb76b7286e618d0be6a64cc5dc143e87f976f319499e62eda3ac955aa70 |
| `internal/cli/change/envelope.go` | `go/internal/cli/change/envelope.go` | 2585f74f849af94d5f2f5f19a2f4b28ccbcbfe5a6d3219388b8593a2e91fc529 |
| `internal/cli/change/failure.go` | `go/internal/cli/change/failure.go` | e317305da8d2f230f77411ee523e8145c99a51722d9fb7ca81230bc90dffba88 |
| `internal/cli/change/redaction_test.go` | `go/internal/cli/change/redaction_test.go` | 58d6cfcac783544df8a95c197dc33b6f99f2fb5beef238575eb0413fbd5769e3 |
| `internal/cli/change/render.go` | `go/internal/cli/change/render.go` | 950be348d12151afe30f2bf9f9bbb4913b5d3ed611ac3b46524578b4c16652c6 |
| `internal/cli/change/request.go` | `go/internal/cli/change/request.go` | 42558831fb485280741cfdd5f0d52af46de2af318b01c7ee5430215c53d67aa0 |
| `internal/cli/config/backend.go` | `go/internal/cli/config/backend.go` | 41e3bf7aec6f7ee4a74420098a0d6b61ed54fed7612301c52df31d0d4a9aa7d9 |
| `internal/cli/config/doc.go` | `go/internal/cli/config/doc.go` | 49632c472da719d977cd3b3fcec7c8c1fe08997cbd01ef892d4b5db4962b18ce |
| `internal/cli/config/env.go` | `go/internal/cli/config/env.go` | cf89e376d98050e57ce06504eb7c3f8417afcbb01531fb6a56cef836a33e1eb9 |
| `internal/cli/config/validate.go` | `go/internal/cli/config/validate.go` | 585dcf8beedc76a2cec9cdfc30a227623fce0667fb9babfd7de7d2915493ec15 |
| `internal/cli/entitymap/doc.go` | `go/internal/cli/entitymap/doc.go` | db264953ba5a5bc806b98b21f6a13bf7ca3cf7a4ccf63caf85fd155957e68dc1 |
| `internal/cli/entitymap/entitymap.go` | `go/internal/cli/entitymap/entitymap.go` | 4f898e789781801a6044b2a54c8579fd1455758915b4a179b587541d4aacaee5 |
| `internal/cli/entitymap/redaction_test.go` | `go/internal/cli/entitymap/redaction_test.go` | 94ae133fbe59be7abea32e908332e195b039808fdd038e1db6bd3c906aa496e0 |
| `internal/cli/entitymap/render.go` | `go/internal/cli/entitymap/render.go` | 50e920e0ba6d851046deebd1ba76e0667ede2fcd3448ddddbb27822074cdfc67 |
| `internal/cli/entitymap/values.go` | `go/internal/cli/entitymap/values.go` | 5722611c86ea0fc9936e42795fa978000318efe6d88b2602c32f6b90ae7ce715 |
| `internal/cli/freshness/changedsince.go` | `go/internal/cli/freshness/changedsince.go` | 35c9b0547cbef58f354cd07991a10f2ab91ed4fe081ec211f9a43a9719d34633 |
| `internal/cli/freshness/doc.go` | `go/internal/cli/freshness/doc.go` | ef08b3be6c72724589775d51a76e916f4a56c7062cf70014301cf2d0bf77c685 |
| `internal/cli/freshness/envelope.go` | `go/internal/cli/freshness/envelope.go` | 6e034e7408c6536fff92ca7e46b0482521b1dad95edf6fc15fde976232def9c8 |
| `internal/cli/freshness/generations.go` | `go/internal/cli/freshness/generations.go` | 01cf1087f726d58ac1121d2330c57e005f1ffed95cf38a6286a567bd49c516b3 |
| `internal/cli/freshness/redaction_test.go` | `go/internal/cli/freshness/redaction_test.go` | 759a38d239a2a89e18c5b047c06004e8080c6fe5df44535d24e61a5dad287abf |
| `internal/cli/freshness/servicechangedsince.go` | `go/internal/cli/freshness/servicechangedsince.go` | 350bae037135b68d9565771f3c18e455b15d4063e4d12497d3b155ddd07f6d53 |
| `internal/cli/freshness/values.go` | `go/internal/cli/freshness/values.go` | 48d4d167e2ea36d4506fd260fed9ddabc1766fd82e9727b93633ce0241d4f4fe |
| `internal/cli/freshness/write.go` | `go/internal/cli/freshness/write.go` | 22eb97bded18031b2cff789858d069a8d919c798bfad544ee0e7eb3af607cfbf |
| `internal/cli/playbooks/doc.go` | `go/internal/cli/playbooks/doc.go` | 9b3bf1aa1b147628cc03f369cc930698504347411c04a7de5c646d44e86b70a1 |
| `internal/cli/playbooks/playbooks.go` | `go/internal/cli/playbooks/playbooks.go` | 903f1815789921a71f16a687af9862354846c07e915dafc0f298fddadcaf077f |
| `internal/cli/reposelector/doc.go` | `go/internal/cli/reposelector/doc.go` | 4763bc47bed28dc74feb608abbe39450ab739c02ce6545ed0d3b064bf7c493fe |
| `internal/cli/reposelector/reposelector.go` | `go/internal/cli/reposelector/reposelector.go` | b32d93ad9f1224471e5ec396b99a25a670a0119747713314cc081bc2f0bedd5f |
| `internal/cli/trace/doc.go` | `go/internal/cli/trace/doc.go` | 585bcd56fc3d216e6ad74e210cdde528bfd45423f92696dc87962e60fda87bea |
| `internal/cli/trace/envelope.go` | `go/internal/cli/trace/envelope.go` | 71131154c46f7214a8b6bdf91fe3b8b90021a4f398343df9952a38e07b3a68e7 |
| `internal/cli/trace/render.go` | `go/internal/cli/trace/render.go` | 06a62a4a9ecd7eecdad65184eb03d65caa1e041ce434953748c8136ad1eccd97 |
| `internal/cli/trace/value.go` | `go/internal/cli/trace/value.go` | bfdb199ce11f07129af6b4e0bfa464b61a62f4e198de4286e627e7278eabc583 |
| `internal/envregistry/doc.go` | `go/internal/envregistry/doc.go` | dd6f2c53f9ccd14cdf4f393d911b8bfe24baf4346a98f9a89bf6220523254978 |
| `internal/envregistry/entries.go` | `go/internal/envregistry/entries.go` | fc483bc9e0b428b90b762309716792019c493174cad0508396d755baaeae9788 |
| `internal/envregistry/entries_collectors.go` | `go/internal/envregistry/entries_collectors.go` | 6cc3bafb25035511c33d387d5e4412055a3af873ecfd4608006f40222e63f4eb |
| `internal/envregistry/markdown.go` | `go/internal/envregistry/markdown.go` | f16fb725e52e637da7ca1d7943e6b947bbda03ad170027647ccfbb46fb22a557 |
| `internal/envregistry/pages.go` | `go/internal/envregistry/pages.go` | 24ce46be9bb3ba2c4c8b95dd93dffefd3e5ab71d0f88ed0db1e119bdb03a446d |
| `internal/envregistry/registry.go` | `go/internal/envregistry/registry.go` | bf9873cf1a5881e212b90c6b64fb4e0ab9ae9aa7a8afeeb70339ef7647b6c594 |
