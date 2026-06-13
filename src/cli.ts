#!/usr/bin/env node
// specd — agent-agnostic, spec-driven coding harness CLI (SPEC §5).
// Arg routing + exit-code contract. All state mutation lives in commands/*.
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { EXIT, SpecdError } from "./core/exit.js";

/** Read the package version from the shipped package.json (sibling of src/ and dist/). */
function readVersion(): string {
  try {
    const here = dirname(fileURLToPath(import.meta.url));
    const pkg = JSON.parse(readFileSync(join(here, "..", "package.json"), "utf8")) as { version?: string };
    return `specd ${pkg.version ?? "unknown"}`;
  } catch { return "specd (unknown version)"; }
}

export interface Args {
  pos: string[];
  flags: Record<string, string | true>;
}

const BOOLEAN_FLAGS = new Set(["force", "json", "all", "unverified"]);

/** Hand-rolled arg parser: positionals + `--key value` / `--flag` (no deps). */
export function parseArgs(argv: string[]): Args {
  const pos: string[] = [];
  const flags: Record<string, string | true> = {};
  for (let i = 0; i < argv.length; i++) {
    const tok = argv[i];
    if (tok.startsWith("--")) {
      const key = tok.slice(2);
      if (BOOLEAN_FLAGS.has(key)) {
        flags[key] = true;
      } else if (i + 1 < argv.length && !argv[i + 1].startsWith("--")) {
        flags[key] = argv[++i];
      } else {
        flags[key] = true;
      }
    } else {
      pos.push(tok);
    }
  }
  return { pos, flags };
}

const USAGE = `specd — spec-driven coding harness

Usage: specd <command> [args] [flags]

Commands:
  init [--force]                      Scaffold .specd/ + AGENTS.md
  new <slug> [--title "..."]          Create a spec with the six artifacts
  status [<slug>] [--json]            Render the durable ledger / board
  program [status] [--json]           Cross-spec DAG: which specs are runnable across the program now
  program link <spec> --on <dep>      Declare a cross-spec dependency edge
  program unlink <spec> --on <dep>    Remove a cross-spec dependency edge
  context <slug> [--json]             Minimal phase-scoped briefing (what to load + next action)
  check <slug> [--json]               Run all validation gates (§10)
  next <slug> [--all] [--json]        Next runnable task; --all prints the whole runnable frontier
  dispatch <slug> [--json]            Ready-to-run packets for the runnable frontier (role+contract+verify)
  verify <slug> <id>                  Run the task's verify: command, record a verified evidence proof
  verify <slug> --criterion <r>.<n> --status pass|fail --evidence "..."  Record a per-criterion VERIFY proof (G5)
  task <slug> <id> --status <s> [..]  Evidence-gated task status flip (complete needs a verified proof)
  approve <slug> [--json]             Clear an awaiting-approval gate or advance the planning phase
  decision <slug> "<text>" [--supersedes <id>]
  midreq <slug> "<input>" --impact <low|medium|high|critical> [--interpretation ..] [--changes ..]
  memory <slug> add --key <k> --pattern ".." --body ".." --source ".." --criticality <c> [--related ..]
  memory <slug> promote --key <k>
  report <slug> [--format md|html] [--out <path>]
  waves <slug> [--json]

Exit codes: 0 ok · 1 gate/validation · 2 usage · 3 not found`;

type CommandFn = (args: Args) => void | number | Promise<void | number>;

async function dispatch(cmd: string, args: Args): Promise<number> {
  const load = async (): Promise<CommandFn> => {
    switch (cmd) {
      case "init": return (await import("./commands/init.js")).run;
      case "new": return (await import("./commands/new.js")).run;
      case "status": return (await import("./commands/status.js")).run;
      case "context": return (await import("./commands/context.js")).run;
      case "check": return (await import("./commands/check.js")).run;
      case "next": return (await import("./commands/next.js")).run;
      case "dispatch": return (await import("./commands/dispatch.js")).run;
      case "program": return (await import("./commands/program.js")).run;
      case "verify": return (await import("./commands/verify.js")).run;
      case "task": return (await import("./commands/task.js")).run;
      case "approve": return (await import("./commands/approve.js")).run;
      case "decision": return (await import("./commands/decision.js")).run;
      case "midreq": return (await import("./commands/midreq.js")).run;
      case "memory": return (await import("./commands/memory.js")).run;
      case "report": return (await import("./commands/report.js")).run;
      case "waves": return (await import("./commands/waves.js")).run;
      default:
        throw new SpecdError(EXIT.USAGE, `unknown command: ${cmd}\n\n${USAGE}`);
    }
  };
  const fn = await load();
  const rc = await fn(args);
  return typeof rc === "number" ? rc : EXIT.OK;
}

export async function main(argv: string[]): Promise<number> {
  if (argv.length === 0 || argv[0] === "--help" || argv[0] === "-h" || argv[0] === "help") {
    console.log(USAGE);
    return argv.length === 0 ? EXIT.USAGE : EXIT.OK;
  }
  if (argv[0] === "--version" || argv[0] === "-v" || argv[0] === "version") {
    console.log(readVersion());
    return EXIT.OK;
  }
  const [cmd, ...rest] = argv;
  try {
    return await dispatch(cmd, parseArgs(rest));
  } catch (err) {
    if (err instanceof SpecdError) {
      console.error(`error: ${err.message}`);
      return err.code;
    }
    console.error(`error: ${err instanceof Error ? err.message : String(err)}`);
    return EXIT.GATE;
  }
}

// Run when invoked as a binary (not when imported by tests).
const isMain = process.argv[1] && (import.meta.url === `file://${process.argv[1]}` || process.argv[1].endsWith("cli.ts") || process.argv[1].endsWith("cli.js"));
if (isMain) {
  main(process.argv.slice(2)).then((code) => process.exit(code));
}
