#!/bin/bash
# Tier-1/Tier-2 parity over the populated CLI (Refs eshu-hq/eshu#8874).
#
#   scripts/check-tier1-parity.sh check              # build dest, diff vs goldens
#   scripts/check-tier1-parity.sh capture-core BIN   # capture core-side goldens
#   scripts/check-tier1-parity.sh capture-cli        # capture dest-pinned goldens
#
# Goldens live in testdata/parity/goldens/<case>.{out,err,exit}. Cases whose
# name starts with "core-" hold core CLI bytes at the Tier-2 pin
# (78c5b3d1a4; Tier-1 families were re-captured there and must be unchanged).
# The destination must reproduce stdout, stderr and exit code byte-for-byte.
# Cases starting with "cli-" pin sanctioned diffs: the E3 stub set (shrunk by
# Tier-2), the E4 index discovery contract, root help without the RUNTIME
# launchers, the Tier-2 verify/config-reference outputs (the E5 per-family
# help deltas), and the local-verify tools fail-closed behavior.
set -euo pipefail
cd "$(dirname "$0")/.."

GOLDENS=testdata/parity/goldens
SERVER=testdata/parity/stub-server.py
PORT=18091

# help cases: "<case-suffix> <args...>" (--help appended by the runner).
# Root help is NOT here: it is a cli-pinned sanctioned diff.
# Tier-2 adds the populated families plus the kept core surface of the
# partially-thin component and mcp parents (local verbs keep core Use/Short/
# flags with a stub body, so parent and per-verb help match byte-for-byte).
HELP_CASES=(
  "query query" "list list" "stats stats" "index-status index-status"
  "index index" "change change" "change-impact change impact"
  "change-plan change plan" "map map" "trace trace" "trace-service trace service"
  "freshness freshness" "freshness-generations freshness generations"
  "freshness-changed-since freshness changed-since"
  "freshness-service-changed-since freshness service-changed-since"
  "playbooks playbooks" "playbooks-list playbooks list"
  "playbooks-resolve playbooks resolve" "ecosystem ecosystem"
  "ecosystem-overview ecosystem overview" "find find" "find-name find name"
  "analyze analyze" "analyze-calls analyze calls" "neo4j neo4j"
  "neo4j-setup neo4j setup" "version version" "completion completion"
  "report report" "report-capture report capture" "report-validate report validate"
  "doctor doctor"
  "workspace workspace" "workspace-status workspace status"
  "workspace-plan workspace plan" "workspace-sync workspace sync"
  "workspace-index workspace index" "workspace-watch workspace watch"
  "config config" "config-show config show" "config-set config set"
  "config-reset config reset" "config-db config db" "config-validate config validate"
  "service-report service-report"
  "answer-quality-scorecard answer-quality-scorecard"
  "component component" "component-inventory component inventory"
  "component-diagnostics component diagnostics" "component-inspect component inspect"
  "component-verify component verify" "component-install component install"
  "component-list component list" "component-enable component enable"
  "component-disable component disable" "component-uninstall component uninstall"
  "component-conform component conform" "component-init component init"
  "component-init-collector component init collector" "component-index component index"
  "component-index-verify component index verify"
  "component-extraction-readiness component extraction-readiness"
  "component-schema-versions component schema-versions"
  "component-grant component grant" "component-revoke-grant component revoke-grant"
  "component-grants component grants"
  "mcp mcp" "mcp-setup mcp setup" "mcp-tools mcp tools" "mcp-start mcp start"
)
# server cases: "<case-suffix> <expected-exit> <args...>"
SERVER_CASES=(
  "query 0 query hello"
  "list 0 list"
  "stats 0 stats"
  "index-status 0 index-status"
  "ecosystem-overview 0 ecosystem overview"
  "find-name 0 find name foo"
  "freshness-generations 0 freshness generations --json"
  "freshness-changed-since 0 freshness changed-since --repository r --json"
  "freshness-service-changed-since 0 freshness service-changed-since --service-id s --json"
  "playbooks-list 0 playbooks list"
  "playbooks-resolve 0 playbooks resolve pb --input k=v"
  "change-impact 0 change impact --repo-id r --file foo.go --json"
  "change-plan 0 change plan --repo-id r --file foo.go --json"
  "map 0 map --from X --json"
  "trace-service 0 trace service svc --json"
  "analyze-calls 0 analyze calls fn"
  "query-503 1 query HOSTILE_503"
  "query-400 1 query HOSTILE_400"
  "trace-404 2 trace service HOSTILE_404 --json"
  "trace-501 6 trace service HOSTILE_501 --json"
  "trace-ambiguous 3 trace service ENV_AMBIGUOUS --json"
  "trace-building 4 trace service ENV_BUILDING --json"
  "trace-partial 5 trace service ENV_PARTIAL --json"
  "trace-unsupported 6 trace service ENV_UNSUPPORTED --json"
  "map-404 2 map --from HOSTILE_404 --json"
  "map-501 6 map --from HOSTILE_501 --json"
  "change-404 2 change impact --repo-id r --file HOSTILE_404 --json"
  "freshness-503 1 freshness generations --scope-id HOSTILE_503 --json"
  "report-capture 0 report capture --endpoint /api/v0/q --params {\"x\":\"1\"} --note parity"
  "report-validate-pass 0 report validate --from testdata/parity/inputs/bundle-pass.json"
  "report-validate-strict 0 report validate --from testdata/parity/inputs/bundle-pass.json --require-public"
  "report-validate-fail 1 report validate --from testdata/parity/inputs/bundle-fail.json"
  "service-report 0 service-report --from testdata/parity/inputs/dossier.json"
  "service-report-json 0 service-report --from testdata/parity/inputs/dossier.json --json"
  "service-report-supply 0 service-report --from testdata/parity/inputs/dossier.json --supply-chain-from testdata/parity/inputs/supply.json"
  "scorecard-pass 0 answer-quality-scorecard --from testdata/parity/inputs/evidence-pass.json"
  "scorecard-pass-json 0 answer-quality-scorecard --from testdata/parity/inputs/evidence-pass.json --json"
  "scorecard-fail 1 answer-quality-scorecard --from testdata/parity/inputs/evidence-fail.json"
  "component-inventory 0 component inventory"
  "component-inventory-json 0 component inventory --json"
  "component-diagnostics 0 component diagnostics demo.ext"
  "workspace-status 0 workspace status"
)
# local cases (no server): "<case-suffix> <expected-exit> <args...>"
LOCAL_CASES=(
  "map-no-from 2 map"
  "query-no-args 1 query"
  "change-no-repoid 2 change impact --file foo.go --json"
  "trace-no-name 1 trace service"
  "trace-empty-name 2 trace service __EMPTY__"
  "playbooks-bad-input 1 playbooks resolve pb --input badinput"
  "report-no-endpoint 2 report capture"
  "report-credential 2 report capture --endpoint https://svc:C4N4RY@example.com/api"
  "report-bad-endpoint 2 report capture --endpoint ::::::"
  "report-bad-params 1 report capture --endpoint /api/v0/q --params {bad"
  "report-bad-method 1 report capture --endpoint /api/v0/q --method PUT"
  "report-digest-no-scope 1 report"
  "report-digest 0 report --scope repo:demo/app"
  "report-digest-json 0 report --scope repo:demo/app --json"
  "service-report-no-input 1 service-report"
  "service-report-bad-json 1 service-report --from testdata/parity/inputs/bad.json"
  "component-inspect-arg 1 component inspect"
  "component-enable-arg 1 component enable"
  "component-grants-arg 1 component grants extra"
  "mcp-setup-print 0 mcp setup --platform generic"
  "mcp-setup-hosted 0 mcp setup --platform generic --hosted --auth token --service-url http://127.0.0.1:18091"
  "mcp-tools 0 mcp tools"
)
# E3 stub cases (exit 1): top-level names. Tier-2 shrinks the set: report,
# service-report, mcp, m, config, component, and answer-quality-scorecard are
# thin now. Tier-3 removes doctor (moved, closure 3) and workspace (thin
# status plus removed/retained children). investigation stays: its packet
# build is truth assembly in the query closure with no build endpoint, so
# copying it would fork truth.
STUB_CASES=(
  investigation scan vuln-scan
  docs demo demo-benchmark first-run first-run-benchmark
  hosted-setup hosted-onboard assistant
  competitive-parity evidence evidence-packet-dogfood graph
  watch w admin install
)

