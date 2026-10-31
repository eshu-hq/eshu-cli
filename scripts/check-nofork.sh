#!/bin/bash
# No-fork diff for the Tier-1/Tier-2 population (Refs eshu-hq/eshu#8874).
#
# Verifies every copied file is byte-identical to the pinned core commit
# except the module import-path repointing (decision 7 moved-package freeze:
# divergent evolution in either tree is a fork and blocks release).
#
#   scripts/check-nofork.sh                      # offline: dest bytes vs recorded pin hashes
#   scripts/check-nofork.sh --fetch              # CI: sparse-fetch core at the pin SHA and
#                                                # diff file by file (loud skip offline)
#   scripts/check-nofork.sh --fetch --require-fetch  # CI lane: fetch failure is a
#                                                # hard FAIL, never a SKIP (eshu-hq/eshu-cli#23)
#
# Tier-2 re-pins the whole freeze at the Tier-2 pin (78c5b3d1a4): every
# Tier-1 source is byte-identical between the S1 pin and the Tier-2 pin
# (verified by diffing all mapped files across both SHAs), so one pin covers
# both tiers. Tier-2 rows live in docs/tier2-pin.md; the offline check reads
# both pin files.
#
# Tier-3 reuses the same pin SHA: every Tier-3 source (doctor, evidredact
# endpoint, urlredact query) is byte-identical between the Tier-2 pin and
# eshu origin/main, verified per file before the copy. Tier-3 rows live in
# docs/tier3-pin.md.
set -euo pipefail
cd "$(dirname "$0")/.."

PIN_FILES="docs/tier1-pin.md docs/tier2-pin.md docs/tier3-pin.md"
PIN_SHA=78c5b3d1a4b7f5309a4df6eed4487162634c4010
OLD_PREFIX="github.com/eshu-hq/eshu/go/internal"
NEW_PREFIX="github.com/eshu-hq/eshu-cli/internal"
FAIL=0

