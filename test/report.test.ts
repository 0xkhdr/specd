import { test } from "node:test";
import assert from "node:assert/strict";
import { renderHtml, renderMarkdown, badge, type ReportData } from "../src/core/report.js";
import { initialState } from "../src/core/state.js";

function data(withBlocker: boolean): ReportData {
  const state = initialState("demo", "Demo Feature");
  state.status = "executing";
  state.tasks = {
    T1: { id: "T1", title: "first", role: "builder", wave: 1, depends: [], requirements: [1], status: "complete", evidence: "x", startedAt: null, finishedAt: null, blocker: null },
    T2: { id: "T2", title: "second", role: "builder", wave: 2, depends: ["T1"], requirements: [1], status: "pending", startedAt: null, finishedAt: null, evidence: null, blocker: null },
  };
  if (withBlocker) state.blockers = [{ task: "T2", reason: "need decision", since: "Turn 1" }];
  return {
    state,
    requirements: "# Requirements — Demo\n\n## Introduction\nThe intro.\n",
    design: "# Design — Demo\n\n## Overview\nThe overview.\n",
    tasks: "# Tasks — Demo\n",
    decisions: null, memory: null, midReqs: null,
  };
}

test("markdown report has §11 sections in order", () => {
  const md = renderMarkdown(data(false));
  const order = ["Executive Summary", "Progress Overview", "Requirements", "Plan / Design", "Tasks", "Mid-Requirements", "Build Knowledge", "Decisions"];
  let last = -1;
  for (const s of order) {
    const idx = md.indexOf(s);
    assert.ok(idx > last, `section ${s} out of order`);
    last = idx;
  }
  assert.match(md, /Implementing/); // executing -> Implementing badge label
});

test("blockers section present only when blockers exist", () => {
  assert.doesNotMatch(renderMarkdown(data(false)), /🚧 Blockers/);
  assert.match(renderMarkdown(data(true)), /🚧 Blockers/);
});

test("html is dependency-free single file", () => {
  const html = renderHtml(data(false));
  assert.match(html, /^<!doctype html>/);
  assert.doesNotMatch(html, /<script/);
  assert.doesNotMatch(html, /https?:\/\//); // no external resources
  assert.match(html, new RegExp(badge("executing").color));
});

test("acceptance criteria section present only when the G5 ledger has entries", () => {
  assert.doesNotMatch(renderMarkdown(data(false)), /🧪 Acceptance Criteria/);
  const d = data(false);
  d.state.acceptance = { "1.1": { requirement: 1, criterion: 1, status: "pass", evidence: "UAT ok", ranAt: "t" } };
  const md = renderMarkdown(d);
  assert.match(md, /🧪 Acceptance Criteria/);
  assert.match(md, /✅ \*\*1\.1\*\* _\(req 1\)_ — UAT ok/);
});

test("badge mapping", () => {
  assert.equal(badge("complete").label, "Complete");
  assert.equal(badge("blocked").color, "#f85149");
  assert.equal(badge("requirements").label, "Planning");
});