isolate_home() {
  export ESHU_HOME
  ESHU_HOME="$(mktemp -d)"
  export ESHU_SERVICE_URL="http://127.0.0.1:$PORT"
}

start_server() {
  setsid -f python3 "$SERVER" "$PORT" "$GOLDENS/.requests.log" </dev/null >>"$GOLDENS/.server.log" 2>&1
  for _ in $(seq 1 50); do
    if curl -s -o /dev/null "http://127.0.0.1:$PORT/"; then return 0; fi
    sleep 0.1
  done
  echo "stub server did not start" >&2
  return 1
}

stop_server() {
  pkill -f "stub-server.py $PORT" || true
}

# Per-case extra environment, assigned by the config/verify runners before
# calling run_one and cleared after. Empty by default; run_one prefixes the
# invocation with `env` plus these assignments so cases can pin ESHU_* input
# without touching the ambient environment.
EXTRA_ENV=()

# canon_capture normalizes the capture bundle's wall-clock fields. Capture
# stamps created_at at run time, and bundle_id is a digest over the bundle
# including created_at, so core and dest goldens for the same fixed input
# differ in exactly those two fields by design; everything else (including
# the content digest data_digest and the full response echo) must match.
canon_capture() {
  sed -i -E -e 's/"created_at": "[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z"/"created_at": "CAPTURED_AT"/' -e 's/"bundle_id": "[0-9a-f]{64}"/"bundle_id": "BUNDLE_ID"/' "$1"
}

