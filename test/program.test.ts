// G4 — `specd program`: the cross-spec / program view. Edges live in a central `.specd/program.json`
// manifest; the view projects each spec as a node in a spec-level DAG and reports which specs are
// runnable across the whole program right now. A spec is runnable when all the specs it depends on
// are complete; a cross-spec cycle is a hard error.
import { test } from "node:test";
import assert from "node:assert/strict";
import { writeFileSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { run, newTmp } from "./helpers.js";

const statePath = (dir: string, slug: string) => join(dir, ".specd", "specs", slug, "state.json");

/** Force a spec's status directly on disk — the full requirements→complete lifecycle is out of scope here. */
function setStatus(dir: string, slug: string, status: string): void {
  const p = statePath(dir, slug);
  const s = JSON.parse(readFileSync(p, "utf8"));
  s.status = status;
  writeFileSync(p, JSON.stringify(s, null, 2) + "\n");
}

async function setup(...slugs: string[]): Promise<string> {
  const dir = newTmp();
  await run(dir, "init");
  for (const s of slugs) await run(dir, "new", s);
  return dir;
}

test("program: no edges — every spec is wave 1 and runnable", async () => {
  const dir = await setup("alpha", "beta");
  const j = JSON.parse((await run(dir, "program", "--json")).out);
  assert.equal(j.kind, "program");
  assert.equal(j.count, 2);
  assert.deepEqual(j.frontier.sort(), ["alpha", "beta"]);
  assert.deepEqual(j.waves, [{ wave: 1, specs: ["alpha", "beta"] }]);
  assert.ok(j.specs.every((s: { wave: number; runnable: boolean }) => s.wave === 1 && s.runnable));
});

test("program: link gates the dependent until its dep completes", async () => {
  const dir = await setup("base", "feat");
  const linked = await run(dir, "program", "link", "feat", "--on", "base");
  assert.equal(linked.rc, 0);
  assert.match(linked.out, /feat now depends on base/);

  let j = JSON.parse((await run(dir, "program", "--json")).out);
  assert.deepEqual(j.frontier, ["base"]); // feat gated by incomplete base
  assert.deepEqual(j.waves, [{ wave: 1, specs: ["base"] }, { wave: 2, specs: ["feat"] }]);
  assert.deepEqual(j.criticalPath, ["base", "feat"]);
  const feat = j.specs.find((s: { slug: string }) => s.slug === "feat");
  assert.equal(feat.runnable, false);
  assert.deepEqual(feat.dependsOn, ["base"]);

  setStatus(dir, "base", "complete");
  j = JSON.parse((await run(dir, "program", "--json")).out);
  assert.deepEqual(j.frontier, ["feat"]); // dep cleared
  assert.equal(j.next.kind, "task");
  assert.equal(j.next.id, "feat");

  setStatus(dir, "feat", "complete");
  j = JSON.parse((await run(dir, "program", "--json")).out);
  assert.deepEqual(j.frontier, []);
  assert.equal(j.next.kind, "all-complete");
});

test("program link: refuses an edge that would create a cycle", async () => {
  const dir = await setup("a", "b");
  await run(dir, "program", "link", "b", "--on", "a");
  const r = await run(dir, "program", "link", "a", "--on", "b");
  assert.equal(r.rc, 1);
  assert.match(r.err, /would create a cycle/);
  // manifest unchanged: a has no deps
  const m = JSON.parse(readFileSync(join(dir, ".specd", "program.json"), "utf8"));
  assert.deepEqual(m.dependsOn, { b: ["a"] });
});

test("program link: self-dependency and unknown specs are rejected", async () => {
  const dir = await setup("solo");
  assert.equal((await run(dir, "program", "link", "solo", "--on", "solo")).rc, 2);
  assert.equal((await run(dir, "program", "link", "solo", "--on", "ghost")).rc, 3);
  assert.equal((await run(dir, "program", "link", "ghost", "--on", "solo")).rc, 3);
});

test("program unlink: removes the edge", async () => {
  const dir = await setup("x", "y");
  await run(dir, "program", "link", "y", "--on", "x");
  const r = await run(dir, "program", "unlink", "y", "--on", "x");
  assert.equal(r.rc, 0);
  assert.match(r.out, /no longer depends on x/);
  const j = JSON.parse((await run(dir, "program", "--json")).out);
  assert.deepEqual(j.frontier.sort(), ["x", "y"]);
});

test("program: an edge to a deleted spec is flagged as an orphan (warn, exit 0)", async () => {
  const dir = await setup("keep");
  // hand-write an edge to a spec that does not exist
  writeFileSync(join(dir, ".specd", "program.json"),
    JSON.stringify({ version: 1, dependsOn: { keep: ["gone"] } }, null, 2) + "\n");
  const r = await run(dir, "program", "--json");
  assert.equal(r.rc, 0);
  const j = JSON.parse(r.out);
  assert.deepEqual(j.orphans, [{ spec: "keep", dep: "gone" }]);
  assert.deepEqual(j.specs[0].dependsOn, []); // orphan edge filtered from the live graph
  assert.deepEqual(j.frontier, ["keep"]);
});

test("program text mode: renders waves, legend, and the runnable frontier", async () => {
  const dir = await setup("base", "feat");
  await run(dir, "program", "link", "feat", "--on", "base");
  const r = await run(dir, "program");
  assert.equal(r.rc, 0);
  assert.match(r.out, /# Program — 2 spec\(s\)/);
  assert.match(r.out, /legend:/);
  assert.match(r.out, /Wave 1:/);
  assert.match(r.out, /▶ base/);
  assert.match(r.out, /Wave 2:/);
  assert.match(r.out, /· feat .* ← base/);
  assert.match(r.out, /Runnable now: base/);
  assert.match(r.out, /Critical path: base → feat/);
});
