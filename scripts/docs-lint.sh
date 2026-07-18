#!/usr/bin/env sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

if ! cmp -s "$root/docs/command-reference.md" "$root/docs/CHEATSHEET.md"; then
	echo "docs-lint: docs/CHEATSHEET.md must mirror docs/command-reference.md" >&2
	exit 1
fi

# Historical agent-facing audits intentionally preserve obsolete claims. Keep
# their scope warning prominent so agents do not treat them as live contracts.
historical_agent_doc="$root/AGENT-DRIVEABILITY-ANALYSIS.md"
if ! sed -n '1,12p' "$historical_agent_doc" | grep -Fq 'Historical document — not current operating guidance.' ||
	! sed -n '1,12p' "$historical_agent_doc" | grep -Fq 'scoped to commit `2ccd2a6`' ||
	! sed -n '1,12p' "$historical_agent_doc" | grep -Fq 'specd help --json'; then
	echo "docs-lint: historical agent analysis must identify its commit scope and current guidance" >&2
	exit 1
fi

# --- Drift guard (SPEC-07 T-07-05): gate count + Go version from one source ---
# Two facts drift because they live in prose in many docs. Pin each to its single
# authoritative on-disk source and fail the lint when a doc disagrees.
cd "$root"
docs="README.md CLAUDE.md CONTRIBUTING.md TESTING.md CHANGELOG.md SECURITY.md \
	docs/README.md docs/validation-gates.md docs/concepts.md docs/user-guide.md \
	docs/command-reference.md docs/CHEATSHEET.md docs/contributor-guide.md docs/github-action.md \
	docs/agent-integration.md docs/mcp-guide.md docs/open-spec-format.md docs/troubleshooting.md \
	docs/scale-envelope.md docs/observability.md docs/versioning-policy.md"

# 1. Gate count — authoritative source: the registrations in the core registry.
gate_count=$(grep -c 'registry.Register(' internal/core/gates/core.go)
if grep -rhoE '[0-9]+ core gates' $docs | grep -qvx "${gate_count} core gates"; then
	echo "docs-lint: a doc claims a gate count other than the ${gate_count} registered in internal/core/gates/core.go" >&2
	grep -rnoE '[0-9]+ core gates' $docs | grep -v "${gate_count} core gates" >&2
	exit 1
fi

# 2. Go floor — authoritative source: the `go` directive in go.mod.
go_floor=$(awk '$1 == "go" { print $2; exit }' go.mod)
if grep -rhoE 'Go 1\.[0-9]+\+' $docs | grep -qvx "Go ${go_floor}+"; then
	echo "docs-lint: a doc claims a Go floor other than go.mod's ${go_floor}" >&2
	grep -rnoE 'Go 1\.[0-9]+\+' $docs | grep -v "Go ${go_floor}+" >&2
	exit 1
fi

echo "docs-lint: ok"