# run_one BIN PREFIX SUFFIX EXPECTED_EXIT -- ARGS...
run_one() {
  local bin="$1" prefix="$2" suffix="$3" want="$4"
  shift 4
  local name="$prefix-$suffix"
  local out="$GOLDENS/$name.out" errf="$GOLDENS/$name.err" exitf="$GOLDENS/$name.exit"
  mkdir -p "$GOLDENS"
  # __EMPTY__ carries an empty-string argv through the unquoted expansion.
  local translated=()
  local a
  for a in "$@"; do
    if [ "$a" = __EMPTY__ ]; then translated+=(""); else translated+=("$a"); fi
  done
  set -- "${translated[@]}"
  if [ "$MODE" = capture ]; then
    set +e
    env "${EXTRA_ENV[@]}" "$bin" "$@" >"$out" 2>"$errf"
    echo -n "$?" >"$exitf"
    set -e
    case "$suffix" in
      *report-capture*) canon_capture "$out" ;;
      *local-doctor*) canon_doctor "$ESHU_HOME" <"$out" >"$out.canon" && mv "$out.canon" "$out" ;;
    esac
    if [ "$(cat "$exitf")" != "$want" ]; then
      echo "NOTE: $name exited $(cat "$exitf"), want $want"
    fi
    return 0
  fi
  local tmp_o tmp_e got
  tmp_o="$(mktemp)"
  tmp_e="$(mktemp)"
  set +e
  env "${EXTRA_ENV[@]}" "$bin" "$@" >"$tmp_o" 2>"$tmp_e"
  got="$?"
  set -e
  case "$suffix" in
    *report-capture*) canon_capture "$tmp_o" ;;
    *local-doctor*) canon_doctor "$ESHU_HOME" <"$tmp_o" >"$tmp_o.canon" && mv "$tmp_o.canon" "$tmp_o" ;;
  esac
  if [ "$got" != "$want" ]; then
    echo "FAIL(exit): $name got $got want $want"
    echo "--- stdout ---"; cat "$tmp_o"
    echo "--- stderr ---"; cat "$tmp_e"
    FAIL=1
  elif ! cmp -s "$tmp_o" "$out"; then
    echo "FAIL(stdout): $name"
    diff "$out" "$tmp_o" | head -20
    FAIL=1
  elif ! cmp -s "$tmp_e" "$errf"; then
    echo "FAIL(stderr): $name"
    diff "$errf" "$tmp_e" | head -20
    FAIL=1
  else
    echo "PASS: $name"
  fi
  rm -f "$tmp_o" "$tmp_e"
}

