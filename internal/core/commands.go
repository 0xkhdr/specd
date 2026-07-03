package core

// Flag describes one command-line flag surfaced by help metadata.
type Flag struct {
	Name        string
	TakesValue  bool
	Description string
}

// Command describes one supported top-level command. This metadata is the
// source of truth for help and integration surfaces.
type Command struct {
	Name        string
	Usage       string
	Description string
	Flags       []Flag
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
		Name:        "verify",
		Usage:       "specd verify <task-id>",
		Description: "Run and record task verification.",
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
