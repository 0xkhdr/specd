package core

// Flag describes one command-line flag surfaced by help metadata.
type Flag struct {
	Name        string `json:"name"`
	TakesValue  bool   `json:"takes_value,omitempty"`
	Description string `json:"description,omitempty"`
}

// Command describes one supported top-level command. This metadata is the
// source of truth for help and integration surfaces.
type Command struct {
	Name        string `json:"name"`
	Usage       string `json:"usage"`
	Description string `json:"description"`
	Flags       []Flag `json:"flags,omitempty"`
	// Deferred marks a registered verb whose implementation is intentionally
	// not wired yet. The dispatcher reports the deferral and exits 0; the
	// handler-parity test treats a deferred verb as satisfied.
	Deferred bool `json:"deferred,omitempty"`
}

// Commands is the stable top-level command palette.
var Commands = []Command{
	{
		Name:        "help",
		Usage:       "specd help [command] [--json]",
		Description: "Show command help.",
		Flags: []Flag{
			{Name: "json", Description: "Emit machine-readable help."},
		},
	},
	{
		Name:        "init",
		Usage:       "specd init [--agent=<name>]",
		Description: "Initialize specd project state.",
		Flags: []Flag{
			{Name: "agent", TakesValue: true, Description: "Select agent harness."},
		},
	},
	{
		Name:        "new",
		Usage:       "specd new <name> [--agent=<name>]",
		Description: "Create a new spec workspace.",
		Flags: []Flag{
			{Name: "agent", TakesValue: true, Description: "Select agent harness."},
		},
	},
	{
		Name:        "approve",
		Usage:       "specd approve <spec> <gate>",
		Description: "Record human approval for a lifecycle gate.",
	},
	{
		Name:        "midreq",
		Usage:       "specd midreq <spec>",
		Description: "Capture a scoped mid-stream requirement change.",
	},
	{
		Name:        "decision",
		Usage:       "specd decision <spec>",
		Description: "Record an explicit human decision.",
	},
	{
		Name:        "next",
		Usage:       "specd next [--waves|--dispatch|--json]",
		Description: "Select the next eligible task or wave.",
		Flags: []Flag{
			{Name: "waves", Description: "Show wave groups."},
			{Name: "dispatch", Description: "Emit dispatch-ready task data."},
			{Name: "json", Description: "Emit machine-readable output."},
		},
	},
	{
		Name:        "status",
		Usage:       "specd status [spec] [--json]",
		Description: "Report current spec and task state.",
		Flags: []Flag{
			{Name: "json", Description: "Emit machine-readable status."},
		},
	},
	{
		Name:        "task",
		Usage:       "specd task <id>",
		Description: "Show task details.",
	},
	{
		Name:        "check",
		Usage:       "specd check <spec> [--security] [--json]",
		Description: "Run the validation gate registry against a spec.",
		Flags: []Flag{
			{Name: "security", Description: "Include the opt-in security gate."},
			{Name: "json", Description: "Emit machine-readable findings."},
		},
	},
	{
		Name:        "verify",
		Usage:       "specd verify <task-id>",
		Description: "Run and record task verification.",
	},
	{
		Name:        "context",
		Usage:       "specd context <slug> <task-id> [--json|--hud]",
		Description: "Build the bounded context manifest for a task.",
		Flags: []Flag{
			{Name: "json", Description: "Emit machine-readable context."},
			{Name: "hud", Description: "Render the operator HUD (files, bytes, tokens, mode)."},
		},
	},
	{
		Name:        "memory",
		Usage:       "specd memory <slug> <add|promote> [flags]",
		Description: "Append or promote steering-memory patterns (learning flywheel).",
		Flags: []Flag{
			{Name: "key", TakesValue: true, Description: "Pattern key (H2 heading)."},
			{Name: "pattern", TakesValue: true, Description: "One-line pattern statement (add)."},
			{Name: "body", TakesValue: true, Description: "Detail of the pattern (add)."},
			{Name: "source", TakesValue: true, Description: "Where the pattern came from (add)."},
			{Name: "criticality", TakesValue: true, Description: "minor|important|critical (add)."},
			{Name: "related", TakesValue: true, Description: "Comma-separated related keys → wikilinks (add)."},
			{Name: "force", Description: "Promote past the threshold (promote)."},
		},
	},
	{
		Name:        "mcp",
		Usage:       "specd mcp",
		Description: "Serve the MCP integration surface over stdio.",
	},
	{
		Name:        "handshake",
		Usage:       "specd handshake [bootstrap|policy]",
		Description: "Emit bootstrap or policy handshake material.",
	},
	{
		Name:        "brain",
		Usage:       "specd brain <start|step|run|status> <spec> [--authority]",
		Description: "Run the opt-in deterministic orchestration controller.",
		Flags: []Flag{
			{Name: "authority", Description: "Grant dispatch authority (fail-closed by default)."},
		},
	},
	{
		Name:        "report",
		Usage:       "specd report <spec> [--pr|--metrics|--json]",
		Description: "Render evidence-backed status and PR reports.",
		Flags: []Flag{
			{Name: "pr", Description: "Emit PR-oriented report."},
			{Name: "metrics", Description: "Emit metrics summary."},
			{Name: "json", Description: "Emit machine-readable report."},
		},
	},
	{
		Name:        "triage",
		Usage:       "specd triage <spec>",
		Description: "Run the opt-in extended-loop triage tier.",
		Deferred:    true,
	},
}

// CommandNames returns command names in help order.
func CommandNames() []string {
	names := make([]string, len(Commands))
	for i, command := range Commands {
		names[i] = command.Name
	}
	return names
}
