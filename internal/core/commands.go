package core

type FlagMeta struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type ExitCodeMeta struct {
	Code    int    `json:"code"`
	Meaning string `json:"meaning"`
}

type CommandMeta struct {
	Command         string         `json:"command"`
	Category        string         `json:"category"`
	Description     string         `json:"description"`
	Usage           string         `json:"usage"`
	Synopsis        string         `json:"synopsis"`
	LongDescription string         `json:"longDescription"`
	Flags           []FlagMeta     `json:"flags"`
	ExitCodes       []ExitCodeMeta `json:"exitCodes"`
	Examples        []string       `json:"examples"`
}

var Commands = []CommandMeta{
	{
		Command: "init", Category: "lifecycle",
		Description: "Scaffold .specd/ + AGENTS.md",
		Usage: "specd init [--force]", Synopsis: "specd init [--force]",
		LongDescription: "Scaffolds the .specd/ directory, roles, steering config, and AGENTS.md in the current working directory.",
		Flags:     []FlagMeta{{Name: "force", Type: "boolean", Description: "Overwrite an existing .specd/ directory"}},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, ".specd/ already exists (without --force)"}},
		Examples:  []string{"specd init", "specd init --force"},
	},
	{
		Command: "new", Category: "lifecycle",
		Description: "Create a spec with six artifacts",
		Usage: "specd new <slug> [--title \"...\"]", Synopsis: "specd new <slug> [--title \"...\"]",
		LongDescription: "Creates a new spec directory under .specd/specs/<slug>/ with six artifact stubs.",
		Flags:     []FlagMeta{{Name: "title", Type: "string", Description: "The title of the spec"}},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, ".specd/ not found or spec already exists"}},
		Examples:  []string{"specd new my-feature", "specd new my-feature --title \"My Feature\""},
	},
	{
		Command: "approve", Category: "lifecycle",
		Description: "Clear approval gate / advance phase",
		Usage: "specd approve <slug> [--json]", Synopsis: "specd approve <slug> [--json]",
		LongDescription: "Clears an awaiting-approval gate or advances the planning phase of the specified spec.",
		Flags:     []FlagMeta{{Name: "json", Type: "boolean", Description: "Output in JSON format"}},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {1, "Gate validation failed"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:  []string{"specd approve my-feature", "specd approve my-feature --json"},
	},
	{
		Command: "decision", Category: "lifecycle",
		Description: "Record an architectural decision (ADR)",
		Usage: "specd decision <slug> \"<text>\" [--supersedes <id>]", Synopsis: "specd decision <slug> \"<text>\" [--supersedes <id>]",
		LongDescription: "Appends an architectural decision record (ADR) to decisions.md.",
		Flags:     []FlagMeta{{Name: "supersedes", Type: "string", Description: "ID of the decision being superseded"}},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:  []string{"specd decision my-feature \"Use SQLite instead of PostgreSQL\""},
	},
	{
		Command: "midreq", Category: "lifecycle",
		Description: "Log feedback mid-requirement",
		Usage: "specd midreq <slug> \"<input>\" --impact <low|medium|high|critical>", Synopsis: "specd midreq <slug> \"<input>\" --impact <low|medium|high|critical>",
		LongDescription: "Logs user feedback or mid-requirement changes and triggers appropriate gates.",
		Flags:     []FlagMeta{{Name: "impact", Type: "string", Description: "Impact level (low/medium/high/critical)"}, {Name: "interpretation", Type: "string"}, {Name: "changes", Type: "string"}},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:  []string{"specd midreq my-feature \"Add search bar\" --impact medium"},
	},
	{
		Command: "memory", Category: "lifecycle",
		Description: "Add or promote learning items",
		Usage: "specd memory <slug> add|promote [flags]", Synopsis: "specd memory <slug> <add|promote> [flags]",
		LongDescription: "Manages persistent project learnings.",
		Flags:     []FlagMeta{{Name: "key", Type: "string"}, {Name: "pattern", Type: "string"}, {Name: "body", Type: "string"}, {Name: "source", Type: "string"}, {Name: "criticality", Type: "string"}, {Name: "related", Type: "string"}, {Name: "force", Type: "boolean"}},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:  []string{"specd memory my-feature add --key db-lock --pattern \"parallel writes\" --body \"SQLite locks on concurrent write\" --source log --criticality important"},
	},
	{
		Command: "next", Category: "execution",
		Description: "Next runnable task",
		Usage: "specd next <slug> [--all] [--json]", Synopsis: "specd next <slug> [--all] [--json]",
		LongDescription: "Finds and prints the next runnable task in the spec's task DAG.",
		Flags:     []FlagMeta{{Name: "all", Type: "boolean", Description: "Print all runnable tasks"}, {Name: "json", Type: "boolean"}},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:  []string{"specd next my-feature", "specd next my-feature --all --json"},
	},
	{
		Command: "dispatch", Category: "execution",
		Description: "Ready-to-run packets for frontier",
		Usage: "specd dispatch <slug> [--json]", Synopsis: "specd dispatch <slug> [--json]",
		LongDescription: "Produces ready-to-run packets for all tasks in the current runnable frontier.",
		Flags:     []FlagMeta{{Name: "json", Type: "boolean"}},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:  []string{"specd dispatch my-feature --json"},
	},
	{
		Command: "verify", Category: "execution",
		Description: "Run task verify command / record proof",
		Usage: "specd verify <slug> <id>  |  specd verify <slug> --criterion <r>.<n> --status pass|fail --evidence \"...\"",
		Synopsis: "specd verify <slug> [id] [flags]",
		LongDescription: "Runs a task's verification command and records the result, or records a per-criterion acceptance proof.",
		Flags:     []FlagMeta{{Name: "criterion", Type: "string"}, {Name: "status", Type: "string"}, {Name: "evidence", Type: "string"}},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {1, "Verification failed"}, {2, "Usage error"}, {3, "Spec or task not found"}},
		Examples:  []string{"specd verify my-feature T1", "specd verify my-feature --criterion 1.1 --status pass --evidence \"Tested manually\""},
	},
	{
		Command: "task", Category: "execution",
		Description: "Evidence-gated status flip",
		Usage: "specd task <slug> <id> --status <s> [--evidence \"...\"] [--reason \"...\"] [--force]",
		Synopsis: "specd task <slug> <id> --status <status> [flags]",
		LongDescription: "Updates the status of a specific task. Completing requires a passing verify record.",
		Flags:     []FlagMeta{{Name: "status", Type: "string"}, {Name: "evidence", Type: "string"}, {Name: "reason", Type: "string"}, {Name: "force", Type: "boolean"}, {Name: "unverified", Type: "boolean"}},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {1, "Gate verification failed"}, {2, "Usage error"}, {3, "Spec or task not found"}},
		Examples:  []string{"specd task my-feature T1 --status running", "specd task my-feature T1 --status complete --evidence \"Ran tests\""},
	},
	{
		Command: "status", Category: "inspection",
		Description: "Render durable ledger / board",
		Usage: "specd status [<slug>] [--json]", Synopsis: "specd status [<slug>] [--json]",
		LongDescription: "Renders the durable status board of a specific spec, or lists all specs.",
		Flags:     []FlagMeta{{Name: "json", Type: "boolean"}},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:  []string{"specd status", "specd status my-feature --json"},
	},
	{
		Command: "check", Category: "inspection",
		Description: "Run all validation gates",
		Usage: "specd check <slug> [--json]", Synopsis: "specd check <slug> [--json]",
		LongDescription: "Runs all seven validation gates on the specified spec.",
		Flags:     []FlagMeta{{Name: "json", Type: "boolean"}},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {1, "Validation failed"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:  []string{"specd check my-feature", "specd check my-feature --json"},
	},
	{
		Command: "context", Category: "inspection",
		Description: "Phase-scoped briefing",
		Usage: "specd context <slug> [--json]", Synopsis: "specd context <slug> [--json]",
		LongDescription: "Provides a minimal phase-scoped briefing for the current spec phase.",
		Flags:     []FlagMeta{{Name: "json", Type: "boolean"}},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:  []string{"specd context my-feature", "specd context my-feature --json"},
	},
	{
		Command: "report", Category: "inspection",
		Description: "Generate markdown or HTML report",
		Usage: "specd report <slug> [--format md|html] [--out <path>]", Synopsis: "specd report <slug> [--format md|html] [--out <path>]",
		LongDescription: "Compiles a comprehensive HTML or Markdown progress report.",
		Flags:     []FlagMeta{{Name: "format", Type: "string"}, {Name: "out", Type: "string"}},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:  []string{"specd report my-feature --format html --out ./report.html"},
	},
	{
		Command: "waves", Category: "inspection",
		Description: "Show task wave DAG",
		Usage: "specd waves <slug> [--json]", Synopsis: "specd waves <slug> [--json]",
		LongDescription: "Renders the task wave dependency graph in ASCII or JSON.",
		Flags:     []FlagMeta{{Name: "json", Type: "boolean"}},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}, {3, "Spec not found"}},
		Examples:  []string{"specd waves my-feature"},
	},
	{
		Command: "program", Category: "inspection",
		Description: "Cross-spec DAG runnable frontier",
		Usage: "specd program [status] [--json]  |  specd program <link|unlink> <spec> --on <dep>",
		Synopsis: "specd program [status|link|unlink] [args] [flags]",
		LongDescription: "Manages cross-spec dependencies and displays the cross-spec runnable frontier.",
		Flags:     []FlagMeta{{Name: "on", Type: "string"}, {Name: "json", Type: "boolean"}},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {2, "Usage error"}},
		Examples:  []string{"specd program status", "specd program link feat-b --on feat-a"},
	},
	{
		Command: "update", Category: "meta",
		Description: "Update specd to latest version",
		Usage: "specd update [--force]", Synopsis: "specd update [--force]",
		LongDescription: "Downloads the latest pre-built binary from GitHub Releases and replaces the running binary.",
		Flags:     []FlagMeta{{Name: "force", Type: "boolean", Description: "Force update even if already up to date"}},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {1, "Update failed"}, {2, "Usage error"}},
		Examples:  []string{"specd update", "specd update --force"},
	},
	{
		Command: "version", Category: "meta",
		Description: "Show version information",
		Usage: "specd version", Synopsis: "specd version",
		LongDescription: "Prints the version of the installed specd binary.",
		Flags: []FlagMeta{}, ExitCodes: []ExitCodeMeta{{0, "Success"}},
		Examples: []string{"specd version"},
	},
	{
		Command: "help", Category: "meta",
		Description: "Show detailed help for a command",
		Usage: "specd help [command]", Synopsis: "specd help [command]",
		LongDescription: "Prints summary documentation for all commands, or detailed reference for a single command.",
		Flags:     []FlagMeta{{Name: "json", Type: "boolean", Description: "Output the entire command registry as JSON"}},
		ExitCodes: []ExitCodeMeta{{0, "Success"}, {2, "Usage error (unknown command)"}},
		Examples:  []string{"specd help", "specd help init", "specd help --json"},
	},
}
