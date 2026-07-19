# Contributing to specd

A fast path for your first change. For the architecture, the domain map, and the concurrency /
durability model, read [docs/contributor-guide.md](docs/contributor-guide.md).

## Setup

Requires **Go 1.26+** (the `go` directive in `go.mod`). No other tooling is needed to build or
test — specd is standard-library-only with zero runtime dependencies.

```bash
git clone https://github.com/0xkhdr/specd && cd specd
go build -o specd .      # single static binary
go run . help            # try it
```

## The loop

1. Branch off `main`.
2. Make the change. Keep the diff small; prefer cutting over adding.
3. Run the full gate set locally — the same gates CI runs — with one script:

   ```bash
   ./scripts/ci-local.sh
   ```

   It mirrors the CI pipeline (gofmt, go vet, go mod tidy, test-lint, docs-lint,
   install-script tests, `go test -race`, order-dependence rerun, perf-gate,
   coverage floor, staticcheck via golangci-lint, govulncheck, shellcheck). The
   three external tools are skipped with a notice if not installed locally; CI
   always runs them.

   The full testing reference — coverage floor, regression harnesses, stress jobs — is in
   [TESTING.md](TESTING.md).
4. Open a PR. CI must be green.

## House rules (non-negotiable)

These are the whole point of the tool — a change that breaks one will be rejected:

- **No LLM** in any gate, DAG, or report path — they are pure functions of on-disk state.
- **Evidence integrity** — a task completes only against a passing verify record. **Never add a
  bypass flag.**
- **Zero runtime dependencies** — keep `go.mod`/`go.sum` tidy.
- **Docs sync** — `docs/command-reference.md` is generated from the palette by `tools/gendocs`.
  If you touch a verb or flag, regenerate it with `go run ./tools/gendocs` (`docs-lint.sh` fails
  on drift).
- **Never touch `reference/`** — it is a frozen v1 museum.

New behaviour needs a test. Follow the structural test conventions in
[docs/contributor-guide.md](docs/contributor-guide.md); `./scripts/test-lint.sh` enforces them.
