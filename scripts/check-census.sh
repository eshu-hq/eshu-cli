#!/bin/bash
# Closure + hygiene census for the Tier-1 population (Refs eshu-hq/eshu#8874).
#
# Fails closed on:
#   1. any populated closure containing a forbidden package (query, mcp,
#      serviceintel, doctruth, component, storage, eshulocal,
#      localsupervisor) or any core import (github.com/eshu-hq/eshu/go);
#   2. per-family internal-package counts drifting from the verdict census;
#   3. go.mod carrying a replace directive or a go.work file;
#   4. credential material (assigned secrets, keys, tokens) in Go sources.
set -euo pipefail
cd "$(dirname "$0")/.."

FAIL=0

# 1. per-family internal closure counts (verdict census, corroborated here
#    with go list -deps; the counts were first established with git grep).
check_count() {
  local pkg="$1" want="$2" got
  got="$(go list -deps "./internal/$pkg" 2>/dev/null | grep -c "eshu-cli/internal" || true)"
  if [ "$got" != "$want" ]; then
    echo "FAIL: internal/$pkg closes over $got internal packages, want $want"
    FAIL=1
  else
    echo "PASS: internal/$pkg closes over $got internal packages"
  fi
}

check_count cli/trace 1
check_count cli/apierr 1
check_count cli/playbooks 1
check_count cli/change 2
check_count cli/entitymap 2
check_count cli/freshness 2
check_count cli/config 1
check_count cli/reposelector 1
check_count cli/mcpsetup 4
check_count cli/opdigest 1
check_count cli/component 1
check_count cli/evidredact 2
check_count urlredact 1
check_count cli/resultshape 1
check_count cli/doctor 3
check_count buildinfo 1
# Tier-3 note: evidredact 1->2 and mcpsetup 3->4 come from the endpoint.go
# move (Endpoint walks urlredact.Query). Core shows the same edge:
# go/internal/cli/evidredact closes over urlredact plus itself at the pin.
# No new forbidden package; the closure stays urlredact-only beyond itself.

# 2. forbidden packages and core imports nowhere in any populated closure.
FORBIDDEN="query|mcp|serviceintel|doctruth|component|storage|eshulocal|localsupervisor"
if go list -deps ./... 2>/dev/null | grep -E "eshu-cli/internal/($FORBIDDEN)"; then
  echo "FAIL: populated closure contains a forbidden package"
  FAIL=1
else
  echo "PASS: no forbidden package in any populated closure"
fi

if go list -deps ./... 2>/dev/null | grep -F "eshu-hq/eshu/go/internal"; then
  echo "FAIL: destination imports core go/internal"
  FAIL=1
else
  echo "PASS: zero core go/internal imports"
fi

if grep -rF "eshu-hq/eshu/go/internal" --include='*.go' cmd internal; then
  echo "FAIL: core import path text remains in sources"
  FAIL=1
else
  echo "PASS: no core import path text in sources"
fi

# 3. go.mod pins: exact versions, no replace, no go.work.
if grep -E "^replace" go.mod; then
  echo "FAIL: go.mod carries a replace directive"
  FAIL=1
else
  echo "PASS: no replace directive"
fi
if [ -e go.work ] || [ -e ../go.work ]; then
  echo "FAIL: go.work present"
  FAIL=1
else
  echo "PASS: no go.work"
fi
echo "--- pinned modules ---"
grep -E "^require|^\s+github" go.mod

# 4. credential material: assigned secret values in Go sources and scripts.
#    Env-var NAMES (ESHU_API_KEY etc.) are the contract and are allowed;
#    assigned secret VALUES are not. The redaction canary C4N4RY* is synthetic.
if grep -rEni '(password|passwd|secret|api[_-]?key|bearer)[[:space:]]*[:=][[:space:]]*"[^"]{4,}"' --include='*.go' cmd internal | grep -v "C4N4RY"; then
  echo "FAIL: possible credential material in Go sources"
  FAIL=1
else
  echo "PASS: no credential material in Go sources"
fi
if grep -rEn 'BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY|AKIA[0-9A-Z]{16}|ghp_[A-Za-z0-9]{20,}|xox[bpas]-[A-Za-z0-9-]+' --include='*.go' --include='*.sh' --include='*.py' cmd internal scripts testdata 2>/dev/null; then
  echo "FAIL: key material pattern in tree"
  FAIL=1
else
  echo "PASS: no key material patterns"
fi

if [ "$FAIL" != 0 ]; then
  echo "CENSUS FAILED"
  exit 1
fi
echo "CENSUS PASSED"
