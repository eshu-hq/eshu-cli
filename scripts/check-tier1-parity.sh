#!/bin/bash
# Tier-1 parity over the populated CLI (Refs eshu-hq/eshu#8874).
#
#   scripts/check-tier1-parity.sh check              # build dest, diff vs goldens
#   scripts/check-tier1-parity.sh capture-core BIN   # capture core-side goldens
#   scripts/check-tier1-parity.sh capture-cli        # capture dest-pinned goldens
#
# Goldens live in testdata/parity/goldens/<case>.{out,err,exit}. Cases whose
# name starts with "core-" hold core CLI bytes at the pinned core SHA
# (73418463ed, S1 merged): the destination must reproduce stdout, stderr and
# exit code byte-for-byte. Cases starting with "cli-" pin sanctioned diffs:
# the E3 stub set, the E4 index discovery contract, and root help without the
# RUNTIME launchers (the E5 per-family help deltas).
set -euo pipefail
cd "$(dirname "$0")/.."

GOLDENS=testdata/parity/goldens
SERVER=testdata/parity/stub-server.py
PORT=18091

# help cases: "<case-suffix> <args...>" (--help appended by the runner).
# Root help is NOT here: it is a cli-pinned sanctioned diff.
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
)
# local cases (no server): "<case-suffix> <expected-exit> <args...>"
LOCAL_CASES=(
  "map-no-from 2 map"
  "query-no-args 1 query"
  "change-no-repoid 2 change impact --file foo.go --json"
  "trace-no-name 1 trace service"
  "trace-empty-name 2 trace service __EMPTY__"
  "playbooks-bad-input 1 playbooks resolve pb --input badinput"
)
# E3 stub cases (exit 1): top-level names
STUB_CASES=(
  report investigation mcp m config component scan vuln-scan
  service-report docs demo demo-benchmark first-run first-run-benchmark
  hosted-setup hosted-onboard assistant answer-quality-scorecard
  competitive-parity evidence evidence-packet-dogfood workspace graph
  watch w admin doctor install
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
    "$bin" "$@" >"$out" 2>"$errf"
    echo -n "$?" >"$exitf"
    set -e
    if [ "$(cat "$exitf")" != "$want" ]; then
      echo "NOTE: $name exited $(cat "$exitf"), want $want"
    fi
    return 0
  fi
  local tmp_o tmp_e got
  tmp_o="$(mktemp)"
  tmp_e="$(mktemp)"
  set +e
  "$bin" "$@" >"$tmp_o" 2>"$tmp_e"
  got="$?"
  set -e
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
    ;;
  capture-cli)
    MODE=capture
    isolate_home
    go build -o "$GOLDENS/.cli-eshu" ./cmd/eshu
    BIN="$GOLDENS/.cli-eshu"
    run_one "$BIN" cli "help-root" 0 --help
    capture_stubs "$BIN" cli
    capture_e4 "$BIN" cli
    ;;
  check)
    isolate_home
    go build -o "$GOLDENS/.cli-eshu" ./cmd/eshu
    BIN="$GOLDENS/.cli-eshu"
    capture_help "$BIN" core
    run_one "$BIN" cli "help-root" 0 --help
    start_server
    trap stop_server EXIT
    capture_server "$BIN" core
    capture_local "$BIN" core
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
