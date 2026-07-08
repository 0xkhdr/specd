# specd — GitHub Action

specd ships a composite action that runs the validation gates on a pull request and upserts a
deterministic PR-summary comment. It lives at `.github/actions/specd-pr/action.yml`.

**Supply-chain note.** The action uses only `bash`, `curl`, and `jq` (preinstalled on
GitHub-hosted runners). It pulls in **no third-party actions** — nothing to pin, no transitive
action to audit. specd itself is stdlib-only and makes no network calls; the only network in
the action is the GitHub API call that upserts the comment.

## Inputs

| Input | Required | Default | Description |
|---|---|---|---|
| `spec` | yes | — | Spec slug to check (e.g. `payments`). |
| `specd` | no | `specd` | Path to the specd binary (on `PATH` or absolute). |
| `token` | no | `${{ github.token }}` | Token used to upsert the PR comment (needs `pull-requests: write`). |
| `comment` | no | `"true"` | Whether to upsert a PR comment with the summary. |

## What it does

Three steps, in order:

1. **Render + check.** Runs `specd report <spec> --pr` into `specd-pr-summary.md` (the
   deterministic, network-free summary — rendered regardless of gate outcome, since the summary
   itself reports gate status), then runs `specd check <spec>` and captures its exit code.
2. **Upsert PR comment** (only on `pull_request` events, when `comment == "true"`). Finds an
   existing comment carrying the per-spec marker `<!-- specd-pr-summary:<spec> -->` and PATCHes
   it, or POSTs a new one — an **idempotent upsert**, so re-runs update in place instead of
   piling up comments.
3. **Fail on gate violations.** If `specd check` exited non-zero, the step fails the job. The
   gate verdict — not the comment — drives CI status.

## Usage

You must build (or download) the `specd` binary and put it on `PATH` before invoking the
action. A minimal workflow:

```yaml
name: specd
on: pull_request
permissions:
  contents: read
  pull-requests: write   # required for the comment upsert
jobs:
  specd:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.26' }
      - run: go build -o /usr/local/bin/specd .
      - uses: ./.github/actions/specd-pr
        with:
          spec: payments
```

To run the gate without commenting (e.g. a required status check without PR chatter), set
`comment: "false"`. The job still fails on gate violations.

---

**See also:** [command-reference.md](command-reference.md#report) ·
[validation-gates.md](validation-gates.md) · [user-guide.md](user-guide.md)
