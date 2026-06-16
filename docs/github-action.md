# GitHub-native integration

specd ships a composite GitHub Action at `.github/actions/specd-pr` that runs the
validation gates on a pull request and upserts a deterministic summary comment.
The specd binary makes **no network calls**; the only network access is the
Action's GitHub API call to post the comment.

## What it does

1. Renders `specd report <spec> --pr-summary` — wave/task progress, gate status,
   and the commit↔task link map — to a file. Deterministic and network-free.
2. Runs `specd check <spec>`; the gate verdict drives the job status (a failing
   gate fails the check).
3. Upserts a PR comment keyed by a hidden marker (`<!-- specd-pr-summary:<spec> -->`),
   so re-runs update the same comment instead of stacking new ones.

## Workflow snippet

```yaml
name: specd
on:
  pull_request:

# Least privilege: read the code, write only PR comments. No other scopes.
permissions:
  contents: read
  pull-requests: write

jobs:
  specd:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4   # pin to a SHA in production

      # Install specd (stdlib-only Go binary). Build from source or download a
      # pinned release; either way the binary itself touches no network at runtime.
      - uses: actions/setup-go@v5
        with: { go-version: "1.22" }
      - run: go build -o /usr/local/bin/specd .

      - uses: ./.github/actions/specd-pr
        with:
          spec: my-feature
```

## Permissions (least privilege)

| Scope | Why |
|-------|-----|
| `contents: read` | Check out the repo and read spec artifacts. |
| `pull-requests: write` | Upsert the summary comment. |

Nothing else is required. The Action uses the automatically-provided
`${{ github.token }}` by default; pass `token:` only to override it.

## Supply chain

The composite Action depends on **no third-party actions** — only `bash`,
`curl`, and `jq`, which are preinstalled on GitHub-hosted runners. There is
therefore no transitive action to pin or audit inside it. When you reference
external actions in your *workflow* (`actions/checkout`, `actions/setup-go`),
pin them to a commit SHA rather than a moving tag.

## Inputs

| Input | Default | Meaning |
|-------|---------|---------|
| `spec` | — (required) | Spec slug to check. |
| `specd` | `specd` | Path to the specd binary. |
| `token` | `${{ github.token }}` | Token for the comment upsert. |
| `comment` | `true` | Set `false` to run gates without commenting. |
