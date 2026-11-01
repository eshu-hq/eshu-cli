// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

// Package ciworkflow holds the contract test for .github/workflows/ci.yml:
// the lane bodies live in the pinned eshu-hq/.github reusable, and this
// caller keeps its path classifier, the lanes call, and the required
// aggregator. The checks are plain string reads so the module stays
// dependency-free (Refs eshu-hq/eshu-cli#1).
package ciworkflow

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// reusablePin is the exact eshu-hq/.github commit the caller must pin (the
// peeled v2 tag). A pin move fails the tests until the new reusable body is
// re-verified.
const reusablePin = "eshu-hq/.github/.github/workflows/go-collector-ci.yml@4d01ab90baf5dd77c21cc1ee6bdf701332613449"

// actionlintSHA256 is the sha256 of actionlint 1.7.12's linux_amd64 tarball.
const actionlintSHA256 = "8aca8db96f1b94770f1b0d72b6dddcb1ebb8123cb3712530b08cc387b349a3d8"

// aggregatorName is the only status context the repo ruleset requires.
// Renaming it makes every PR unmergeable.
const aggregatorName = "ci-complete (required aggregator)"

// retiredGates pins every run-* input to the exact expression the retired
// inline lane used as its `if:`. A flipped or widened gate turns the suite
// red, so lane selection cannot go dark (or wide) unnoticed.
var retiredGates = map[string]string{
	"run-lint":        "${{ needs.changes.outputs.go == 'true' || needs.changes.outputs.deps == 'true' || needs.changes.outputs.workflow == 'true' }}",
	"run-test":        "${{ needs.changes.outputs.go == 'true' || needs.changes.outputs.deps == 'true' || needs.changes.outputs.fixtures == 'true' || needs.changes.outputs.workflow == 'true' }}",
	"run-race":        "${{ needs.changes.outputs.go == 'true' || needs.changes.outputs.deps == 'true' || needs.changes.outputs.gate == 'true' || needs.changes.outputs.workflow == 'true' }}",
	"run-shell":       "${{ needs.changes.outputs.shell == 'true' || needs.changes.outputs.workflow == 'true' }}",
	"run-image":       "${{ needs.changes.outputs.image == 'true' || needs.changes.outputs.workflow == 'true' }}",
	"run-actionlint":  "${{ needs.changes.outputs.workflow == 'true' }}",
	"run-govulncheck": "${{ needs.changes.outputs.deps == 'true' || github.event_name == 'schedule' }}",
	"run-gosec":       "${{ needs.changes.outputs.go == 'true' || needs.changes.outputs.deps == 'true' || github.event_name == 'schedule' }}",
	"run-boundary":    "false",
}

// toolPins are the tool versions the retired lanes pinned, plus the lane
// settings that must survive the move.
var toolPins = map[string]string{
	"module":                "eshu-cli",
	"go-version-file":       "go.mod",
	"coverage-floor":        "90.2",
	"golangci-lint-version": "v2.12.2",
	"govulncheck-version":   "v1.9.0",
	"gosec-version":         "v2.30.0",
	"actionlint-version":    "1.7.12",
	"actionlint-sha256":     actionlintSHA256,
	"image-tag":             "eshu-cli:ci",
	"image-context":         ".",
}

var (
	jobHeader    = regexp.MustCompile(`^  ([A-Za-z][A-Za-z0-9_-]*):\s*$`)
	withInput    = regexp.MustCompile(`^      ([a-z][a-z0-9-]*): (.+)$`)
	needsLine    = regexp.MustCompile(`(?m)^    needs: \[(.*)\]\s*$`)
	usesLine     = regexp.MustCompile(`(?m)^\s*(?:- )?uses: (\S+)(.*)$`)
	pinnedAction = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._/-]+@[0-9a-f]{40}$`)
	secretUse    = regexp.MustCompile(`secrets\.([A-Za-z0-9_]+)`)
	floorLine    = regexp.MustCompile(`(?m)^COVERAGE_FLOOR\s*\??=\s*["']?([0-9.]+)["']?`)
	goDirective  = regexp.MustCompile(`(?m)^go ([0-9][0-9.]*)\s*$`)
	dockerGoArg  = regexp.MustCompile(`(?m)^ARG GO_VERSION=([0-9][0-9.]*)\s*$`)
)