# Moved files: "<dest-path> <core-path>". Destination-authored files (new
# commands and their tests, plus the Tier-1 seeds) are excluded: their
# contract is pinned by parity goldens, not by the freeze.
moved_files() {
  find cmd/eshu internal -name '*.go' | sort | while read -r dest; do
    case "$dest" in
      cmd/eshu/doc.go | cmd/eshu/neo4j.go | cmd/eshu/stubs.go | cmd/eshu/stubs_test.go | \
        cmd/eshu/client_bearer_test.go | \
        cmd/eshu/report.go | cmd/eshu/report_test.go | \
        cmd/eshu/service_report.go | cmd/eshu/service_report_test.go | \
        cmd/eshu/scorecard.go | cmd/eshu/scorecard_test.go | \
        cmd/eshu/config.go | cmd/eshu/config_test.go | \
        cmd/eshu/component.go | cmd/eshu/component_test.go | \
        cmd/eshu/mcp.go | cmd/eshu/mcp_test.go | \
        cmd/eshu/doctor.go | cmd/eshu/doctor_test.go | \
        cmd/eshu/workspace.go | cmd/eshu/workspace_test.go | \
        cmd/eshu/install.go | cmd/eshu/install_test.go | \
        internal/cli/graphinstall/* | \
        internal/cli/change/redaction_helper_test.go | internal/cli/config/credential_roundtrip_test.go | \
        internal/cli/resultshape/shape_cli_test.go | \
        internal/cli/apierr/apierr_cli_test.go | \
        internal/buildinfo/buildinfo_cli_test.go | \
        internal/cli/evidredact/evidredact_cli_test.go | \
        internal/cli/playbooks/playbooks_cli_test.go | \
        internal/cli/reposelector/reposelector_cli_test.go | \
        internal/cli/component/component_cli_test.go | \
        internal/cli/trace/trace_cli_test.go | \
        internal/cli/opdigest/opdigest_cli_test.go | \
        internal/cli/mcpsetup/mcpsetup_cli_test.go | \
        internal/cli/mcpsetup/mcpsetup_verify_cli_test.go | \
        internal/urlredact/query_tier3_test.go | \
        internal/urlredact/urlredact_cli_test.go)
        continue
        ;;
    esac
    case "$dest" in
      cmd/eshu/*) echo "$dest go/$dest" ;;
      internal/cli/resultshape/*) echo "$dest go/internal/mcp/result/shape/$(basename "$dest")" ;;
      internal/*) echo "$dest go/$dest" ;;
    esac
  done
}

WANT_FETCH=0
REQUIRE_FETCH=0
for arg in "$@"; do
  case "$arg" in
    --fetch) WANT_FETCH=1 ;;
    --require-fetch) REQUIRE_FETCH=1 ;;
  esac
done

fail_or_skip_fetch() {
  if [ "$REQUIRE_FETCH" = 1 ]; then
    echo "FAIL(fetch): $1 (--require-fetch is set, the independent leg must run)"
    FAIL=1
  else
    echo "SKIP(fetch): $1; offline pin-hash check below still enforces the freeze"
  fi
}

if [ "$WANT_FETCH" = 1 ]; then
  CORE_D="$(mktemp -d)"
  trap 'rm -rf "$CORE_D"' EXIT
  if ! git clone -q --filter=blob:none --no-checkout https://github.com/eshu-hq/eshu.git "$CORE_D" 2>/dev/null; then
    fail_or_skip_fetch "cannot reach github.com"
  elif ! git -C "$CORE_D" fetch -q --depth 1 origin "$PIN_SHA" >/dev/null 2>&1; then
    fail_or_skip_fetch "cannot fetch pin SHA $PIN_SHA"
  else
    git -C "$CORE_D" checkout -q "$PIN_SHA" >/dev/null 2>&1
    while read -r dest core; do
      if [ ! -f "$CORE_D/$core" ]; then
        echo "FAIL(fetch): $core missing at pin SHA"
        FAIL=1
        continue
      fi
      # Recorded-delta verification: the live diff of import-normalized core
      # vs destination must equal the committed patch (absent patch file
      # means the file must be identical). Word-matching filters cannot
      # express "this hunk is the recorded surgery", so the patch is the
      # oracle: any unrecorded hunk fails the freeze.
      sed "s|$OLD_PREFIX|$NEW_PREFIX|g" "$CORE_D/$core" >"$CORE_D/.norm"
      patch="testdata/parity/deltas/$dest.patch"
      actual="$(diff -u --label "core/$core" --label "dest/$dest" "$CORE_D/.norm" "$dest" || true)"
      if [ ! -f "$patch" ]; then
        if [ -z "$actual" ]; then
          echo "PASS(fetch): $dest identical to pin except import paths"
        else
          echo "FAIL(fetch): $dest differs from pin beyond import paths:"
          echo "$actual" | head -20
          FAIL=1
        fi
      elif [ "$actual" = "$(cat "$patch")" ]; then
        echo "PASS(fetch): $dest matches pin except imports and the recorded delta"
      else
        echo "FAIL(fetch): $dest differs from the recorded delta $patch:"
        diff "$patch" <(printf '%s\n' "$actual") | head -30
        FAIL=1
      fi
    done < <(moved_files)
  fi
fi

# Offline pin-hash check: destination bytes must match the pin files
# (docs/tier1-pin.md for Tier-1 rows, docs/tier2-pin.md for Tier-2 rows,
# docs/tier3-pin.md for Tier-3 rows).
while read -r dest core; do
  want=""
  for pin in $PIN_FILES; do
    want="$(grep -F "| \`$dest\` |" "$pin" | sed -n 's/.*| \([0-9a-f]\{64\}\) |.*/\1/p' | head -1 || true)"
    [ -n "$want" ] && break
  done
  if [ -z "$want" ]; then
    echo "FAIL(pin): no pin hash for $dest in $PIN_FILES"
    FAIL=1
    continue
  fi
  got="$(sha256sum "$dest" | cut -d' ' -f1)"
  if [ "$got" != "$want" ]; then
    echo "FAIL(pin): $dest drifted from pin ($got != $want)"
    FAIL=1
  fi
done < <(moved_files)
if [ "$FAIL" = 0 ]; then
  echo "NOFORK PIN CHECK PASSED"
fi
exit "$FAIL"
