// Bespoke line parser + serializer for the constrained tasks.md format (SPEC §7.3).
// No markdown AST lib — the format is deliberately rigid. Serialization is canonical so a
// round-trip of a canonical document is byte-stable.
import { gateError } from "./exit.js";
import { stripHtmlComments } from "./md.js";

export const MANDATORY_KEYS = ["why", "role", "files", "contract", "acceptance", "verify", "depends"] as const;
export const KEY_ORDER = [...MANDATORY_KEYS, "requirements"] as const;
export const VALID_ROLES = ["investigator", "builder", "reviewer", "verifier"] as const;
export const READONLY_ROLES = ["investigator", "reviewer"] as const;

export type MetaKey = (typeof KEY_ORDER)[number];

export type Annotation =
  | { kind: "complete"; evidence: string; ts: string }
  | { kind: "blocked"; reason: string };

export interface ParsedTask {
  id: string;
  title: string; // bare title, annotation stripped
  wave: number;
  checked: boolean; // [x]
  meta: Record<MetaKey, string>;
  annotation?: Annotation;
  line: number; // 1-based source line of the task item (for error reporting)
}

export interface ParsedTasks {
  title: string;
  tasks: ParsedTask[]; // document order
}

export interface ParseIssue {
  line: number;
  message: string;
}

const TASK_RE = /^- \[( |x)\] (T\d+) — (.*)$/;
const WAVE_RE = /^## Wave (\d+)\s*$/;
const TITLE_RE = /^# Tasks — (.*)$/;
const META_RE = /^  - ([a-z]+): (.*)$/;

const ANNOT_COMPLETE_RE = / ✓ complete · evidence: (.*?) · ([^·]*)$/;
const ANNOT_BLOCKED_RE = / ⚠ blocked · reason: (.*)$/;

/** Strip a status annotation off a task title, returning the bare title + parsed annotation. */
function splitAnnotation(rawTitle: string): { title: string; annotation?: Annotation } {
  let m = rawTitle.match(ANNOT_COMPLETE_RE);
  if (m) {
    return {
      title: rawTitle.slice(0, m.index).trimEnd(),
      annotation: { kind: "complete", evidence: m[1], ts: m[2].trim() },
    };
  }
  m = rawTitle.match(ANNOT_BLOCKED_RE);
  if (m) {
    return {
      title: rawTitle.slice(0, m.index).trimEnd(),
      annotation: { kind: "blocked", reason: m[1] },
    };
  }
  return { title: rawTitle };
}

/** Parse depends value into a list of task ids ('—' / 'none' / empty -> []). */
export function parseDepends(value: string): string[] {
  const v = value.trim();
  if (v === "" || v === "—" || v === "-" || v.toLowerCase() === "none") return [];
  return v.split(",").map((s) => s.trim()).filter(Boolean);
}

/** Parse requirements value into req numbers (best effort; non-numeric tokens dropped). */
export function parseRequirements(value: string): number[] {
  return value
    .split(",")
    .map((s) => parseInt(s.trim(), 10))
    .filter((n) => !Number.isNaN(n));
}

/**
 * Parse a tasks.md document. Throws SpecdError(1) with line number on structural errors
 * (so the CLI surfaces a located message). Missing mandatory keys are also reported here.
 */
