#!/usr/bin/env sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

# docs/command-reference.md is generated from the specd help --json palette
# (internal/core/commands.go) by tools/gendocs. Fail if the committed doc has
# drifted from the generator output.
if ! (cd "$root" && go run ./tools/gendocs -check); then
	echo "docs-lint: docs/command-reference.md is stale — run: go run ./tools/gendocs" >&2
	exit 1
fi

# --- Drift guard (SPEC-07 T-07-05): gate count + Go version from one source ---
# Two facts drift because they live in prose in many docs. Pin each to its single
# authoritative on-disk source and fail the lint when a doc disagrees.
cd "$root"
docs="README.md AGENTS.md CLAUDE.md CONTRIBUTING.md TESTING.md CHANGELOG.md SECURITY.md scripts/README.md \
	docs/README.md docs/validation-gates.md docs/concepts.md docs/user-guide.md \
	docs/command-reference.md docs/contributor-guide.md docs/github-action.md \
	docs/agent-integration.md docs/mcp-guide.md docs/open-spec-format.md docs/troubleshooting.md \
	docs/scale-envelope.md docs/observability.md docs/versioning-policy.md"

# 1. Gate count — authoritative source: the registrations in the core registry.
gate_count=$(grep -c 'registry.Register(' internal/core/gates/core.go)
if grep -rhoE '[0-9]+ core (validation )?gates' $docs | grep -qvE "^${gate_count} core (validation )?gates$"; then
	echo "docs-lint: a doc claims a gate count other than the ${gate_count} registered in internal/core/gates/core.go" >&2
	grep -rnoE '[0-9]+ core (validation )?gates' $docs | grep -vE "${gate_count} core (validation )?gates" >&2
	exit 1
fi

# 2. Go floor — authoritative source: the `go` directive in go.mod.
go_floor=$(awk '$1 == "go" { print $2; exit }' go.mod)
if grep -rhoE 'Go 1\.[0-9]+\+' $docs | grep -qvx "Go ${go_floor}+"; then
	echo "docs-lint: a doc claims a Go floor other than go.mod's ${go_floor}" >&2
	grep -rnoE 'Go 1\.[0-9]+\+' $docs | grep -v "Go ${go_floor}+" >&2
	exit 1
fi

# 3. Removed live surfaces must not remain in operator/contributor documentation.
retired='CHEATSHEET\.md|stress-(acp|orchestration|program|brain-recovery|checkpoint-fault)\.sh'
if grep -nE "$retired" $docs; then
	echo "docs-lint: live documentation references a retired path" >&2
	exit 1
fi

echo "docs-lint: ok"
