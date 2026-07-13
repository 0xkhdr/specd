# specd — GitHub Actions

## PR check action

specd ships a composite action that runs validation gates on a pull request and upserts a
deterministic PR-summary comment. It lives at `.github/actions/specd-pr/action.yml`.

**Supply-chain note.** The action uses only `bash`, `curl`, and `jq` (preinstalled on
GitHub-hosted runners). It pulls in **no third-party actions** — nothing to pin, no transitive
action to audit. specd itself is stdlib-only and makes no network calls; the only network in
the action is the GitHub API call that upserts the comment.

### Inputs

| Input | Required | Default | Description |
|---|---|---|---|
| `spec` | yes | — | Spec slug to check (e.g. `payments`). |
| `specd` | no | `specd` | Path to the specd binary (on `PATH` or absolute). |
| `token` | no | `${{ github.token }}` | Token used to upsert the PR comment (needs `pull-requests: write`). |
| `comment` | no | `"true"` | Whether to upsert a PR comment with the summary. |

### What it does

1. **Render + check.** Runs `specd report <spec> --pr` into `specd-pr-summary.md`, then runs
   `specd check <spec>` and captures its exit code.
2. **Upsert PR comment.** On pull requests when enabled, PATCHes the per-spec marker comment or
   POSTs it once. Re-runs update in place.
3. **Fail on gate violations.** Gate verdict, not comment delivery, drives CI status.

Build or download `specd` and put it on `PATH` before invoking action:

```yaml
name: specd
on: pull_request
permissions:
  contents: read
  pull-requests: write
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

Set `comment: "false"` to run gates without commenting. Job still fails on gate violations.

See also: [command-reference.md](command-reference.md#report),
[validation-gates.md](validation-gates.md), and [user-guide.md](user-guide.md).

## Delivery binding action

`.github/actions/specd-delivery` is an optional, local composite action that binds an immutable
source evidence-set digest and git HEAD to one built artifact digest, SBOM reference, provenance
reference, target environment, deployment ID, and positive attempt. It performs no checkout,
credential discovery, network request, deployment, or production approval.

Use it only from a trusted release workflow. Protect the `production` environment with GitHub
environment approval and pass credentials only to the later deployment step that needs them. The
action rejects every pull-request attempt to target production. It also rejects a fork pull request
when the caller reports that production credentials are present. GitHub does not pass environment
secrets to fork workflows; workflows must preserve that default and must not route secrets through
ordinary inputs.

```yaml
- uses: ./.github/actions/specd-delivery
  id: delivery
  with:
    artifact: dist/specd_linux_amd64.tar.gz
    artifact-digest: ${{ needs.candidate.outputs.artifact_digest }}
    source-evidence-digest: ${{ needs.candidate.outputs.evidence_digest }}
    git-head: ${{ github.sha }}
    sbom-ref: ${{ needs.candidate.outputs.sbom_ref }}
    provenance-ref: ${{ needs.candidate.outputs.provenance_ref }}
    environment: production
    deployment-id: ${{ github.run_id }}
    attempt: ${{ github.run_attempt }}
```

Artifact bytes are hashed again immediately before binding. A mismatch exits `2`; the action never
emits a binding for substituted bytes. The resulting JSON path is available as
`steps.delivery.outputs.binding` and can be embedded in the Domain 10 adapter result payload.

Externally asserted CI/runtime identity uses `core.CIIdentityEnvelopeV1`. Signers use Ed25519 and
verifiers call `core.VerifyCIIdentity` with an explicit local `key_id` → public-key allowlist plus
the expected repository, environment, audience, and verification time. Verification is offline and
fails closed for tampered claims, wrong identity/audience, expired or not-yet-valid assertions, and
unknown keys. Key distribution, rotation, workflow environment protection, and deployment
credentials remain operator responsibilities; no private key or credential belongs in `.specd/`.
