import { test } from "node:test";
import assert from "node:assert/strict";
import {
  parseTasks, serializeTasks, applyTaskAnnotation, parseDepends, findTask,
} from "../src/core/tasksParser.js";

const FIXTURE = `# Tasks — Demo

## Wave 1
- [ ] T1 — First task
  - why: because
  - role: investigator
  - files: a.ts
  - contract: do x
  - acceptance: x done
  - verify: N/A
  - depends: —
  - requirements: 1

## Wave 2
- [x] T2 — Second task ✓ complete · evidence: npm test PASS · 2026-06-13T00:00:00.000Z
  - why: build it
  - role: builder
  - files: b.ts
  - contract: do y
  - acceptance: y works
  - verify: npm test
  - depends: T1
  - requirements: 1
`;

test("round-trip of canonical fixture is byte-stable", () => {
  const doc = parseTasks(FIXTURE);
  assert.equal(serializeTasks(doc), FIXTURE);
});

test("[x] maps to complete checkbox + annotation parsed", () => {
  const doc = parseTasks(FIXTURE);
  const t2 = findTask(doc, "T2")!;
  assert.equal(t2.checked, true);
  assert.equal(t2.title, "Second task");
  assert.equal(t2.annotation?.kind, "complete");
  if (t2.annotation?.kind === "complete") assert.equal(t2.annotation.evidence, "npm test PASS");
});

test("missing mandatory key -> located error", () => {
  const bad = `# Tasks — X\n\n## Wave 1\n- [ ] T1 — t\n  - why: a\n  - role: builder\n`;
  assert.throws(() => parseTasks(bad), /missing key/);
});

test("parseDepends handles list, dash, none", () => {
  assert.deepEqual(parseDepends("T1, T2"), ["T1", "T2"]);
  assert.deepEqual(parseDepends("—"), []);
  assert.deepEqual(parseDepends("none"), []);
});

test("commented example task lines are ignored", () => {
  const withComment = `# Tasks — X\n\n<!--\n- [ ] T9 — phantom\n  - why: x\n-->\n\n## Wave 1\n- [ ] T1 — real\n  - why: a\n  - role: builder\n  - files: a\n  - contract: c\n  - acceptance: ac\n  - verify: npm test\n  - depends: —\n`;
  const doc = parseTasks(withComment);
  assert.equal(doc.tasks.length, 1);
  assert.equal(doc.tasks[0].id, "T1");
});

test("applyTaskAnnotation edits the real line, preserves the rest", () => {
  const out = applyTaskAnnotation(FIXTURE, "T1", true, { kind: "complete", evidence: "proof", ts: "2026-06-13T01:00:00.000Z" });
  assert.match(out, /- \[x\] T1 — First task ✓ complete · evidence: proof/);
  // unrelated content intact
  assert.match(out, /## Wave 2/);
});

test("applyTaskAnnotation skips commented task with same id", () => {
  const withComment = `# Tasks — X\n\n<!--\n- [ ] T1 — phantom\n-->\n\n## Wave 1\n- [ ] T1 — real\n  - why: a\n  - role: builder\n  - files: a\n  - contract: c\n  - acceptance: ac\n  - verify: npm test\n  - depends: —\n`;
  const out = applyTaskAnnotation(withComment, "T1", true, { kind: "complete", evidence: "x", ts: "t" });
  assert.match(out, /- \[ \] T1 — phantom/); // comment untouched
  assert.match(out, /- \[x\] T1 — real/);    // real line flipped
});
