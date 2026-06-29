GO      ?= go
VERSION ?= $(shell git describe --tags --dirty --always 2>/dev/null || echo "dev")
LDFLAGS  = -s -w -X main.version=$(VERSION)
BIN      = specd

.PHONY: all build install test wrapper-test test-order cover cover-check fmt-check lint test-lint shellcheck stress stress-acp stress-orchestration stress-program stress-brain-recovery perf-gate bench ci clean

all: build

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) .

install:
	$(GO) install -ldflags "$(LDFLAGS)" .

test: wrapper-test
	$(GO) test ./... -race -count=1

wrapper-test:
	python3 scripts/test-specd-workflow.py

# Catch golden/iteration-order dependence (Stage 07 F4).
test-order:
	$(GO) test ./... -count=2

cover:
	$(GO) test ./... -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out | tail -1

# Enforce the regression floor (Stage 07 F1).
cover-check:
	./scripts/coverage-check.sh

fmt-check:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then echo "not gofmt-clean:"; echo "$$unformatted"; exit 1; fi

lint: fmt-check shellcheck test-lint
	$(GO) vet ./...

# Structural lint for the test suite (spec.md §7): no banned file suffixes, no
# space-separated subtest names, no duplicate helpers.
test-lint:
	./scripts/test-lint.sh

shellcheck:
	shellcheck scripts/*.sh

# Cross-process concurrency stress (Stage 07 F6).
stress: build
	./scripts/stress.sh

stress-acp:
	./scripts/stress-acp.sh

stress-orchestration:
	./scripts/stress-orchestration.sh

stress-program:
	./scripts/stress-program.sh

stress-brain-recovery:
	./scripts/stress-brain-recovery.sh

# Onboarding deterministic-output gate (T26). Byte-stability of init receipts and
# probe contract fields, run twice to catch order/iteration dependence. No
# wall-clock assertions — latency is tracked via `make bench`, not gated.
# Baselines & regression policy: docs/agent-harness-baselines.md.
perf-gate:
	$(GO) test ./internal/cmd/... ./internal/mcp/... ./internal/context/... -run 'Deterministic|BenchmarkContract|ManifestDisabledMode' -count=2

# Record onboarding latency baselines (informational; never a CI gate).
bench:
	$(GO) test ./internal/cmd/... ./internal/mcp/... -run '^$$' -bench 'Init|Probe|Detection' -benchmem

# Everything CI runs, locally.
ci: lint test test-order cover-check perf-gate stress stress-acp stress-orchestration stress-program stress-brain-recovery

clean:
	rm -f $(BIN) coverage.out coverage-core.out
