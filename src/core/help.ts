// Help rendering engine for specd.
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { COMMANDS } from "./commands.js";

function getVersion(): string {
  try {
    const here = dirname(fileURLToPath(import.meta.url));
    const pkg = JSON.parse(readFileSync(join(here, "..", "..", "package.json"), "utf8")) as { version?: string };
    return pkg.version ?? "unknown";
  } catch {
    return "unknown";
  }
}

export function renderHelp(): string {
  const version = getVersion();
  const categories = {
    lifecycle: "LIFECYCLE",
    execution: "EXECUTION",
    inspection: "INSPECTION",
    meta: "META",
  };

  let out = `specd — spec-driven coding harness v${version}\n\n`;

  for (const [key, label] of Object.entries(categories)) {
    out += `${label}\n`;
    const cmds = COMMANDS.filter((c) => c.category === key);
    for (const c of cmds) {
      const cmdStr = `  ${c.usage.replace("specd ", "")}`;
      const paddedCmd = cmdStr.padEnd(30, " ");
      out += `${paddedCmd}${c.description}\n`;
    }
    out += `\n`;
  }
  out += `Use "specd help <command>" for detailed usage of a command.\n`;
  return out;
}

export function renderCommandHelp(cmdName: string): string {
  const c = COMMANDS.find((x) => x.command === cmdName);
  if (!c) {
    throw new Error(`Unknown command: ${cmdName}`);
  }

  let out = `NAME\n  specd ${c.command} — ${c.description}\n\n`;
  out += `SYNOPSIS\n  ${c.usage}\n\n`;
  out += `DESCRIPTION\n  ${c.longDescription}\n\n`;

  if (c.flags.length > 0) {
    out += `FLAGS\n`;
    for (const f of c.flags) {
      const typeStr = f.type === "string" ? " <val>" : "";
      out += `  --${f.name}${typeStr}    ${f.description}\n`;
    }
    out += `\n`;
  }

  if (c.exitCodes.length > 0) {
    out += `EXIT CODES\n`;
    for (const e of c.exitCodes) {
      out += `  ${e.code}  ${e.meaning}\n`;
    }
    out += `\n`;
  }

  if (c.examples.length > 0) {
    out += `EXAMPLE\n`;
    for (const ex of c.examples) {
      out += `  ${ex}\n`;
    }
  }

  return out;
}

export function renderHelpJson(): string {
  return JSON.stringify(COMMANDS, null, 2);
}
