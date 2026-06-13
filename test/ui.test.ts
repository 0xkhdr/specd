import { test } from "node:test";
import assert from "node:assert/strict";
import { isJsonMode, json } from "../src/core/ui.js";
import { renderHelp, renderCommandHelp, renderHelpJson } from "../src/core/help.js";

test("isJsonMode checks env and global flags", () => {
  const origEnv = process.env.SPECd_JSON;
  try {
    delete process.env.SPECd_JSON;
    (globalThis as any).specdJsonMode = false;
    assert.equal(isJsonMode(), false);

    (globalThis as any).specdJsonMode = true;
    assert.equal(isJsonMode(), true);

    (globalThis as any).specdJsonMode = false;
    process.env.SPECd_JSON = "1";
    assert.equal(isJsonMode(), true);
  } finally {
    process.env.SPECd_JSON = origEnv;
    delete (globalThis as any).specdJsonMode;
  }
});

test("json helper structures LogEntry correctly", () => {
  const entry = json("info", "test message");
  assert.equal(entry.level, "info");
  assert.equal(entry.message, "test message");
  assert.ok(entry.timestamp);
  assert.doesNotThrow(() => new Date(entry.timestamp));
});

test("renderHelp includes CLI header and categories", () => {
  const help = renderHelp();
  assert.match(help, /specd — spec-driven coding harness/);
  assert.match(help, /LIFECYCLE/);
  assert.match(help, /EXECUTION/);
  assert.match(help, /INSPECTION/);
  assert.match(help, /META/);
  assert.match(help, /init/);
  assert.match(help, /update/);
});

test("renderCommandHelp formats single command instructions", () => {
  const initHelp = renderCommandHelp("init");
  assert.match(initHelp, /NAME\s+specd init/);
  assert.match(initHelp, /SYNOPSIS\s+specd init/);
  assert.match(initHelp, /DESCRIPTION/);
  assert.match(initHelp, /FLAGS/);
  assert.match(initHelp, /EXIT CODES/);
  assert.match(initHelp, /EXAMPLE/);
});

test("renderCommandHelp throws for unknown command", () => {
  assert.throws(() => renderCommandHelp("not-a-command"));
});

test("renderHelpJson outputs valid JSON schema of commands", () => {
  const jsonStr = renderHelpJson();
  const parsed = JSON.parse(jsonStr);
  assert.ok(Array.isArray(parsed));
  assert.ok(parsed.length > 0);
  const initCmd = parsed.find((c: any) => c.command === "init");
  assert.ok(initCmd);
  assert.equal(initCmd.category, "lifecycle");
  assert.ok(initCmd.usage);
});
