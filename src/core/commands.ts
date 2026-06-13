// Command registry and metadata for specd CLI.

export interface FlagMetadata {
  name: string;
  type: "string" | "boolean";
  description: string;
}

export interface ExitCodeMetadata {
  code: number;
  meaning: string;
}

export interface CommandMetadata {
  command: string;
  category: "lifecycle" | "execution" | "inspection" | "meta";
  description: string;
  usage: string;
  synopsis: string;
  longDescription: string;
  flags: FlagMetadata[];
  exitCodes: ExitCodeMetadata[];
  examples: string[];
}

export const COMMANDS: CommandMetadata[] = [
  {
    command: "init",
    category: "lifecycle",
    description: "Scaffold .specd/ + AGENTS.md",
    usage: "specd init [--force]",
    synopsis: "specd init [--force]",
    longDescription: "Scaffolds the .specd/ directory, roles, steering config, and AGENTS.md in the current working directory.",
    flags: [
      { name: "force", type: "boolean", description: "Overwrite an existing .specd/ directory" }
    ],
    exitCodes: [
      { code: 0, meaning: "Success" },
      { code: 2, meaning: "Usage error" },
      { code: 3, meaning: ".specd/ already exists (without --force)" }
    ],
    examples: ["specd init", "specd init --force"]
  },
  {
    command: "new",
    category: "lifecycle",
    description: "Create a spec with six artifacts",
    usage: "specd new <slug> [--title \"...\"]",
    synopsis: "specd new <slug> [--title \"...\"]",
    longDescription: "Creates a new spec directory under .specd/specs/<slug>/ with six artifact stubs (requirements, design, tasks, etc.).",
    flags: [
      { name: "title", type: "string", description: "The title of the spec" }
    ],
    exitCodes: [
      { code: 0, meaning: "Success" },
      { code: 2, meaning: "Usage error" },
      { code: 3, meaning: ".specd/ directory not found, or spec already exists" }
    ],
    examples: ["specd new my-feature", "specd new my-feature --title \"My Feature\""]
  },
  {
    command: "approve",
    category: "lifecycle",
    description: "Clear approval gate / advance phase",
    usage: "specd approve <slug> [--json]",
    synopsis: "specd approve <slug> [--json]",
    longDescription: "Clears an awaiting-approval gate or advances the planning phase of the specified spec (ANALYZE -> DESIGN -> IMPLEMENT -> VERIFY -> COMPLETE).",
    flags: [
      { name: "json", type: "boolean", description: "Output in JSON format" }
    ],
    exitCodes: [
      { code: 0, meaning: "Success" },
      { code: 1, meaning: "Gate validation failed" },
      { code: 2, meaning: "Usage error" },
      { code: 3, meaning: "Spec not found" }
    ],
    examples: ["specd approve my-feature", "specd approve my-feature --json"]
  },
  {
    command: "decision",
    category: "lifecycle",
    description: "Record an architectural decision (ADR)",
    usage: "specd decision <slug> \"<text>\" [--supersedes <id>]",
    synopsis: "specd decision <slug> \"<text>\" [--supersedes <id>]",
    longDescription: "Appends an architectural decision record (ADR) to the decisions.md artifact.",
    flags: [
      { name: "supersedes", type: "string", description: "ID of the decision being superseded" }
    ],
    exitCodes: [
      { code: 0, meaning: "Success" },
      { code: 2, meaning: "Usage error" },
      { code: 3, meaning: "Spec not found" }
    ],
    examples: ["specd decision my-feature \"Use SQLite instead of PostgreSQL\"", "specd decision my-feature \"Use PostgreSQL\" --supersedes D1"]
  },
  {
    command: "midreq",
    category: "lifecycle",
    description: "Log feedback mid-requirement",
    usage: "specd midreq <slug> \"<input>\" --impact <low|medium|high|critical> [--interpretation ..] [--changes ..]",
    synopsis: "specd midreq <slug> \"<input>\" --impact <low|medium|high|critical> [--interpretation ..] [--changes ..]",
    longDescription: "Logs user feedback or mid-requirement changes to the feedback.md artifact and triggers appropriate gates.",
    flags: [
      { name: "impact", type: "string", description: "The impact of the change (low, medium, high, critical)" },
      { name: "interpretation", type: "string", description: "The developer's interpretation of the feedback" },
      { name: "changes", type: "string", description: "Proposed spec changes in response to the feedback" }
    ],
    exitCodes: [
      { code: 0, meaning: "Success" },
      { code: 2, meaning: "Usage error" },
      { code: 3, meaning: "Spec not found" }
    ],
    examples: ["specd midreq my-feature \"Add search bar\" --impact medium"]
  },
  {
    command: "memory",
    category: "lifecycle",
    description: "Add or promote learning items",
    usage: "specd memory <slug> add --key <k> --pattern \"..\" --body \"..\" --source \"..\" --criticality <c> [--related ..] OR specd memory <slug> promote --key <k>",
    synopsis: "specd memory <slug> <add|promote> --key <k> [flags]",
    longDescription: "Manages persistent project learnings. 'add' logs a new learning item, and 'promote' elevates a local memory item to project level.",
    flags: [
      { name: "key", type: "string", description: "Unique identifier for the memory entry" },
      { name: "pattern", type: "string", description: "The pattern observed" },
      { name: "body", type: "string", description: "Description of the learning" },
      { name: "source", type: "string", description: "Source of the learning (e.g. log, test)" },
      { name: "criticality", type: "string", description: "Criticality score (1-5)" },
      { name: "related", type: "string", description: "Related keys comma-separated" },
      { name: "force", type: "boolean", description: "Override threshold during promotion" }
    ],
    exitCodes: [
      { code: 0, meaning: "Success" },
      { code: 2, meaning: "Usage error" },
      { code: 3, meaning: "Spec not found" }
    ],
    examples: ["specd memory my-feature add --key db-lock --pattern \"parallel write locks\" --body \"SQLite locks db on concurrent write\" --source log --criticality 4"]
  },
  {
    command: "next",
    category: "execution",
    description: "Next runnable task",
    usage: "specd next <slug> [--all] [--json]",
    synopsis: "specd next <slug> [--all] [--json]",
    longDescription: "Finds and prints the next runnable task in the spec's task DAG. Use --all to print the entire runnable frontier.",
    flags: [
      { name: "all", type: "boolean", description: "Print all runnable tasks in the frontier" },
      { name: "json", type: "boolean", description: "Output in JSON format" }
    ],
    exitCodes: [
      { code: 0, meaning: "Success" },
      { code: 2, meaning: "Usage error" },
      { code: 3, meaning: "Spec not found" }
    ],
    examples: ["specd next my-feature", "specd next my-feature --all --json"]
  },
  {
    command: "dispatch",
    category: "execution",
    description: "Ready-to-run packets for frontier",
    usage: "specd dispatch <slug> [--json]",
    synopsis: "specd dispatch <slug> [--json]",
    longDescription: "Produces ready-to-run packets (role + contract + verification script) for all tasks in the current runnable frontier.",
    flags: [
      { name: "json", type: "boolean", description: "Output in JSON format" }
    ],
    exitCodes: [
      { code: 0, meaning: "Success" },
      { code: 2, meaning: "Usage error" },
      { code: 3, meaning: "Spec not found" }
    ],
    examples: ["specd dispatch my-feature", "specd dispatch my-feature --json"]
  },
  {
    command: "verify",
    category: "execution",
    description: "Run task verify command / record proof",
    usage: "specd verify <slug> <id> OR specd verify <slug> --criterion <r>.<n> --status pass|fail --evidence \"...\"",
    synopsis: "specd verify <slug> [id] [flags]",
    longDescription: "Runs a task's verification command and records a verified evidence proof, or logs verification results for a specific acceptance criterion.",
    flags: [
      { name: "criterion", type: "string", description: "Requirement criterion ID (e.g. 1.1)" },
      { name: "status", type: "string", description: "Verification status (pass or fail)" },
      { name: "evidence", type: "string", description: "Evidence text supporting the status" }
    ],
    exitCodes: [
      { code: 0, meaning: "Success" },
      { code: 1, meaning: "Verification failed (command exited non-zero)" },
      { code: 2, meaning: "Usage error" },
      { code: 3, meaning: "Spec or task not found" }
    ],
    examples: ["specd verify my-feature T1", "specd verify my-feature --criterion 1.1 --status pass --evidence \"Tested manually, search returns expected items\""]
  },
  {
    command: "task",
    category: "execution",
    description: "Evidence-gated status flip",
    usage: "specd task <slug> <id> --status <s> [--evidence \"...\"] [--reason \"...\"] [--force]",
    synopsis: "specd task <slug> <id> --status <status> [flags]",
    longDescription: "Updates the status of a specific task. Flipping a task to 'complete' requires evidence and verified proofs unless forced.",
    flags: [
      { name: "status", type: "string", description: "Target status (pending, doing, blocked, complete)" },
      { name: "evidence", type: "string", description: "Explanation of how the task was verified" },
      { name: "reason", type: "string", description: "Reason why the task is blocked (required if status is blocked)" },
      { name: "force", type: "boolean", description: "Bypass evidence and dependency checks" },
      { name: "unverified", type: "boolean", description: "Mark task as complete with manual/unverified evidence" }
    ],
    exitCodes: [
      { code: 0, meaning: "Success" },
      { code: 1, meaning: "Gate verification failed (dependencies not met or no verified record)" },
      { code: 2, meaning: "Usage error" },
      { code: 3, meaning: "Spec or task not found" }
    ],
    examples: ["specd task my-feature T1 --status doing", "specd task my-feature T1 --status complete --evidence \"Ran npm test successfully\""]
  },
  {
    command: "status",
    category: "inspection",
    description: "Render durable ledger / board",
    usage: "specd status [<slug>] [--json]",
    synopsis: "specd status [<slug>] [--json]",
    longDescription: "Renders the durable status board of a specific spec, or lists all specs and their active phases if no slug is provided.",
    flags: [
      { name: "json", type: "boolean", description: "Output in JSON format" }
    ],
    exitCodes: [
      { code: 0, meaning: "Success" },
      { code: 2, meaning: "Usage error" },
      { code: 3, meaning: "Spec not found" }
    ],
    examples: ["specd status", "specd status my-feature --json"]
  },
  {
    command: "check",
    category: "inspection",
    description: "Run all validation gates",
    usage: "specd check <slug> [--json]",
    synopsis: "specd check <slug> [--json]",
    longDescription: "Runs all seven validation gates (§10) on the specified spec, checking requirements, design, tasks schema, DAG, sync, evidence, and traceability.",
    flags: [
      { name: "json", type: "boolean", description: "Output in JSON format" }
    ],
    exitCodes: [
      { code: 0, meaning: "Success" },
      { code: 1, meaning: "Validation failed (one or more gates failed)" },
      { code: 2, meaning: "Usage error" },
      { code: 3, meaning: "Spec not found" }
    ],
    examples: ["specd check my-feature", "specd check my-feature --json"]
  },
  {
    command: "context",
    category: "inspection",
    description: "Phase-scoped briefing",
    usage: "specd context <slug> [--json]",
    synopsis: "specd context <slug> [--json]",
    longDescription: "Provides a minimal phase-scoped briefing outlining what files to load and the next action based on the current spec phase.",
    flags: [
      { name: "json", type: "boolean", description: "Output in JSON format" }
    ],
    exitCodes: [
      { code: 0, meaning: "Success" },
      { code: 2, meaning: "Usage error" },
      { code: 3, meaning: "Spec not found" }
    ],
    examples: ["specd context my-feature", "specd context my-feature --json"]
  },
  {
    command: "report",
    category: "inspection",
    description: "Generate markdown or HTML report",
    usage: "specd report <slug> [--format md|html] [--out <path>]",
    synopsis: "specd report <slug> [--format md|html] [--out <path>]",
    longDescription: "Compiles a comprehensive HTML or Markdown progress report for the specified spec.",
    flags: [
      { name: "format", type: "string", description: "Report format (md or html, default is md)" },
      { name: "out", type: "string", description: "Destination path for the report" }
    ],
    exitCodes: [
      { code: 0, meaning: "Success" },
      { code: 2, meaning: "Usage error" },
      { code: 3, meaning: "Spec not found" }
    ],
    examples: ["specd report my-feature --format html --out ./report.html"]
  },
  {
    command: "waves",
    category: "inspection",
    description: "Show task wave DAG",
    usage: "specd waves <slug> [--json]",
    synopsis: "specd waves <slug> [--json]",
    longDescription: "Renders the task wave dependency graph (DAG) in ASCII representation or JSON.",
    flags: [
      { name: "json", type: "boolean", description: "Output in JSON format" }
    ],
    exitCodes: [
      { code: 0, meaning: "Success" },
      { code: 2, meaning: "Usage error" },
      { code: 3, meaning: "Spec not found" }
    ],
    examples: ["specd waves my-feature", "specd waves my-feature --json"]
  },
  {
    command: "program",
    category: "inspection",
    description: "Cross-spec DAG runnable frontier",
    usage: "specd program [status] [--json] OR specd program <link|unlink> <spec> --on <dep>",
    synopsis: "specd program [status|link|unlink] [args] [flags]",
    longDescription: "Manages cross-spec dependencies. Displays the cross-spec runnable frontier, or links/unlinks specs dynamically.",
    flags: [
      { name: "on", type: "string", description: "The dependency spec slug" },
      { name: "json", type: "boolean", description: "Output in JSON format" }
    ],
    exitCodes: [
      { code: 0, meaning: "Success" },
      { code: 2, meaning: "Usage error" }
    ],
    examples: ["specd program status", "specd program link feat-b --on feat-a"]
  },
  {
    command: "update",
    category: "meta",
    description: "Update specd to latest version",
    usage: "specd update [--force]",
    synopsis: "specd update [--force]",
    longDescription: "Queries GitHub for the latest release/commit, and if a newer version is found, performs an in-place update by pulling, running 'npm install', and running 'npm run build'.",
    flags: [
      { name: "force", type: "boolean", description: "Force re-clone and rebuild even if versions match" }
    ],
    exitCodes: [
      { code: 0, meaning: "Success" },
      { code: 1, meaning: "Update failed (compilation error, check connection)" },
      { code: 2, meaning: "Usage error" }
    ],
    examples: ["specd update", "specd update --force"]
  },
  {
    command: "version",
    category: "meta",
    description: "Show version information",
    usage: "specd version",
    synopsis: "specd version",
    longDescription: "Prints the version of the installed specd binary.",
    flags: [],
    exitCodes: [
      { code: 0, meaning: "Success" }
    ],
    examples: ["specd version"]
  },
  {
    command: "help",
    category: "meta",
    description: "Show detailed help for a command",
    usage: "specd help [command]",
    synopsis: "specd help [command]",
    longDescription: "Prints summary documentation for all commands, or detailed reference pages for a single command.",
    flags: [
      { name: "json", type: "boolean", description: "Output the entire command registry as a JSON schema" }
    ],
    exitCodes: [
      { code: 0, meaning: "Success" },
      { code: 2, meaning: "Usage error (unknown command)" }
    ],
    examples: ["specd help", "specd help init", "specd help --json"]
  }
];
