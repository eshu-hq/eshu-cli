#!/bin/bash
# No-fork diff for the Tier-1 population (Refs eshu-hq/eshu#8874).
#
# Verifies every copied file is byte-identical to the pinned core commit
# except the module import-path repointing (decision 7 moved-package freeze:
# divergent evolution in either tree is a fork and blocks release).
#
#   scripts/check-nofork.sh          # offline: dest bytes vs recorded pin hashes
#   scripts/check-nofork.sh --fetch  # CI: sparse-fetch core at the pin SHA and
#                                    # diff file by file (loud skip offline)
set -euo pipefail
cd "$(dirname "$0")/.."

PIN_FILE=docs/tier1-pin.md
PIN_SHA=73418463ed48d086047d40b82a89f4eef28f2b2f
OLD_PREFIX="github.com/eshu-hq/eshu/go/internal"
NEW_PREFIX="github.com/eshu-hq/eshu-cli/internal"
FAIL=0

# Moved files: "<dest-path> <core-path>". The four new files (doc.go,
# neo4j.go, stubs.go, stubs_test.go) are destination-authored, not moves.
moved_files() {
  find cmd/eshu internal -name '*.go' | sort | while read -r dest; do
    case "$dest" in
      cmd/eshu/doc.go | cmd/eshu/neo4j.go | cmd/eshu/stubs.go | cmd/eshu/stubs_test.go | \
        internal/cli/change/redaction_helper_test.go | internal/cli/config/credential_roundtrip_test.go)
        continue
        ;;
    esac
    case "$dest" in
      cmd/eshu/*) echo "$dest go/$dest" ;;
      internal/*) echo "$dest go/$dest" ;;
    esac
  done
}

if [ "${1:-}" = "--fetch" ]; then
  CORE_D="$(mktemp -d)"
  trap 'rm -rf "$CORE_D"' EXIT
  if ! git clone -q --filter=blob:none --no-checkout https://github.com/eshu-hq/eshu.git "$CORE_D" 2>/dev/null; then
    echo "SKIP(fetch): cannot reach github.com; offline pin-hash check below still enforces the freeze"
  elif ! git -C "$CORE_D" fetch -q --depth 1 origin "$PIN_SHA" >/dev/null 2>&1; then
    echo "SKIP(fetch): cannot fetch pin SHA $PIN_SHA; offline pin-hash check below still enforces the freeze"
  else
    git -C "$CORE_D" checkout -q "$PIN_SHA" >/dev/null 2>&1
    while read -r dest core; do
      if [ ! -f "$CORE_D/$core" ]; then
        echo "FAIL(fetch): $core missing at pin SHA"
        FAIL=1
        continue
      fi
      sed "s|$OLD_PREFIX|$NEW_PREFIX|g" "$CORE_D/$core" >"$CORE_D/.norm"
      if diff -q "$CORE_D/.norm" "$dest" >/dev/null 2>&1; then
        echo "PASS(fetch): $dest identical to pin except import paths"
      elif [ "$dest" = cmd/eshu/basic.go ] || [ "$dest" = cmd/eshu/root.go ] || [ "$dest" = cmd/eshu/output.go ]; then
        # Recorded deltas only: the watch registration (basic.go), the doctor
        # command (root.go), or the freeze nolint annotations (output.go).
        # Anything else is divergent evolution and fails the freeze.
        rest="$(diff "$CORE_D/.norm" "$dest" | grep "^[<>]" | grep -vi "watch\|doctor\|runwatch\|doctorcmd\|nolint:unused\|freeze keeps the shared output\|callers land with later tiers" || true)"
        if [ -z "$rest" ]; then
          echo "PASS(fetch): $dest matches pin except imports and the recorded delta"
        else
          echo "FAIL(fetch): $dest differs beyond imports and the recorded delta:"
          echo "$rest" | head -20
          FAIL=1
        fi
      else
        echo "FAIL(fetch): $dest differs from pin beyond import paths:"
        diff "$CORE_D/.norm" "$dest" | head -20
        FAIL=1
      fi
    done < <(moved_files)
  fi
fi

# Offline pin-hash check: destination bytes must match docs/tier1-pin.md.
while read -r dest core; do
  want="$(grep -F "| \`$dest\` |" "$PIN_FILE" | sed -n 's/.*| \([0-9a-f]\{64\}\) |.*/\1/p' | head -1 || true)"
  if [ -z "$want" ]; then
    echo "FAIL(pin): no pin hash for $dest in $PIN_FILE"
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
