import { test } from "node:test";
import assert from "node:assert/strict";
import { writeFileSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { run, newTmp } from "./helpers.js";

const REQS = `# Requirements — A

## Introduction
x

## Requirement 1 — gate
**User story:** As an agent, I want gated tasks, so that quality holds.

**Acceptance criteria:**
1. WHEN a task completes THE SYSTEM SHALL require evidence
`;

const BAD_REQS = `# Requirements — A

## Requirement 1 — bad
**User story:** As an agent, I want X.

**Acceptance criteria:**
1. the system should maybe do a thing
`;

const DESIGN = `# Design — A

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

const TASKS = `# Tasks — A

## Wave 1
- [ ] T1 — build
  - why: implement requirement 1
  - role: builder
  - files: a.ts
  - contract: add check
  - acceptance: works
  - verify: npm test
  - depends: —
  - requirements: 1
`;

async function spec(slug = "a"): Promise<string> {
  const dir = newTmp();
  await run(dir, "init");
  await run(dir, "new", slug);
  return dir;
}
const statePath = (dir: string, slug = "a") => join(dir, ".specd", "specs", slug, "state.json");
const readState = (dir: string, slug = "a") => JSON.parse(readFileSync(statePath(dir, slug), "utf8"));

test("approve advances the planning ratchet requirements → design → tasks → executing", async () => {
  const dir = await spec();
  const s = join(dir, ".specd", "specs", "a");
  writeFileSync(join(s, "requirements.md"), REQS);
  writeFileSync(join(s, "design.md"), DESIGN);
  writeFileSync(join(s, "tasks.md"), TASKS);

  assert.equal(readState(dir).status, "requirements");
  assert.equal((await run(dir, "approve", "a")).rc, 0);
  assert.equal(readState(dir).status, "design");
  assert.equal((await run(dir, "approve", "a")).rc, 0);
  assert.equal(readState(dir).status, "tasks");
  const last = await run(dir, "approve", "a");
  assert.equal(last.rc, 0);
  assert.equal(readState(dir).status, "executing");
  assert.equal(readState(dir).phase, "execute");

  // past the planning gates → nothing to approve
  const past = await run(dir, "approve", "a");
  assert.equal(past.rc, 1);
  assert.match(past.err, /nothing to approve/);
});

test("approve refuses to advance a phase that fails its gate", async () => {
  const dir = await spec();
  writeFileSync(join(dir, ".specd", "specs", "a", "requirements.md"), BAD_REQS);
  const r = await run(dir, "approve", "a");
  assert.equal(r.rc, 1);
  assert.match(r.err, /cannot approve 'requirements'/);
  assert.equal(readState(dir).status, "requirements"); // unchanged
});

test("approve clears a midreq awaiting-approval gate", async () => {
  const dir = await spec();
  await run(dir, "midreq", "a", "scope change", "--impact", "high");
  assert.equal(readState(dir).gate, "awaiting-approval");
  const r = await run(dir, "approve", "a");
  assert.equal(r.rc, 0);
  assert.match(r.out, /gate cleared/);
  assert.equal(readState(dir).gate, "none");
});

test("next and task refuse to proceed while gated, and --force overrides", async () => {
  const dir = await spec();
  const s = join(dir, ".specd", "specs", "a");
  writeFileSync(join(s, "tasks.md"), TASKS);
  await run(dir, "midreq", "a", "rework", "--impact", "critical");

  const n = await run(dir, "next", "a");
  assert.equal(n.rc, 1);
  assert.match(n.err, /awaiting-approval/);

  const nForce = await run(dir, "next", "a", "--force");
  assert.equal(nForce.rc, 0);

  const t = await run(dir, "task", "a", "T1", "--status", "running");
  assert.equal(t.rc, 1);
  assert.match(t.err, /gated/);

  const tForce = await run(dir, "task", "a", "T1", "--status", "running", "--force");
  assert.equal(tForce.rc, 0);
});

test("next --json embeds the full task payload", async () => {
  const dir = await spec();
  writeFileSync(join(dir, ".specd", "specs", "a", "tasks.md"), TASKS);
  const r = await run(dir, "next", "a", "--json");
  const parsed = JSON.parse(r.out);
  assert.equal(parsed.id, "T1");
  assert.equal(parsed.task.role, "builder");
  assert.equal(parsed.task.verify, "npm test");
});

test("--version prints the package version", async () => {
  const dir = newTmp();
  const r = await run(dir, "--version");
  assert.equal(r.rc, 0);
  assert.match(r.out, /^specd \d+\.\d+\.\d+/);
});
