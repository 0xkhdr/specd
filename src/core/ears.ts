// EARS grammar linter for requirements.md (SPEC §7.1, §10 gate 1).
import { stripHtmlComments } from "./md.js";

export type EarsPattern =
  | "ubiquitous" | "event-driven" | "state-driven" | "optional-feature" | "unwanted";

const PATTERNS: { name: EarsPattern; re: RegExp }[] = [
  { name: "unwanted", re: /^IF .+ THEN THE SYSTEM SHALL .+/i },
  { name: "event-driven", re: /^WHEN .+ THE SYSTEM SHALL .+/i },
  { name: "state-driven", re: /^WHILE .+ THE SYSTEM SHALL .+/i },
  { name: "optional-feature", re: /^WHERE .+ THE SYSTEM SHALL .+/i },
  { name: "ubiquitous", re: /^THE SYSTEM SHALL .+/i },
];

/** Classify a criterion line. Returns the matched EARS pattern or null. */
export function matchEars(line: string): EarsPattern | null {
  const text = line.trim();
  for (const p of PATTERNS) if (p.re.test(text)) return p.name;
  return null;
}

export interface EarsIssue {
  line: number;
  message: string;
}

const REQ_HEADER_RE = /^##\s+Requirement\b/i;
const USER_STORY_RE = /\*\*User story:\*\*/i;
const CRITERION_RE = /^\s*\d+\.\s+(.*)$/;

interface ReqBlock {
  headerLine: number;
  name: string;
  hasUserStory: boolean;
  criteria: number;
}

/** Lint a requirements.md document. Returns all violations (empty = valid). */
export function lintEars(text: string): EarsIssue[] {
  const lines = stripHtmlComments(text).split("\n");
  const issues: EarsIssue[] = [];
  const blocks: ReqBlock[] = [];
  let current: ReqBlock | null = null;
  let inAcceptance = false;

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const lineNo = i + 1;

    if (REQ_HEADER_RE.test(line)) {
      current = { headerLine: lineNo, name: line.replace(/^##\s+/, "").trim(), hasUserStory: false, criteria: 0 };
      blocks.push(current);
      inAcceptance = false;
      continue;
    }
    if (!current) continue;

    if (USER_STORY_RE.test(line)) { current.hasUserStory = true; continue; }
    if (/\*\*Acceptance criteria:\*\*/i.test(line)) { inAcceptance = true; continue; }

    const cm = line.match(CRITERION_RE);
    if (cm && inAcceptance) {
      current.criteria++;
      if (!matchEars(cm[1])) {
        issues.push({ line: lineNo, message: `criterion does not match any EARS pattern: "${cm[1].trim()}"` });
      }
    }
  }

  for (const b of blocks) {
    if (!b.hasUserStory) issues.push({ line: b.headerLine, message: `requirement "${b.name}" missing **User story:** line` });
    if (b.criteria === 0) issues.push({ line: b.headerLine, message: `requirement "${b.name}" has no acceptance criteria` });
  }
  if (blocks.length === 0) issues.push({ line: 1, message: "no '## Requirement N' sections found" });

  return issues;
}
