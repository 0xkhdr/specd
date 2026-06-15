# spec.md — GitHub-native Integration (PR ↔ spec binding)

**Status:** proposed
**Source:** specd-report.html §8 idea **E3** (impact: high · effort: med · moat: med) · §9 north-star item **#5**
**Date:** 2026-06-16
**Scope:** a GitHub Action + `specd` PR-summary output; optional GitHub App is a follow-up.

---

## 1. Objective

A GitHub Action that posts the wave DAG + gate status as a PR check and comment,
links each commit to its task ID, and blocks merge on gate failure. Meet
developers exactly where review already happens. Distribution beats features;
living inside the PR is how a workflow tool reaches millions without anyone
running `install.sh` first.

> **Hard invariant:** the `specd` binary stays deterministic, stdlib-only, and
> network-free — it only **emits** the data (gate status, wave DAG, commit↔task
> links) as structured output / a Markdown PR summary. All GitHub API
> interaction lives in a thin **Action wrapper** (composite/JS action) that
> calls `specd` and uses `GITHUB_TOKEN` + `gh`/REST. Gate pass/fail drives the
> check via specd's existing exit codes. specd itself never calls the network.

## 2. Context

- `specd check` returns exit 1 on gate failure (`exit.go`) — already the right
  merge-block signal.
- `specd report` / `status` / `waves` emit the DAG + gate status, with
  `SPECD_JSON=1` structured variants.
- Tasks carry IDs; commits can reference them (the report's commit↔task linkage
  goal).

## 3. Requirements (EARS)

- **R1 (H)** THE SYSTEM SHALL provide a GitHub Action that runs `specd check` on
  a PR and sets the check status from specd's exit code (non-zero ⇒ failing
  check ⇒ blockable merge).
- **R2 (H)** THE Action SHALL post (and update in place) a PR comment containing
  the wave DAG + per-gate status, rendered from `specd report`/`status` output.
- **R3 (M)** `specd` SHALL provide a PR-summary output mode (Markdown +
  `SPECD_JSON=1`) suitable for the Action to post, computed deterministically and
  without network access.
- **R4 (M)** WHERE commit messages reference a task ID, the summary SHALL link
  each referenced commit to its task; unreferenced commits SHALL be listed
  separately, never dropped.
- **R5 (M)** THE Action SHALL use `GITHUB_TOKEN`/least-privilege permissions and
  SHALL be pinned (no floating action refs), consistent with the repo's
  supply-chain stance.
- **R6 (L)** Documentation SHALL include a copy-paste workflow snippet and the
  required permissions.

## 4. Design / approach

1. **specd side** — add a `specd report --pr-summary` (or extend report) that
   emits a deterministic Markdown PR summary + JSON: wave DAG, gate status,
   commit↔task links. No network.
2. **Action wrapper** — `.github/actions/specd-pr/` composite action: install
   specd, run `check`, capture `--pr-summary`, set the check status from the exit
   code, upsert the PR comment via `gh`/REST with `GITHUB_TOKEN`.
3. **Commit linking** — parse commit messages for task IDs to build the link map
   (in specd, deterministically; the Action just renders).
4. **Supply chain** — pin action refs; least-privilege `permissions:`.

## 5. Non-goals

- No network calls inside the `specd` binary — the Action owns all API traffic.
- No full GitHub App (webhooks/installations) in this spec — Action first.
- No change to gate semantics; the check is driven by existing exit codes.

## 6. Acceptance criteria

- A PR running the Action gets a check whose pass/fail mirrors `specd check`'s
  exit code (gate failure blocks merge).
- The Action posts/updates one PR comment with the wave DAG + gate status from
  `specd`'s deterministic PR-summary output.
- Commit↔task links appear; unreferenced commits are listed, not dropped.
- `specd` makes no network call (asserted); action refs pinned, least-privilege
  token; workflow snippet documented; `make ci` green; binary stdlib-only.