export function parseTasks(text: string): ParsedTasks {
  const lines = stripHtmlComments(text).split("\n");
  let title = "";
  let currentWave = 0;
  const tasks: ParsedTask[] = [];
  let current: ParsedTask | null = null;

  const flush = () => {
    if (!current) return;
    const missing = MANDATORY_KEYS.filter((k) => !(k in current!.meta));
    if (missing.length) {
      throw gateError(`tasks.md:${current.line}: task ${current.id} missing key(s): ${missing.join(", ")}`);
    }
    tasks.push(current);
    current = null;
  };

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const lineNo = i + 1;

    const tm = line.match(TITLE_RE);
    if (tm) { title = tm[1].trim(); continue; }

    const wm = line.match(WAVE_RE);
    if (wm) { flush(); currentWave = parseInt(wm[1], 10); continue; }

    const taskM = line.match(TASK_RE);
    if (taskM) {
      flush();
      if (currentWave === 0) {
        throw gateError(`tasks.md:${lineNo}: task ${taskM[2]} appears before any '## Wave N' header`);
      }
      const { title: bare, annotation } = splitAnnotation(taskM[3]);
      current = {
        id: taskM[2],
        title: bare,
        wave: currentWave,
        checked: taskM[1] === "x",
        meta: {} as Record<MetaKey, string>,
        annotation,
        line: lineNo,
      };
      continue;
    }

    const mm = line.match(META_RE);
    if (mm) {
      if (!current) {
        throw gateError(`tasks.md:${lineNo}: metadata '${mm[1]}' outside of a task`);
      }
      const key = mm[1] as MetaKey;
      if (!(KEY_ORDER as readonly string[]).includes(key)) {
        throw gateError(`tasks.md:${lineNo}: unknown metadata key '${key}'`);
      }
      current.meta[key] = mm[2].trim();
      continue;
    }
    // blank lines / prose are ignored
  }
  flush();

  if (!title) throw gateError("tasks.md:1: missing '# Tasks — <Title>' header");
  return { title, tasks };
}

/** Render a single task block in canonical form. */
function serializeTask(t: ParsedTask): string {
  let titleLine = `- [${t.checked ? "x" : " "}] ${t.id} — ${t.title}`;
  if (t.annotation?.kind === "complete") {
    titleLine += ` ✓ complete · evidence: ${t.annotation.evidence} · ${t.annotation.ts}`;
  } else if (t.annotation?.kind === "blocked") {
    titleLine += ` ⚠ blocked · reason: ${t.annotation.reason}`;
  }
  const metaLines = KEY_ORDER.filter((k) => k in t.meta).map((k) => `  - ${k}: ${t.meta[k]}`);
  return [titleLine, ...metaLines].join("\n");
}

/** Serialize a parsed model back to canonical tasks.md text (byte-stable round-trip). */
export function serializeTasks(doc: ParsedTasks): string {
  const out: string[] = [`# Tasks — ${doc.title}`, ""];
  const waves = [...new Set(doc.tasks.map((t) => t.wave))].sort((a, b) => a - b);
  for (const w of waves) {
    out.push(`## Wave ${w}`);
    const inWave = doc.tasks.filter((t) => t.wave === w);
    for (const t of inWave) {
      out.push(serializeTask(t), "");
    }
  }
  // trim trailing blank, end with single newline
  while (out.length && out[out.length - 1] === "") out.pop();
  return out.join("\n") + "\n";
}

/** Find a task by id or return null. */
export function findTask(doc: ParsedTasks, id: string): ParsedTask | null {
  return doc.tasks.find((t) => t.id === id) ?? null;
}

/** Build the canonical task title line for a checkbox + annotation. */
export function renderTaskLine(id: string, bareTitle: string, checked: boolean, annotation?: Annotation): string {
  let line = `- [${checked ? "x" : " "}] ${id} — ${bareTitle}`;
  if (annotation?.kind === "complete") {
    line += ` ✓ complete · evidence: ${annotation.evidence} · ${annotation.ts}`;
  } else if (annotation?.kind === "blocked") {
    line += ` ⚠ blocked · reason: ${annotation.reason}`;
  }
  return line;
}

/**
 * Surgically rewrite the single task line for `id` in raw tasks.md text, preserving all other
 * content (comments, prose, blank lines) byte-for-byte. Throws if the task line is absent.
 */
export function applyTaskAnnotation(
  text: string,
  id: string,
  checked: boolean,
  annotation: Annotation | undefined,
): string {
  const lines = text.split("\n");
  const scan = stripHtmlComments(text).split("\n"); // commented task lines blanked here
  for (let i = 0; i < lines.length; i++) {
    const m = scan[i].match(TASK_RE);
    if (m && m[2] === id) {
      const { title } = splitAnnotation(m[3]);
      lines[i] = renderTaskLine(id, title, checked, annotation);
      return lines.join("\n");
    }
  }
  throw gateError(`tasks.md: task line for '${id}' not found`);
}
