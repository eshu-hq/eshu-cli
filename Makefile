# eshu-cli local CI parity.
#
# `make ci` runs the same commands as the `lint`, `test`, and `race`
# lanes that .github/workflows/ci.yml takes from the pinned
# eshu-hq/.github go-collector-ci reusable. Go lanes skip with success
# while the repo holds no go.mod, so a docs-only checkout stays green.
# `make race` additionally writes a coverprofile, posts per-package
# totals, and enforces COVERAGE_FLOOR via scripts/coverage-gate.sh; the
# CI race lane enforces the same floor from the same total.
# Part of eshu-hq/eshu-cli#1 (#6 gates race).

GO_FILES := $(shell git ls-files '*.go' 2>/dev/null)
HAS_GO_MOD := $(shell test -f go.mod && echo yes)

# Mirrors the coverage-floor input in ci.yml (#6); ci_workflow_test.go
# fails when the two differ.
COVERAGE_FLOOR ?= 90.2
COVERAGE_PROFILE ?= coverage-race.out

.PHONY: ci lint test race filecap

ci: lint test race

lint: filecap
ifndef HAS_GO_MOD
	@echo "lint: no go.mod, Go checks skip (file cap above still enforced)."
else
	go vet ./...
	"$(shell go env GOPATH)/bin/golangci-lint" run ./...
	"$(shell go env GOPATH)/bin/golangci-lint" fmt --diff ./...
	cp go.mod /tmp/eshu-cli-go.mod.orig
	cp go.sum /tmp/eshu-cli-go.sum.orig
	go mod tidy
	diff /tmp/eshu-cli-go.mod.orig go.mod
	diff /tmp/eshu-cli-go.sum.orig go.sum
endif

test:
ifndef HAS_GO_MOD
	@echo "test: no go.mod, nothing to build or test."
else
	go build ./...
	go test ./... -count=1
endif

race:
ifndef HAS_GO_MOD
	@echo "race: no go.mod, race skips until Go code lands."
else
	go test ./... -race -count=1 -covermode=atomic -coverprofile=$(COVERAGE_PROFILE)
	bash scripts/coverage-gate.sh $(COVERAGE_PROFILE) $(COVERAGE_FLOOR)
endif

# 500-line cap on .go files (plain wc, no plugin). Mirrors the lint
# lane step so the rule fails locally before it fails in CI.
filecap:
	@fail=0; \
	for f in $(GO_FILES); do \
		lines=$$(wc -l < "$$f" | tr -d ' '); \
		if [ "$$lines" -gt 500 ]; then \
			echo "OVER CAP: $$f has $$lines lines (max 500)"; \
			fail=1; \
		fi; \
	done; \
	exit $$fail
