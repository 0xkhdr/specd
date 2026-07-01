# S4 Tasks: Documentation Alignment — Root Docs

Dependencies: S1, S2, S3 (this spec describes the post-removal state; land after or
alongside those specs, not before — otherwise these doc edits describe code that doesn't
exist yet).

Blocks: S6 (final grep-based validation gate expects these files clean).

---

## Wave 1 — `README.md`

- [x] **T1.1** Rewrite the "Uninstall" section (currently `README.md:56-59`) from a
  `curl | bash` invocation of the now-deleted `scripts/uninstall.sh` to manual-removal
  prose covering: removing the install directory (`~/.local/share/specd` or
  `~/.specd-repo`), the `~/.local/bin/specd` symlink, and the `# specd` PATH line from
  shell rc files. Match the tone/format of the surrounding Install/Update sections.
  - Dependencies: S3 (script must actually be gone for this to be accurate).
  - Completion evidence: `grep -n "uninstall.sh" README.md` returns nothing. Confirmed.
  - **Deviation from spec:** live inspection of `scripts/install.sh` shows it installs a
    single plain binary to `${BIN_DIR}/specd` (default `~/.local/bin`) — there is no
    `~/.local/share/specd` / `~/.specd-repo` install directory and no symlink (source is
    fetched into a `mktemp` dir and cleaned up). Rewrote the section to describe the
    actual mechanism (remove the binary, remove the `# specd`-tagged `PATH` line) rather
    than the spec's speculative paths.

- [x] **T1.2** Reword the `docs/troubleshooting.md` link text (currently `README.md:143`,
  *"...for `doctor` remediation"*) to not reference the removed `doctor` command — e.g.
  *"...for scaffold and MCP remediation"* or similar, matched to what
  `docs/troubleshooting.md` actually covers post-S5-audit.
  - Dependencies: S1 (doctor must be gone); loosely coordinate with S5's audit of
    `docs/troubleshooting.md` itself so the link text matches the target content.
  - Completion evidence: `grep -n '`doctor`' README.md` returns nothing (or only
    historical/explanatory mentions, reviewed manually). Confirmed — zero hits. Used
    exactly the spec's suggested wording, cross-checked against
    `docs/troubleshooting.md`'s actual section headers (gate blocks, concurrency, sandbox,
    onboarding/MCP, Brain/Pinky orchestration) — "scaffold and MCP remediation" fits.

- [x] **T1.3** Confirm no other edits needed: `grep -n "specd update\b" README.md` (expect
  no bare-command hits — the existing Update section already uses `install.sh --force`);
  `grep -n "0.1.0" README.md` still shows the install examples pinned correctly (no
  regression).
  - Completion evidence: manual confirmation, no edit required if greps come back clean.
  Confirmed clean: zero `specd update\b` hits; the one `0.1.0` hit is the existing
  `--version 0.1.0` install-pin example, unchanged.

**Wave 1 validation:** manual read-through of README.md's Install/Update/Uninstall
section for internal consistency; `grep -rn "v0\.2\.0\|v0\.3\.0\|v1\.0\.0" README.md`
returns nothing.

---

## Wave 2 — `AGENTS.md`

- [x] **T2.1** Delete lines 252-293 (the entire RTK / "Rust Token Killer" section,
  delimited by `<!-- headroom:rtk-instructions -->` and `<!-- /headroom:rtk-instructions -->`
  HTML comments, including both delimiter comments themselves).
  - Dependencies: none.
  - Completion evidence: `grep -n "RTK\|Rust Token Killer\|headroom:rtk" AGENTS.md` returns
    nothing. Confirmed. Also removed the trailing blank line so the file now ends cleanly
    on the pre-existing `<!-- SPECD INIT: END v1 -->` marker.

- [x] **T2.2** In the file-tree listing (currently `AGENTS.md:68`: `report.go waves.go
  program.go update.go`), remove `update.go`. Keep `program.go`, `report.go`, `waves.go` —
  all three survive.
  - Dependencies: S1 (update.go must actually be deleted).
  - Completion evidence: `grep -n "update.go" AGENTS.md` returns nothing. Confirmed.

- [x] **T2.3** In the file-tree listing (currently `AGENTS.md:90`: `scripts/ ... install.sh
  uninstall.sh coverage-check.sh stress.sh ...`), remove `uninstall.sh`.
  - Dependencies: S3.
  - Completion evidence: `grep -n "uninstall.sh" AGENTS.md` returns nothing. Confirmed.

**Wave 2 validation:** `grep -rn "v0\.2\.0\|v0\.3\.0\|v1\.0\.0" AGENTS.md` returns nothing;
file still renders as valid markdown (no dangling HTML comment orphaned by T2.1).

---

## Wave 3 — `SECURITY.md`

