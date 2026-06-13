// Read shipped templates and apply simple {{VAR}} substitution.
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { templatesDir } from "./paths.js";

/** Read a template by path relative to the templates dir. */
export function readTemplate(rel: string): string {
  return readFileSync(join(templatesDir(), rel), "utf8");
}

/** Replace {{KEY}} occurrences with provided values. */
export function applyVars(text: string, vars: Record<string, string>): string {
  return text.replace(/\{\{(\w+)\}\}/g, (m, k) => (k in vars ? vars[k] : m));
}
