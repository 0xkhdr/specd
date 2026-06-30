# Tasks — Testing & Reliability (S4)

## Wave 1

- [ ] T1 — Profile existing stress scripts' resource usage
  - why: size timeout/ulimit bounds from real observed behavior, not a guess, per spec.md's risk note
  - role: investigator
  - files: scripts/stress.sh, scripts/stress-acp.sh, scripts/stress-orchestration.sh, scripts/stress-program.sh, scripts/stress-brain-recovery.sh, scripts/stress-checkpoint-fault.sh
  - contract: run each stress script locally once, recording wall-clock duration, peak goroutine count (if instrumentable without code changes, otherwise estimate from scenario design), and peak open file descriptors. Read each script in full to understand its concurrency model. Do NOT modify any script.
  - acceptance: a per-script table of observed duration, concurrency, and resource usage, used to size T2/T3's bounds
  - verify: N/A
  - depends: —
  - requirements: 1, 2

- [ ] T2 — Identify the coverage-floor-raise candidate package
  - why: Requirement 3 requires picking the package with the largest gap between current coverage and stated target, from real data
  - role: investigator
  - files: scripts/coverage-check.sh, coverage-*.out (existing profiles in repo root)
  - contract: run `go tool cover -func` against each of the 11 floor-enforced packages' coverage profiles (or regenerate via `scripts/coverage-check.sh` if profiles are stale); compare current % against each package's floor and stated target ceiling (per TESTING.md). Report the package with the largest current-to-target gap and the specific uncovered functions/branches contributing to it. Do NOT write tests yet.
  - acceptance: a ranked list of all 11 packages by current-to-target coverage gap, with the top candidate's uncovered lines identified
  - verify: N/A
  - depends: —
  - requirements: 3

## Wave 2

- [ ] T3 — Add timeout and ulimit bounds to stress targets
  - why: close the confirmed gap (Makefile stress targets have zero resource bounds today), per Requirement 1, sized from T1's data
  - role: builder
  - files: Makefile, scripts/stress.sh, scripts/stress-acp.sh, scripts/stress-orchestration.sh, scripts/stress-program.sh, scripts/stress-brain-recovery.sh, scripts/stress-checkpoint-fault.sh
  - contract: wrap each `stress*` Makefile target's script invocation with `timeout <N>s` (or set the timeout inside each script via `timeout` around the core loop), where N is sized from T1's observed duration plus a documented margin (comment the rationale inline). Add a `ulimit -u <N>`/`ulimit -n <N>` (process count / open files) at the top of each script, sized from T1's observed concurrency plus margin. Do NOT change any script's fault-injection logic, assertions, or exit-code semantics for the success path — only wrap execution.
  - acceptance: each stress target still passes under normal conditions; manually shortening the timeout (local sanity check, not committed) causes a clear "stress target exceeded budget" failure message, not a silent hang
  - verify: cd /var/www/html/rai/up/specd && make stress && make stress-acp && make stress-orchestration && make stress-program
  - depends: T1
  - requirements: 1

- [ ] T4 — Add goroutine/handle leak detection to stress scenarios
  - why: catch a slow leak before it becomes a production hang, per Requirement 2
  - role: builder
  - files: scripts/stress.sh, scripts/stress-acp.sh, scripts/stress-orchestration.sh, scripts/stress-program.sh, scripts/stress-brain-recovery.sh, scripts/stress-checkpoint-fault.sh
  - contract: for each stress script, capture goroutine count (or process count, since these are shell-orchestrated cross-process scenarios per TESTING.md's "cross-process concurrency stress" description — confirm the right primitive from T1's findings) before the scenario starts and after it completes (with a brief settling period), failing with a specific message if growth exceeds a documented tolerance. Where the scenario opens files/sockets, add an open-handle check using whatever mechanism fits the existing harness (do not introduce a new dependency — stdlib/POSIX tools only, consistent with `go.mod`'s stdlib-only constraint).
  - acceptance: stress scripts pass under normal conditions; introducing a deliberate leak locally (manual sanity check, not committed) causes the new check to fail with a specific leak-detected message
  - verify: cd /var/www/html/rai/up/specd && make stress && make stress-orchestration
  - depends: T1
  - requirements: 2

- [ ] T5 — Add tests to clear the new coverage floor for the T2-identified package
  - why: raise one package's coverage floor with real test coverage backing it, per Requirement 3, not by lowering the bar
  - role: builder
  - files: (package identified by T2, plus its corresponding `*_test.go` files)
  - contract: using T2's uncovered-lines report, add tests covering the identified gaps (prioritize error paths and edge cases per the original review's R9 intent). Do NOT touch `scripts/coverage-check.sh` yet — that's T6, gated on this task's coverage actually clearing the new target.
  - acceptance: `go tool cover -func` shows the target package at or above the new floor T2 identified as the stated target ceiling
  - verify: cd /var/www/html/rai/up/specd && go test ./... -coverprofile=coverage.out -count=1 && go tool cover -func=coverage.out | tail -1
  - depends: T2
  - requirements: 3

## Wave 3

- [ ] T6 — Raise the coverage floor constant
  - why: lock in T5's improvement so it can't silently regress, per Requirement 3.2-3.3
  - role: builder
  - files: scripts/coverage-check.sh
  - contract: update the floor constant for the package T5 improved, to a value at or just below T5's achieved coverage (leave headroom, don't set it to the exact current number which would make any future single-line regression fail). Do NOT change any other package's floor constant — this is a targeted, single-package raise.
  - acceptance: `scripts/coverage-check.sh` passes with the new floor; reverting T5's test additions locally (manual sanity check, not committed) causes the new floor to fail the check
  - verify: cd /var/www/html/rai/up/specd && ./scripts/coverage-check.sh
  - depends: T5
  - requirements: 3

## Wave 4

- [ ] T7 — Full verification run
  - why: gate G3 (action-prompt) requires all tests passing with -race before observability work (S5) begins
  - role: verifier
  - files: N/A
  - contract: run the complete project test, stress, and coverage suite
  - acceptance: `make test`, all six `make stress*` targets, and `scripts/coverage-check.sh` all pass with zero regressions attributable to S4
  - verify: cd /var/www/html/rai/up/specd && make test && make stress && make stress-acp && make stress-orchestration && make stress-program && make stress-brain-recovery && make stress-checkpoint-fault && ./scripts/coverage-check.sh
  - depends: T3, T4, T6
  - requirements: 1, 2, 3