- [x] **T3.1** Reword line 16 from *"specd is pre-1.0; only the latest tagged release
  receives security fixes."* to explicit v0.1.0 language, e.g. *"specd is currently at
  v0.1.0; only the latest tagged release receives security fixes."*
  - Dependencies: none.
  - Completion evidence: `grep -n "pre-1.0" SECURITY.md` returns nothing. Confirmed.

- [x] **T3.2** Rewrite lines 42-45 (the `specd doctor` sandbox-advisory paragraph) to state
  plainly that this advisory pre-check no longer exists post-`doctor`-removal, and that
  `verify --sandbox`'s fail-closed behavior is now the only signal. Do not merely
  find-and-replace "doctor" — the underlying capability is gone, not renamed.
  - Dependencies: S1.
  - Completion evidence: manual review confirms the paragraph no longer claims an advisory
    finding exists; `grep -n "doctor" SECURITY.md` returns one hit — the reworded
    sentence itself, explaining in past tense that the `specd doctor` command "used to
    report it" and has been removed. Historical/explanatory, per the spec's allowed
    exception.

- [x] **T3.3** Rewrite lines 50-52 (*"Self-update integrity. `install.sh` and `specd
  update` fetch..."*) to remove the `specd update` clause, keeping only `install.sh`'s
  checksum-verification behavior. Suggested heading: "Install integrity."
  - Dependencies: S1.
  - Completion evidence: `grep -n "specd update" SECURITY.md` returns nothing. Confirmed.
  Used the suggested "Install integrity." heading.

**Wave 3 validation:** `grep -rn "v0\.2\.0|v0\.3\.0|v1\.0\.0" SECURITY.md` returns nothing;
manual read-through confirms the Threat Model section is internally consistent (no
dangling reference to a removed mitigation).

---

## Wave 4 — `TESTING.md`

- [x] **T4.1** Confirm (do NOT edit) that line 217's `migrate` reference is the unrelated
  `internal/core/state.go:251` state-schema-migration function, not the CLI command. Leave
  untouched.
  - Dependencies: none.
  - Completion evidence: none required (verification-only task) — reviewed, confirmed
    unrelated to the CLI command, left untouched.

- [x] **T4.2** Rewrite line 225's `COVERAGE_GAPS.md` reference. Recommended: remove the
  claim that a separate file tracks the dark-path inventory (since it doesn't exist) and
  either (a) describe the policy without naming a nonexistent artifact, or (b) if the team
  wants the file to exist, flag that as a separate follow-up rather than fabricating
  content here.
  - Dependencies: none.
  - Completion evidence: `grep -n "COVERAGE_GAPS.md" TESTING.md` returns nothing (path a)
    or `test -f COVERAGE_GAPS.md` passes (path b, if chosen). Took path (a): confirmed
    empty grep.

- [x] **T4.3** Rewrite the "### Windows limitation (known, documented)" subsection
  (currently ~244-251) to drop the `specd update`/`update.go` self-replacement framing
  entirely (that capability no longer exists) and instead explain Windows's build-only CI
  status via the POSIX-shell dependency of orchestration/`verify` (cross-reference
  `README.md`'s existing WSL note).
  - Dependencies: S1.
  - Completion evidence: `grep -n "update.go\|specd update" TESTING.md` returns nothing.
  Confirmed. New text cross-references `README.md`'s existing `orchestration requires a
  POSIX shell (sh)...` error text verbatim (checked against
  `internal/worker/runner_windows.go:14`).

- [x] **T4.4** Rewrite the `SHA256SUMS` three-consumer list (currently ~259-264) to a
  two-consumer list (`.goreleaser.yml`, `scripts/install.sh`), removing the
  `internal/cmd/update.go` bullet and adjusting the framing sentence's consumer count.
  - Dependencies: S1.
  - Completion evidence: `grep -n "internal/cmd/update.go" TESTING.md` returns nothing.
  Confirmed. Framing sentence now says "two consumers".

**Wave 4 validation:** `grep -rn "v0\.2\.0\|v0\.3\.0\|v1\.0\.0" TESTING.md` returns nothing;
`grep -n "update.go\|specd update" TESTING.md` returns nothing.

---

## Wave 5 — Cross-file final gate

- [x] **T5.1** Run the combined grep across all four files:
  `grep -rn 'v0\.2\.0\|v0\.3\.0\|v1\.0\.0' README.md AGENTS.md SECURITY.md TESTING.md`
  — expect empty output.
  - Completion evidence: empty output. Confirmed.

- [x] **T5.2** Run:
  `grep -rn 'uninstall.sh\|update.go\|specd update\b\|specd doctor\b\|specd migrate\b' README.md AGENTS.md SECURITY.md TESTING.md`
  — expect empty output or only reviewed, intentional historical mentions.
  - Completion evidence: output reviewed and justified for every remaining hit, or empty.
  One hit: `SECURITY.md:43` — the reworded doctor-removal sentence itself (see T3.2).
  Reviewed and justified; no further edit needed.

**Wave 5 validation (gate for S6):** T5.1 and T5.2 both clean.
