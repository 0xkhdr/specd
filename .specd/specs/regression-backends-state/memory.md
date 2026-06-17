# Memory — Regression: State Backends (default/git/postgres/redis, locking, CAS)

<!--
Source-attributed, generalizable learnings (append-only). Use
`specd memory <spec> add --key <slug> --pattern "<one-line>" --body "<detail>"
  --source "<Turn N, Task T?, role>" --criticality <minor|important|critical> [--related k,k]`.
Only generalizable patterns, never raw observations. Promote to project steering at 3+ specs via
`specd memory <spec> promote --key <slug>`. Format:

## <key-slug>
**Pattern:** <one-line generalizable claim>
**Detail:** <why it's true; the mechanism>
**Source:** Task T3, Turn 2, discovered by investigator
**Criticality:** important
**Related:** [[other-key]]
-->

## backend-inventory-T1
**Pattern:** conformance suite coverage gaps across backends
**Detail:** StateBackend methods: Name/Load/Save(CAS)/WithLock. Impls: fileBackend(default), gitBackend(always, git CLI), postgresBackend(tag specd_postgres, init-registered), redisBackend(tag specd_redis). Conformance suite backend_conformance_test.go runs ONLY file+git; subtests: stale-base CAS, reentrant lock, 32-goroutine no-lost-updates. GAPS: postgres/redis NOT in table (need env-gated skip) R1; no explicit revision-monotone assert R2.3; no durability/atomicity interrupted-write test R4.2; no git one-commit-per-write assert R4.3; no lock-release-on-completion explicit R3.3; no dead-holder recovery R3.2.
**Source:** T1
**Criticality:** important
**Related:** —

## parity-caveat-T5
**Pattern:** backend parity honesty / CI coverage limits
**Detail:** T5 review: T2-T4 skips are visible t.Skip with reasons (not silent passes) — confirmed via -v output (postgres/redis SKIP "not compiled in"). TESTING.md now documents tag+env setup + skip table. HONEST CAVEAT: default-build CI proves file+git parity only; postgres/redis parity proven only when a fork builds with the tag against a live service. No code path lets an unexercised backend report green.
**Source:** T5
**Criticality:** important
**Related:** —
