GO      ?= go
VERSION ?= $(shell git describe --tags --dirty --always 2>/dev/null || echo "dev")
LDFLAGS  = -s -w -X main.version=$(VERSION)
BIN      = specd

.PHONY: all build install test test-order cover cover-check fmt-check lint shellcheck stress perf-gate bench ci clean

all: build

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) .

install:
	$(GO) install -ldflags "$(LDFLAGS)" .

test:
	$(GO) test ./... -race -count=1

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

lint: fmt-check shellcheck
	$(GO) vet ./...

shellcheck:
	shellcheck scripts/*.sh

# Cross-process concurrency stress (Stage 07 F6).
stress: build
	./scripts/stress.sh

# Onboarding deterministic-output gate (T26). Byte-stability of init receipts and
# probe contract fields, run twice to catch order/iteration dependence. No
# wall-clock assertions — latency is tracked via `make bench`, not gated.
# Baselines & regression policy: docs/agent-harness-baselines.md.
perf-gate:
	$(GO) test ./internal/cmd/... ./internal/mcp/... -run 'Deterministic|BenchmarkContract' -count=2

# Record onboarding latency baselines (informational; never a CI gate).
bench:
	$(GO) test ./internal/cmd/... ./internal/mcp/... -run '^$$' -bench 'Init|Probe|Detection' -benchmem

# Everything CI runs, locally.
ci: lint test test-order cover-check perf-gate stress

clean:
	rm -f $(BIN) coverage.out coverage-core.out
