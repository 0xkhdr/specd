# Contributing to specd

Thanks for your interest in improving specd!

The full contributor guide — architecture, repo layout, invariants, and the
review bar — lives in **[`docs/contributor-guide.md`](docs/contributor-guide.md)**.
Testing conventions and the coverage policy are in
**[`TESTING.md`](TESTING.md)**. Please read those before opening a PR.

## Quick start

```sh
git clone https://github.com/0xkhdr/specd.git
cd specd
make ci        # lint + race tests + order-dependence + coverage floor + stress
```

`make ci` is exactly what CI runs; a green `make ci` locally is the bar for a PR.

## Hard invariants (do not break)

- **stdlib-only** at runtime — no third-party runtime dependencies.
- **Zero LLM calls** and **deterministic output** — the binary reasons about
  nothing; it enforces.
- **The Foundational Split** — the agent reasons; the harness enforces.
- **The evidence gate** — no task completes without evidence; non-read-only
  tasks require a passing verify record.

Linters and scanners (golangci-lint/staticcheck, govulncheck) are CI-only dev
tooling and must never become runtime dependencies.

## Pull requests

- Keep changes focused; reference the relevant requirement/issue.
- Add or update tests for behavior changes; never lower a coverage floor to make
  a build pass (see [`TESTING.md`](TESTING.md#coverage-policy)).
- Run `make ci` and ensure it passes before requesting review.

## Security

Do not file security issues publicly — see [`SECURITY.md`](SECURITY.md).
