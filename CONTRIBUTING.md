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
3. Run the gates locally (all of these run in CI):

   ```bash
   go test ./... -race -count=1
   go test ./... -count=2        # order-dependence catch
   gofmt -l .                    # must be empty
   go vet ./...
   go mod tidy                   # must produce no diff
   ./scripts/test-lint.sh
   ./scripts/docs-lint.sh
   golangci-lint run
   ```

   The full testing reference — coverage floor, regression harnesses, stress jobs — is in
   [TESTING.md](TESTING.md).
4. Open a PR. CI must be green.

## House rules (non-negotiable)

These are the whole point of the tool — a change that breaks one will be rejected:

- **No LLM** in any gate, DAG, or report path — they are pure functions of on-disk state.
- **Evidence integrity** — a task completes only against a passing verify record. **Never add a
  bypass flag.**
- **Zero runtime dependencies** — keep `go.mod`/`go.sum` tidy.
- **Docs sync** — if you touch a verb or flag, update **both** `docs/command-reference.md` and
  `docs/CHEATSHEET.md` (they must stay byte-identical; `docs-lint.sh` enforces it).
- **Never touch `reference/`** — it is a frozen v1 museum.

New behaviour needs a test. Follow the structural test conventions in
[docs/contributor-guide.md](docs/contributor-guide.md); `./scripts/test-lint.sh` enforces them.
