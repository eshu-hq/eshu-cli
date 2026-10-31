#!/bin/bash
# Summarize a Go coverprofile per package and enforce the coverage floor.
# Shared by the CI race lane (.github/workflows/ci.yml) and `make race`
# so local and CI parity is one implementation, not two copies.
# Part of eshu-hq/eshu-cli#6.
# Usage: scripts/coverage-gate.sh <coverprofile> <floor-percent> [summary-file]
# Exits 1 when the total statements coverage is below the floor.
set -euo pipefail

profile="${1:?usage: coverage-gate.sh <coverprofile> <floor-percent> [summary-file]}"
floor="${2:?usage: coverage-gate.sh <coverprofile> <floor-percent> [summary-file]}"
summary="${3:-}"

# Fail closed on a non-numeric floor: awk would coerce it to 0 and the
# gate would pass vacuously (eshu-hq/eshu-cli#35 review P1).
case "$floor" in
'' | '.' | *[!0-9.]* | *.*.* | -*)
  echo "coverage-gate: bad floor '$floor' (want a number like 42.4)" >&2
  exit 1
  ;;
esac

total="$(go tool cover -func="$profile" | awk '/^total:/ { gsub(/%/, "", $3); print $3 }')"
if [ -z "$total" ]; then
	echo "coverage-gate: no total line in $profile" >&2
	exit 1
fi

table="$(awk '
	/^mode:/ { next }
	{
		line = $1
		sub(/:[0-9.]+,[0-9.]+$/, "", line)
		pkg = line
		sub(/\/[^\/]+$/, "", pkg)
		stmts = $2 + 0
		count = $3 + 0
		total[pkg] += stmts
		if (count > 0) { covered[pkg] += stmts }
	}
	END {
		for (p in total) {
			pct = (total[p] > 0) ? 100 * covered[p] / total[p] : 0
			printf "%s %.1f%%\n", p, pct
		}
	}
' "$profile" | sort)"

echo "coverage by package (covermode=atomic):"
echo "$table"
echo "total: ${total}% (floor: ${floor}%)"

if [ -n "$summary" ]; then
	{
		echo "## Race coverage (covermode=atomic)"
		echo ""
		echo "| Package | Coverage |"
		echo "| --- | --- |"
		echo "$table" | awk '{ printf "| `%s` | %s |\n", $1, $2 }'
		echo ""
		echo "total: ${total}% (floor: ${floor}%)"
	} >>"$summary"
fi

awk -v total="$total" -v floor="$floor" 'BEGIN {
	if (total + 0 < floor + 0) {
		printf "coverage-gate: total %.1f%% is below floor %s%%\n", total, floor
		exit 1
	}
	printf "coverage-gate: total %.1f%% meets floor %s%%\n", total, floor
}'