capture_help() {
  local bin="$1" prefix="$2"
  local entry suffix
  for entry in "${HELP_CASES[@]}"; do
    suffix="${entry%% *}"
    # shellcheck disable=SC2086
    run_one "$bin" "$prefix" "help-$suffix" 0 ${entry#* } --help
  done
}

capture_server() {
  local bin="$1" prefix="$2"
  local entry rest suffix want
  for entry in "${SERVER_CASES[@]}"; do
    rest="${entry#* }"
    suffix="${entry%% *}"
    want="${rest%% *}"
    # shellcheck disable=SC2086
    run_one "$bin" "$prefix" "server-$suffix" "$want" ${rest#* }
  done
}

capture_local() {
  local bin="$1" prefix="$2"
  local entry rest suffix want
  for entry in "${LOCAL_CASES[@]}"; do
    rest="${entry#* }"
    suffix="${entry%% *}"
    want="${rest%% *}"
    # shellcheck disable=SC2086
    run_one "$bin" "$prefix" "local-$suffix" "$want" ${rest#* }
  done
}

capture_stubs() {
  local bin="$1" prefix="$2"
  local name
  for name in "${STUB_CASES[@]}"; do
    run_one "$bin" "$prefix" "stub-$name" 1 "$name"
  done
}

# canon_doctor rewrites the isolated ESHU_HOME out of the doctor report. The
# home dir is a fresh mktemp per run (missing .env by design), so the raw
# bytes pin the machine that captured them; the placeholder keeps the
# core-vs-dest comparison on the report shape instead of the temp path.
canon_doctor() {
  local home="$1"
  local esc
  esc="$(printf '%s' "$home" | sed 's/[][\\.*^$/]/\\&/g')"
  sed "s|$esc|__ESHU_HOME__|g"
}

# E4 index cases need PATH surgery without breaking tool lookup: run the
# binary through absolute /usr/bin/env so mkdir/cmp/diff keep working.
# The index verb prints the absolute target directory, so stdout is
# normalized with the checkout root rewritten to __WORKDIR__ before it is
# captured or compared: otherwise the golden pins the machine that ran the
# capture and the lane can only pass there (eshu-hq/eshu-cli#23 P1).
WORKDIR_PLACEHOLDER="__WORKDIR__"
canon_workdir() {
  local root="$1"
  local esc
  esc="$(printf '%s' "$root" | sed 's/[][\\.*^$/]/\\&/g')"
  sed "s|$esc|$WORKDIR_PLACEHOLDER|g"
}
capture_e4() {
  local bin="$1" prefix="$2" empty fake
  empty="$(mktemp -d)"
  fake="$(mktemp -d)"
  printf '#!/bin/sh\necho "fake-bootstrap-index $@"\nexit 7\n' >"$fake/eshu-bootstrap-index"
  chmod +x "$fake/eshu-bootstrap-index"
  local name out errf exitf
  name="$prefix-local-index-miss"
  out="$GOLDENS/$name.out"; errf="$GOLDENS/$name.err"; exitf="$GOLDENS/$name.exit"
  if [ "$MODE" = capture ]; then
    set +e
    /usr/bin/env PATH="$empty" "$bin" index . 2>"$errf" | canon_workdir "$PWD" >"$out"
    echo -n "$?" >"$exitf"
    /usr/bin/env PATH="$fake:/usr/bin:/bin" "$bin" index . 2>"$GOLDENS/$prefix-local-index-exec.err" | canon_workdir "$PWD" >"$GOLDENS/$prefix-local-index-exec.out"
    echo -n "$?" >"$GOLDENS/$prefix-local-index-exec.exit"
    set -e
    return 0
  fi
  local tmp_o tmp_e got
  tmp_o="$(mktemp)"; tmp_e="$(mktemp)"
  set +e
  /usr/bin/env PATH="$empty" "$bin" index . 2>"$tmp_e" | canon_workdir "$PWD" >"$tmp_o"
  got="$?"
  set -e
  if [ "$got" != 1 ] || ! cmp -s "$tmp_o" "$out" || ! cmp -s "$tmp_e" "$errf"; then
    echo "FAIL: $name (got exit $got)"
    diff "$out" "$tmp_o" | head; diff "$errf" "$tmp_e" | head
    FAIL=1
  else
    echo "PASS: $name"
  fi
  set +e
  /usr/bin/env PATH="$fake:/usr/bin:/bin" "$bin" index . 2>"$tmp_e" | canon_workdir "$PWD" >"$tmp_o"
  got="$?"
  set -e
  if [ "$got" != 7 ] || ! cmp -s "$tmp_o" "$GOLDENS/$prefix-local-index-exec.out" || ! cmp -s "$tmp_e" "$GOLDENS/$prefix-local-index-exec.err"; then
    echo "FAIL: $prefix-local-index-exec (got exit $got)"
    diff "$GOLDENS/$prefix-local-index-exec.out" "$tmp_o" | head
    diff "$GOLDENS/$prefix-local-index-exec.err" "$tmp_e" | head
    FAIL=1
  else
    echo "PASS: $prefix-local-index-exec"
  fi
  rm -f "$tmp_o" "$tmp_e"
}

# Tier-3 doctor case runs in a clean room (env -i): the report names the
# config directory, PATH-resolved service binaries, the probe target, and the
# configured credentials, so the golden must not depend on the operator's
# HOME, PATH, or ESHU_*. ESHU_HOME points at the isolated dir (missing by
# design), PATH pins /usr/bin:/bin (no service binaries), and the probe
# targets the stub server, which answers /health with 200.
capture_doctor() {
  local bin="$1" prefix="$2"
  EXTRA_ENV=(-i "ESHU_HOME=$ESHU_HOME" "ESHU_SERVICE_URL=http://127.0.0.1:$PORT" PATH=/usr/bin:/bin)
  run_one "$bin" "$prefix" "local-doctor" 0 doctor
  EXTRA_ENV=()
}

# Tier-2 config cases run in a clean room (env -i): `config validate` reads
# the ambient process environment, so the golden must not depend on whatever
# ESHU_* the operator happens to export. The reference case pins the
# published client table the thin command validates against.
capture_config() {
  local bin="$1" prefix="$2"
  EXTRA_ENV=(-i)
  run_one "$bin" "$prefix" "local-config-validate-ok" 0 config validate
  EXTRA_ENV=(-i ESHU_MCP_TRANSPORT=http ESHU_QUERY_PROFILE=production ESHU_MCP_TRANSPOR=http)
  run_one "$bin" "$prefix" "local-config-validate-fixture" 0 config validate
  EXTRA_ENV=(-i ESHU_SITE_LOCAL_EXPERIMENT=1)
  run_one "$bin" "$prefix" "local-config-validate-strict" 0 config validate --strict
  EXTRA_ENV=()
}

# Tier-2 reference output is dest-pinned: the thin command prints the
# published client table while the core binary prints the full platform
# table, by S2 design (the CLI validates exactly the variables it owns).
capture_reference() {
  local bin="$1" prefix="$2"
  EXTRA_ENV=(-i)
  run_one "$bin" "$prefix" "local-config-reference" 0 config validate --reference
  EXTRA_ENV=()
}

# Tier-2 hosted verify is dest-pinned (cli goldens): the tools-visible stage
# reports the deployment's advertised count while the core binary reports its
# embedded build count, so the two can never match byte-for-byte. Everything
# else about the run (stage markers, structure, exit 0) is pinned here; the
# shared stage mechanics stay covered by unit tests.
capture_verify() {
  local bin="$1" prefix="$2"
  EXTRA_ENV=(ESHU_MCP_TOKEN=parity-token)
  run_one "$bin" "$prefix" "server-mcp-verify-hosted" 0 mcp setup --platform generic --hosted --verify --service-url "http://127.0.0.1:$PORT"
  EXTRA_ENV=()
}

MODE=check
FAIL=0
CMD="${1:-check}"

case "$CMD" in
  capture-core)
    MODE=capture
    BIN="${2:?usage: capture-core BIN}"
    isolate_home
    capture_help "$BIN" core
    start_server
    trap stop_server EXIT
    capture_server "$BIN" core
    capture_local "$BIN" core
    capture_doctor "$BIN" core
    capture_config "$BIN" core
    ;;
  capture-cli)
    MODE=capture
    isolate_home
    go build -o "$GOLDENS/.cli-eshu" ./cmd/eshu
    BIN="$GOLDENS/.cli-eshu"
    run_one "$BIN" cli "help-root" 0 --help
    capture_stubs "$BIN" cli
    capture_e4 "$BIN" cli
    capture_reference "$BIN" cli
    start_server
    trap stop_server EXIT
    capture_verify "$BIN" cli
    ;;
  check)
    isolate_home
    go build -o "$GOLDENS/.cli-eshu" ./cmd/eshu
    BIN="$GOLDENS/.cli-eshu"
    capture_help "$BIN" core
    run_one "$BIN" cli "help-root" 0 --help
    capture_reference "$BIN" cli
    start_server
    trap stop_server EXIT
    capture_server "$BIN" core
    capture_local "$BIN" core
    capture_doctor "$BIN" core
    capture_config "$BIN" core
    capture_verify "$BIN" cli
    capture_stubs "$BIN" cli
    capture_e4 "$BIN" cli
    if [ "$FAIL" != 0 ]; then
      echo "PARITY FAILED"
      exit 1
    fi
    echo "PARITY PASSED"
    ;;
  *)
    echo "usage: $0 {check|capture-core BIN|capture-cli}" >&2
    exit 2
    ;;
esac
