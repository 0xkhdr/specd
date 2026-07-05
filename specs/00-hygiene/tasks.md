# Tasks — 00-hygiene

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T1 | scout | scripts/, docs/, CLAUDE.md, README.md | | `printf ok` | Inventory of every artifact CLAUDE.md/README reference vs what exists; list of unconsumed config keys with their loader fields (R4, R6, R7) |
| T2 | craftsman | scripts/test-lint.sh | T1 | `./scripts/test-lint.sh` | Structural test lint per CLAUDE.md description: banned suffixes, space-separated subtest names, duplicated helpers; `set -euo pipefail`; violations printed `file:line: msg`; exits 0 on current clean tree (R1) |
| T3 | craftsman | docs/CHEATSHEET.md | T1 | `test -s docs/CHEATSHEET.md` | Cheatsheet covering every live verb + flag from docs/command-reference.md, one-line-per-verb quick reference (R3) |
| T4 | craftsman | scripts/docs-lint.sh | T3 | `./scripts/docs-lint.sh` | Mirror check between CHEATSHEET.md and command-reference.md; mirror rule documented at top of script; non-zero exit on divergence (R2) |
| T5 | craftsman | docs/decisions/ | T1 | `test $(ls docs/decisions/*.md | wc -l) -ge 9` | One ADR each: triage, conductor, dashboard, packs, harness-sharing, ingest, deploy, observe, eval-prototype; each has Status/Context/Decision/Revisit-trigger (R5) |
| T6 | craftsman | internal/cmd/, internal/core/ (config loader files per T1 finding) | T1 | `go test ./... -race -count=1` | Config loader rejects unknown keys with named-key diagnostic, exit 2; unconsumed-but-kept keys covered by ADR from T5; tests for accept/reject paths (R6) |
| T7 | craftsman | CLAUDE.md, README.md | T2,T4,T5 | `bash -c './scripts/test-lint.sh && ./scripts/docs-lint.sh'` | CLAUDE.md/README reference only artifacts that exist; stress*.sh claim resolved per R7 decision; docs-sync invariant true on a fresh clone (R4, R7) |
| T8 | craftsman | .github/workflows/ (CI config) | T2,T4 | `grep -l "test-lint.sh" .github/workflows/*` | CI runs test-lint.sh and docs-lint.sh so drift class cannot recur |
| T9 | validator | (read-only) | T7,T8 | `bash -c 'gofmt -l . | wc -l | grep -qx 0 && go vet ./... && ./scripts/test-lint.sh && ./scripts/docs-lint.sh && go test ./... -race -count=1'` | Every gate CLAUDE.md names passes end-to-end on clean tree |