func readFile(t *testing.T, path string) string {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// stripComments drops full-line comments so explanatory header text never
// satisfies (or trips) an executable-shape check.
func stripComments(text string) string {
	var b strings.Builder
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// jobBlocks splits ci.yml into its top-level jobs, returning the names in
// file order and each job's body (comments included).
func jobBlocks(t *testing.T) ([]string, map[string]string) {
	t.Helper()

	var (
		names  []string
		blocks = map[string][]string{}
		inJobs bool
		cur    string
	)
	for _, line := range strings.Split(readFile(t, ".github/workflows/ci.yml"), "\n") {
		if line == "jobs:" {
			inJobs = true
			continue
		}
		if !inJobs {
			continue
		}
		if m := jobHeader.FindStringSubmatch(line); m != nil {
			cur = m[1]
			names = append(names, cur)
			continue
		}
		if cur != "" {
			blocks[cur] = append(blocks[cur], line)
		}
	}
	out := make(map[string]string, len(blocks))
	for name, lines := range blocks {
		out[name] = "\n" + strings.Join(lines, "\n")
	}
	return names, out
}

func block(t *testing.T, name string) string {
	t.Helper()

	_, blocks := jobBlocks(t)
	body, ok := blocks[name]
	if !ok {
		t.Fatalf("ci.yml has no %q job", name)
	}
	return body
}

// withInputs returns the `with:` inputs of a job body (six-space indent).
func withInputs(body string) map[string]string {
	inputs := map[string]string{}
	inWith := false
	for _, line := range strings.Split(body, "\n") {
		if line == "    with:" {
			inWith = true
			continue
		}
		if !inWith {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Continuation lines of a block scalar (image-command) sit deeper
		// than the input keys.
		if strings.HasPrefix(line, "       ") {
			continue
		}
		m := withInput.FindStringSubmatch(line)
		if m == nil {
			break
		}
		inputs[m[1]] = m[2]
	}
	return inputs
}

// TestCallerCallsPinnedReusable pins the thin-caller shape: lanes uses the
// reusable at an exact SHA with the tag as a trailing comment, every other
// action is SHA-pinned, and no inline lane body remains.
func TestCallerCallsPinnedReusable(t *testing.T) {
	t.Parallel()

	lanes := block(t, "lanes")
	if !strings.Contains(lanes, "\n    uses: "+reusablePin+" # v2\n") {
		t.Errorf("lanes does not call %s with a trailing # v2 comment", reusablePin)
	}
	if strings.Contains(lanes, "\n    steps:") {
		t.Error("lanes carries inline steps: lane bodies live in the reusable")
	}

	text := stripComments(readFile(t, ".github/workflows/ci.yml"))
	for _, m := range usesLine.FindAllStringSubmatch(text, -1) {
		if !pinnedAction.MatchString(m[1]) {
			t.Errorf("uses %q, want a full commit SHA pin", m[1])
		}
		if !strings.Contains(m[2], "# ") {
			t.Errorf("uses %q has no trailing version comment", m[1])
		}
	}
	for _, inline := range []string{
		"golangci-lint run", "golangci-lint fmt", "go test ./... -race",
		"govulncheck ./...", "gosec -fmt", "go install github.com/rhysd/actionlint",
		"actions/setup-go", "actions/upload-artifact", "codeql-action/upload-sarif",
		"apt-get install", "bash scripts/coverage-gate.sh",
	} {
		if strings.Contains(text, inline) {
			t.Errorf("ci.yml still carries inline lane body %q: it lives in the reusable", inline)
		}
	}
}

// TestCallerKeepsFrozenJobNames keeps the org contract: the classifier, the
// reusable call, and the only ruleset-required context.
func TestCallerKeepsFrozenJobNames(t *testing.T) {
	t.Parallel()

	got, _ := jobBlocks(t)
	want := []string{"changes", "lanes", "ci-complete"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("ci.yml jobs = %v, want exactly %v", got, want)
	}
	if !strings.Contains(block(t, "changes"), "\n    name: Detect changed paths\n") {
		t.Error("changes lost its display name `Detect changed paths`")
	}
	if !strings.Contains(block(t, "ci-complete"), "\n    name: "+aggregatorName+"\n") {
		t.Errorf("ci-complete must keep name %q: the ruleset requires that exact context", aggregatorName)
	}
}

// TestChangesRunsOnEveryEvent guards the schedule trap: every lane needs the
// changes job, so an if: guard would skip every lane on a scheduled run.
func TestChangesRunsOnEveryEvent(t *testing.T) {
	t.Parallel()

	body := block(t, "changes")
	if strings.Contains(stripComments(body), "\n    if:") {
		t.Error("changes job has an if: guard; every lane would skip when it skips")
	}
	for _, out := range []string{"go", "deps", "fixtures", "gate", "shell", "image", "workflow"} {
		if !strings.Contains(body, "      "+out+": ${{ steps.filter.outputs."+out+" }}") {
			t.Errorf("changes drops output %q that a lane gate reads", out)
		}
		if !strings.Contains(body, "\n            "+out+":\n") {
			t.Errorf("changes filter lost the %q path class", out)
		}
	}
}

// TestLanesKeepRetiredGates fails if any lane selector drifts from the
// expression its retired inline lane used, or is dropped (the lane would
// silently un-run).
func TestLanesKeepRetiredGates(t *testing.T) {
	t.Parallel()

	lanes := block(t, "lanes")
	if !strings.Contains(lanes, "\n    needs: [changes]\n") {
		t.Error("lanes must need exactly [changes]")
	}
	inputs := withInputs(lanes)
	for input, want := range retiredGates {
		if got, ok := inputs[input]; !ok || got != want {
			t.Errorf("lanes input %q = %q, want the retired gate %q", input, got, want)
		}
	}
	if got, ok := inputs["boundary-command"]; ok {
		t.Errorf("lanes sets boundary-command %q: this repo has no boundary lane", got)
	}
	if got, ok := inputs["gosec-fail-on-findings"]; ok {
		t.Errorf("lanes sets gosec-fail-on-findings %q: gosec stays advisory", got)
	}
}

// TestLanesKeepToolPins keeps every tool pin, the coverage floor, and the
// actionlint checksum the retired lanes carried, plus the image smoke test.
func TestLanesKeepToolPins(t *testing.T) {
	t.Parallel()

	lanes := block(t, "lanes")
	inputs := withInputs(lanes)
	for input, want := range toolPins {
		if got, ok := inputs[input]; !ok || got != want {
			t.Errorf("lanes input %q = %q, want %q", input, got, want)
		}
	}
	for _, smoke := range []string{
		"docker run --rm eshu-cli:ci --help",
		"docker run --rm eshu-cli:ci version",
	} {
		if !strings.Contains(lanes, smoke) {
			t.Errorf("lanes image-command lost the smoke test %q", smoke)
		}
	}
}

// aggregatorProblems returns every way run breaks the ci-complete contract:
// compare each job's result (whole-object comparison never matches and a
// bare needs.*.result array breaks green runs), fail failed and cancelled
// lanes, and say skips are allowed.
func aggregatorProblems(run string) []string {
	var problems []string
	if !strings.Contains(run, `v.get("result")`) {
		problems = append(problems, `aggregator must compare v.get("result") per job, not whole job objects (false-green)`)
	}
	if strings.Contains(run, "join(needs.*.result") {
		problems = append(problems, "aggregator must not expand needs.*.result (bare array breaks green runs)")
	}
	for _, want := range []string{`"failure"`, `"cancelled"`, "skip", "toJSON(needs)"} {
		if !strings.Contains(run, want) {
			problems = append(problems, "aggregator never mentions "+want)
		}
	}
	return problems
}

// TestCICompleteAggregatesEveryLane keeps ci-complete the only required
// context: it always runs, needs the classifier and the lanes call, and its
// aggregator keeps the per-job result shape.
func TestCICompleteAggregatesEveryLane(t *testing.T) {
	t.Parallel()

	body := block(t, "ci-complete")
	m := needsLine.FindStringSubmatch(body)
	if m == nil {
		t.Fatal("ci-complete has no needs list")
	}
	needed := map[string]bool{}
	for _, name := range strings.Split(m[1], ",") {
		needed[strings.TrimSpace(name)] = true
	}
	for _, name := range []string{"changes", "lanes"} {
		if !needed[name] {
			t.Errorf("ci-complete needs %q missing: its failure would not fail the required context", name)
		}
	}
	if !strings.Contains(body, "\n    if: always()\n") {
		t.Error("ci-complete must run with if: always()")
	}
	if problems := aggregatorProblems(stripComments(body)); len(problems) > 0 {
		t.Errorf("ci-complete aggregator breaks its contract:\n%s", strings.Join(problems, "\n"))
	}

	for name, run := range map[string]string{
		"bare needs array": "RESULTS: ${{ join(needs.*.result, ' ') }}\nfor result in $RESULTS; do :; done",
		"whole objects":    `bad = {k: v for k, v in needs.items() if v == "failure"}`,
	} {
		if len(aggregatorProblems(run)) == 0 {
			t.Errorf("%s: the gate accepted a violating aggregator", name)
		}
	}
}

// TestCoverageFloorParity keeps `make ci` honest: the Makefile floor and the
// lanes coverage-floor input must name the same number.
func TestCoverageFloorParity(t *testing.T) {
	t.Parallel()

	makeFloor := floorLine.FindStringSubmatch(readFile(t, "Makefile"))
	ciFloor, ok := withInputs(block(t, "lanes"))["coverage-floor"]
	if makeFloor == nil || !ok {
		t.Fatalf("COVERAGE_FLOOR literal missing (Makefile=%v lanes=%q)", makeFloor, ciFloor)
	}
	if makeFloor[1] != ciFloor {
		t.Errorf("COVERAGE_FLOOR drift: Makefile=%s lanes=%s, want one shared value", makeFloor[1], ciFloor)
	}
}

// TestImageToolchainMatchesGoMod keeps the image lane honest: the reusable's
// image lane runs a bare docker build with no --build-arg, so the Dockerfile
// default toolchain must equal the go directive CI's other lanes read.
func TestImageToolchainMatchesGoMod(t *testing.T) {
	t.Parallel()

	goMod := goDirective.FindStringSubmatch(readFile(t, "go.mod"))
	docker := dockerGoArg.FindStringSubmatch(readFile(t, "Dockerfile"))
	if goMod == nil || docker == nil {
		t.Fatalf("go directive or Dockerfile ARG GO_VERSION missing (go.mod=%v Dockerfile=%v)", goMod, docker)
	}
	if goMod[1] != docker[1] {
		t.Errorf("toolchain drift: go.mod=%s Dockerfile ARG GO_VERSION=%s", goMod[1], docker[1])
	}
}

// TestCallerSafetyContract keeps fork PRs working and hung runners bounded:
// contents: read only, one cancelling concurrency group per PR, no secret
// beyond the automatic GITHUB_TOKEN, and a timeout on every job that can
// carry one (the schema forbids it on the reusable call).
func TestCallerSafetyContract(t *testing.T) {
	t.Parallel()

	text := readFile(t, ".github/workflows/ci.yml")
	if !strings.Contains(text, "\npermissions:\n  contents: read\n") {
		t.Error("top-level permissions must be contents: read")
	}
	if !strings.Contains(text, "cancel-in-progress: ${{ github.event_name == 'pull_request' }}") {
		t.Error("concurrency must cancel superseded PR runs")
	}
	for _, m := range secretUse.FindAllStringSubmatch(stripComments(text), -1) {
		if m[1] != "GITHUB_TOKEN" {
			t.Errorf("ci.yml uses configured secret %q, want none", m[1])
		}
	}

	names, blocks := jobBlocks(t)
	for _, name := range names {
		has := strings.Contains(blocks[name], "\n    timeout-minutes:")
		if name == "lanes" && has {
			t.Error("lanes sets timeout-minutes: the schema forbids it on reusable calls")
		}
		if name != "lanes" && !has {
			t.Errorf("job %s sets no timeout-minutes", name)
		}
	}
}

// fork-pr lane proof, do not merge
