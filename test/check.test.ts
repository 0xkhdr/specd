import { test } from "node:test";
import assert from "node:assert/strict";
import { writeFileSync } from "node:fs";
import { join } from "node:path";
import { run, newTmp } from "./helpers.js";

const REQS = `# Requirements — Demo

## Introduction
A demo feature.

## Requirement 1 — only
**User story:** As a user, I want x, so that y.

**Acceptance criteria:**
1. WHEN a THE SYSTEM SHALL b
`;

const DESIGN = `# Design — Demo

## Overview
o
## Architecture
a
## Components and interfaces
c
## Data models
d
## Error handling
e
## Verification strategy
v
## Risks and open questions
r
`;

const TASKS = `# Tasks — Demo

## Wave 1
- [ ] T1 — do it
  - why: needed
  - role: builder
  - files: a.ts
  - contract: c
  - acceptance: ac
  - verify: npm test
  - depends: —
  - requirements: 1
`;

async function validSpec() {
  const dir = newTmp();
  await run(dir, "init");
  await run(dir, "new", "demo");
  const s = join(dir, ".specd", "specs", "demo");
  writeFileSync(join(s, "requirements.md"), REQS);
  writeFileSync(join(s, "design.md"), DESIGN);
  writeFileSync(join(s, "tasks.md"), TASKS);
  return { dir, s };
}

test("fully valid spec passes check (exit 0)", async () => {
  const { dir } = await validSpec();
  const r = await run(dir, "check", "demo");
  assert.equal(r.rc, 0, r.err);
});

test("EARS gate fails on bad criterion", async () => {
  const { dir, s } = await validSpec();
  writeFileSync(join(s, "requirements.md"), REQS.replace("WHEN a THE SYSTEM SHALL b", "just do it"));
  const r = await run(dir, "check", "demo", "--json");
  const out = JSON.parse(r.out);
  assert.equal(r.rc, 1);
  assert.ok(out.violations.some((v: any) => v.gate === "ears"));
});

test("design gate fails on TODO/missing section", async () => {
  const { dir, s } = await validSpec();
  writeFileSync(join(s, "design.md"), DESIGN.replace("## Risks and open questions\nr\n", ""));
  const r = await run(dir, "check", "demo", "--json");
  assert.equal(r.rc, 1);
  assert.ok(JSON.parse(r.out).violations.some((v: any) => v.gate === "design"));
});

test("task-schema gate fails on missing key", async () => {
  const { dir, s } = await validSpec();
  writeFileSync(join(s, "tasks.md"), TASKS.replace("  - verify: npm test\n", ""));
  const r = await run(dir, "check", "demo", "--json");
  assert.equal(r.rc, 1);
  assert.ok(JSON.parse(r.out).violations.some((v: any) => v.gate === "task-schema"));
});

test("DAG gate fails on cycle / orphan", async () => {
  const { dir, s } = await validSpec();
  const cyclic = TASKS + `
- [ ] T2 — two
  - why: w
  - role: builder
  - files: b.ts
  - contract: c
  - acceptance: ac
  - verify: npm test
  - depends: T3
  - requirements: 1
`;
  writeFileSync(join(s, "tasks.md"), cyclic);
  const r = await run(dir, "check", "demo", "--json");
  assert.equal(r.rc, 1);
  assert.ok(JSON.parse(r.out).violations.some((v: any) => v.gate === "dag"));
});

test("task-schema gate fails when builder verify is N/A", async () => {
  const { dir, s } = await validSpec();
  writeFileSync(join(s, "tasks.md"), TASKS.replace("  - verify: npm test\n", "  - verify: N/A\n"));
  const r = await run(dir, "check", "demo", "--json");
  assert.equal(r.rc, 1);
  assert.ok(JSON.parse(r.out).violations.some((v: any) => /verify N\/A/.test(v.message)));
});
