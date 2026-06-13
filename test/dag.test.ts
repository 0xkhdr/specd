import { test } from "node:test";
import assert from "node:assert/strict";
import {
  detectCycle, orphanDeps, waveViolations, nextRunnable, criticalPath, type DagTask,
} from "../src/core/dag.js";

const t = (id: string, wave: number, depends: string[], status: DagTask["status"] = "pending"): DagTask =>
  ({ id, wave, depends, status });

test("detectCycle flags a cycle and returns the path", () => {
  const cycle = detectCycle([t("T1", 1, ["T3"]), t("T2", 1, ["T1"]), t("T3", 1, ["T2"])]);
  assert.ok(cycle && cycle.length >= 3);
});

test("detectCycle null on acyclic", () => {
  assert.equal(detectCycle([t("T1", 1, []), t("T2", 2, ["T1"])]), null);
});

test("orphanDeps flags missing dependency", () => {
  assert.deepEqual(orphanDeps([t("T1", 1, ["TX"])]), [{ task: "T1", dep: "TX" }]);
});

test("waveViolations flags dep in later wave", () => {
  assert.deepEqual(waveViolations([t("T1", 1, ["T2"]), t("T2", 2, [])]), [{ task: "T1", dep: "T2" }]);
});

test("nextRunnable picks lowest wave then lowest id", () => {
  const r = nextRunnable([t("T3", 1, []), t("T1", 1, []), t("T2", 2, [])]);
  assert.deepEqual(r, { kind: "task", id: "T1" });
});

test("nextRunnable respects dependency completion", () => {
  const r = nextRunnable([t("T1", 1, [], "complete"), t("T2", 2, ["T1"])]);
  assert.deepEqual(r, { kind: "task", id: "T2" });
});

test("nextRunnable all-complete", () => {
  assert.deepEqual(nextRunnable([t("T1", 1, [], "complete")]), { kind: "all-complete" });
});

test("nextRunnable all-blocked", () => {
  assert.deepEqual(nextRunnable([t("T1", 1, [], "blocked")]), { kind: "all-blocked", blocked: ["T1"] });
});

test("nextRunnable waiting names blocking deps", () => {
  const r = nextRunnable([t("T1", 1, [], "blocked"), t("T2", 2, ["T1"])]);
  assert.deepEqual(r, { kind: "waiting", blocking: ["T1"] });
});

test("criticalPath returns longest chain", () => {
  assert.deepEqual(criticalPath([t("T1", 1, []), t("T2", 2, ["T1"]), t("T3", 3, ["T2"])]), ["T1", "T2", "T3"]);
});
