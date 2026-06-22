# Spec 08 — World-Wide Harness Features

> Wave: **W4 (P3)** · Priority: **P3 — roadmap, post-hardening** · Source:
> LEVEL_UP_PLAN §2 P3 (items 10–13)
> **Do not start until W1+W2 exit gates are green** — the autonomous core must be
> observable and tested before feature growth.

## 1. Context

These are the field features a world-wide harness needs once the core is
trustworthy. They are **additive**, opt-in where possible, and must preserve the
stdlib-only and deterministic-output invariants. Pieces of several already
exist; this spec **promotes, tests, and documents** them.

---

## 2. Feature 10 — Session resumability UX

### Problem
Resume *mechanism* is verified by Spec 02, but there's no first-class,
documented command. Operators need a clean `specd brain resume --session <id>`.

### Solution
- Promote the existing resume pieces (`ResumeProgramOrchestration`, session
  load, lease reconcile) into a first-class `brain resume --session <id>`
  subcommand with `--json` support and the standard exit-code contract.
- Reuse Spec 02's recovery invariants as the test backbone (reclaim, no
  double-dispatch).

### Acceptance
- [ ] `specd brain resume --session <id>` exists, documented, tested (happy path
      + unknown-session error + already-complete session).

---

## 3. Feature 11 — More host adapters

### Problem
Managed adapter set: claude-code / codex / cursor / gemini / vscode, plus
config-snippet fallback for antigravity / claude-desktop. New agents ship over
time.

### Solution
- Add adapters as new agents appear, following the existing adapter pattern in
  `internal/integration`. Keep the `--config` snippet fallback as the
  **universal path** so any host works without a bespoke adapter.

### Acceptance
- [ ] Adapter-addition pattern documented (how to add one); `--config` fallback
      verified as the universal escape hatch. (Specific new adapters are added
      as-needed, not gated on this spec.)

---

## 4. Feature 12 — Metrics export (opt-in)

### Problem
Fleet operators running many specs need machine-readable session metrics. The
event stream already exists (NDJSON via `watch`; structured events via Spec 03).

### Solution
- Add an **output format** (no runtime dep) that reshapes the session event
  stream into a **Prometheus textfile** and/or **OTLP-JSON** shape. Opt-in via a
  flag/env (e.g. `specd report --format prometheus` or a metrics endpoint on
  `serve`). It is a *format*, not a new transport or dependency.
- Source data = Spec 03's structured events + existing telemetry roll-up.

### Acceptance
- [ ] Opt-in metrics output in Prometheus-textfile and/or OTLP-JSON shape.
- [ ] No new runtime dependency (stdlib-only intact); output is deterministic
      and tested against a golden fixture.

---

## 5. Feature 13 — Cost-brake enforcement

### Problem
`--cost-limit` / `--orchestration-cost-limit` exist but the brake depends on
host-reported numbers with **no harnessed enforcement test**.

### Solution
- Implement **soft/hard** modes: **warn at 80%**, **halt at 100%** of the limit.
- Drive enforcement from host-reported cost (existing telemetry inputs); on
  hard-halt, stop dispatching new workers and exit with a clear code/message;
  emit a Spec-03 event (`event=cost_halt` / `cost_warn`).
- Make it **deterministically testable**: feed synthetic host cost reports and
  assert warn-at-80 / halt-at-100 transitions and that no new dispatch occurs
  after halt.

### Acceptance
- [ ] Soft warn at 80%, hard halt at 100%, both tested with synthetic cost
      reports (deterministic).
- [ ] On halt: no new worker dispatched; clear message + exit code; event
      emitted.

---

## 6. Sequencing within W4

```
10 resume UX ─► 13 cost brake ─► 12 metrics ─► 11 adapters (ongoing)
(reuses Spec 02) (reuses Spec 03 events) (reuses Spec 03 stream)
```
Resume + cost-brake are concrete, gateable deliverables. Metrics is a format
layer over Spec 03. Adapters are an ongoing pattern, not a one-time gate.

## 7. Non-goals

- New transports/exporters requiring runtime deps (push-gateways, OTLP gRPC).
  Emit *formats* only.
- Reworking the cost *accounting* model (telemetry stays operator-annotated);
  this adds *enforcement* on top.

## 8. Risks & mitigations

| Risk | Mitigation |
|---|---|
| Cost brake flaky on real host numbers | Test with synthetic injected reports, not live cost |
| Metrics format drift | Golden-fixture test for the output shape |
| Resume UX double-dispatch regression | Reuse Spec 02 invariants as the test backbone |
| Scope creep on adapters | Treat as ongoing pattern; gate only the documented `--config` fallback |
