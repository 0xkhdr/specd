import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { main } from "../src/cli.js";
import { stdout } from "../src/core/output.js";

export interface RunResult { rc: number; out: string; err: string; }

// Node 22 runs a file's top-level tests concurrently and `runOnce` swaps process-global singletons
// (console.log/error, cwd) plus the CLI's stdout sink. Serialize every invocation through this
// chain so concurrent tests never clobber each other's captures.
//
// IMPORTANT: never swap the real `process.stdout.write` — the node:test reporter flushes test
// events through it asynchronously, so swapping it silently drops test registrations. Direct CLI
// output is captured via the redirectable `stdout` sink (src/core/output.ts) instead.
let chain: Promise<unknown> = Promise.resolve();

/** Run the specd CLI in `cwd`, capturing stdout/stderr and the exit code (serialized). */
export function run(cwd: string, ...argv: string[]): Promise<RunResult> {
  const result = chain.then(() => runOnce(cwd, ...argv));
  chain = result.catch(() => undefined);
  return result;
}

async function runOnce(cwd: string, ...argv: string[]): Promise<RunResult> {
  const prevCwd = process.cwd();
  const out: string[] = [];
  const err: string[] = [];
  const origLog = console.log;
  const origErr = console.error;
  const origSink = stdout.write;
  console.log = (...a: unknown[]) => { out.push(a.map(String).join(" ")); };
  console.error = (...a: unknown[]) => { err.push(a.map(String).join(" ")); };
  stdout.write = (s: string) => { out.push(s); };
  process.chdir(cwd);
  try {
    const rc = await main(argv);
    return { rc, out: out.join("\n"), err: err.join("\n") };
  } finally {
    process.chdir(prevCwd);
    console.log = origLog;
    console.error = origErr;
    stdout.write = origSink;
  }
}

export const newTmp = (): string => mkdtempSync(join(tmpdir(), "specd-cli-"));
