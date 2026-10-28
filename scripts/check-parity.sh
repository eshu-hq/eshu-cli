#!/bin/bash
# Re-verify byte-parity of the verbatim contract seed files.
# Usage: scripts/check-parity.sh  (exits 0 on match, 1 on mismatch)
set -euo pipefail
cd "$(dirname "$0")/.."
sha256sum -c docs/contract/PARITY.sha256
