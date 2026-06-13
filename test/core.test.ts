import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, writeFileSync, mkdirSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { atomicWrite, readOrDefault } from "../src/core/io.js";
import { findSpecdRoot } from "../src/core/paths.js";
import { initialState, loadState, saveState } from "../src/core/state.js";

const tmp = () => mkdtempSync(join(tmpdir(), "specd-core-"));

test("atomicWrite round-trips bytes; readOrDefault falls back", () => {
  const dir = tmp();
  const f = join(dir, "sub", "x.txt");
  atomicWrite(f, "héllo\nworld");
  assert.equal(readFileSync(f, "utf8"), "héllo\nworld");
  assert.equal(readOrDefault(join(dir, "missing"), "DEF"), "DEF");
});

test("findSpecdRoot walks up; null at fs root", () => {
  const dir = tmp();
  mkdirSync(join(dir, ".specd"));
  const nested = join(dir, "a", "b");
  mkdirSync(nested, { recursive: true });
  assert.equal(findSpecdRoot(nested), dir);
  assert.equal(findSpecdRoot(tmpdir()), null);
});

test("state load missing -> null; save then reload equals; updatedAt advances", async () => {
  const dir = tmp();
  mkdirSync(join(dir, ".specd", "specs", "s"), { recursive: true });
  assert.equal(loadState(dir, "s"), null);
  const st = initialState("s", "S");
  saveState(dir, "s", st);
  const first = loadState(dir, "s")!;
  assert.equal(first.spec, "s");
  await new Promise((r) => setTimeout(r, 5));
  saveState(dir, "s", first);
  const second = loadState(dir, "s")!;
  assert.ok(new Date(second.updatedAt).getTime() >= new Date(first.updatedAt).getTime());
});
