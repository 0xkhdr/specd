# Tasks — Documentation & Repository Hygiene (S7)

## Wave 1

- [ ] T1 — Re-verify README.md's Windows statement and AGENTS.md's heading structure
  - why: spec.md Requirement 2.2 explicitly requires verifying against the live file at implementation time, not the quoted text in this spec, in case wording has drifted; also need the exact current `##` heading list to build the TOC
  - role: investigator
  - files: README.md, AGENTS.md, TESTING.md
  - contract: read README.md's Windows-support section in full and quote its exact current wording; list every `##` heading (with line number) in AGENTS.md and TESTING.md in document order. Do NOT modify any file.
  - acceptance: exact current Windows-support quote, and ordered heading lists for both files, recorded as task evidence
  - verify: N/A
  - depends: —
  - requirements: 1, 2

## Wave 2

- [ ] T2 — Add table of contents to AGENTS.md and TESTING.md
  - why: close the confirmed no-TOC gap (F11), per Requirement 1.1, using T1's heading list
  - role: builder
  - files: AGENTS.md, TESTING.md
  - contract: insert a "## Table of Contents" section immediately after each file's title/intro paragraph, with a markdown link to every `##` heading from T1's list, in document order. Do not alter any existing heading text or section content.
  - acceptance: every `##` heading in both files has a corresponding TOC entry; TOC links resolve correctly (anchor matches heading slug)
  - verify: cd /var/www/html/rai/up/specd && grep -c '^## ' AGENTS.md TESTING.md
  - depends: T1
  - requirements: 1

- [ ] T3 — Correct the Windows-support documentation scope
  - why: the analysis plan's blanket "POSIX-only on Windows" framing is broader than what README.md:68 actually states (discrepancy D14); ensure any place this gets summarized (this review's own docs, or README.md itself if T1 found it imprecise) states the Brain/Pinky-specific scope precisely
  - role: builder
  - files: README.md (only if T1 found the current wording is itself imprecise — otherwise this task is a no-op confirmation, not a forced edit)
  - contract: using T1's exact quote, confirm README.md:68 already correctly scopes the limitation to Brain/Pinky orchestration. If it does (expected, per discrepancy D14's evidence), make no edit and record that confirmation. If T1 found drift (wording changed since the live-evidence pass), correct it to precisely state Brain/Pinky-only POSIX dependency.
  - acceptance: README.md's Windows statement is confirmed accurate and scoped to Brain/Pinky orchestration specifically, not a blanket claim
  - verify: cd /var/www/html/rai/up/specd && grep -n "POSIX" README.md
  - depends: T1
  - requirements: 2

- [ ] T4 — Surface the AGENTS.md unrelated-content decision point (no deletion)
  - why: Requirement 4 — this block's provenance is unclear; deleting it without confirmation could destroy intentional user content unrelated to this review's scope
  - role: builder
  - files: AGENTS.md
  - contract: do NOT delete or modify lines 252-293 of AGENTS.md. Instead, record in this task's evidence the exact line range, a one-paragraph description of the content found there, and an explicit recommendation that the repository owner confirm whether to remove it, keep it, or relocate it to a tool-specific file outside AGENTS.md. This task's deliverable is the flagged decision point, not a code change.
  - acceptance: a written decision-point note exists in task evidence; AGENTS.md:252-293 is byte-for-byte unchanged
  - verify: cd /var/www/html/rai/up/specd && git diff --stat AGENTS.md
  - depends: T1
  - requirements: 4

- [ ] T5 — Extend scripts/docs-lint.sh with a TOC-consistency check
  - why: keep the new TOC (T2) from silently drifting as headings change, per Requirement 1.2 and 3.1, without disrupting the script's existing two checks
  - role: builder
  - files: scripts/docs-lint.sh
  - contract: add a new bash function checking that every `##` heading in AGENTS.md/TESTING.md has a matching TOC entry (added in T2) and that every TOC entry resolves to an existing heading. Call this function alongside the script's existing two checks (dead command references; cheat-sheet table match) without modifying their logic. Add a comment flagging the existing hardcoded 20-command list as a known maintainability limitation (Requirement 3.3) — do not attempt to fix that limitation in this task.
  - acceptance: `bash scripts/docs-lint.sh` passes with all three checks (two existing + new TOC check) active; manually removing a TOC entry locally (sanity check, not committed) causes the new check to fail
  - verify: cd /var/www/html/rai/up/specd && bash scripts/docs-lint.sh
  - depends: T2
  - requirements: 1, 3

## Wave 3

- [ ] T6 — Full documentation verification
  - why: gate G4's documentation phase must complete cleanly per the rollout plan in progress.md
  - role: verifier
  - files: N/A
  - contract: run docs-lint and confirm no regression across the full documentation suite
  - acceptance: `scripts/docs-lint.sh` passes; `make ci` passes with zero regressions attributable to S7
  - verify: cd /var/www/html/rai/up/specd && bash scripts/docs-lint.sh && make ci
  - depends: T3, T4, T5
  - requirements: 1, 2, 3, 4
