package core

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func RenderHelp() string {
	categories := []struct {
		key   string
		label string
	}{
		{"lifecycle", "LIFECYCLE"},
		{"execution", "EXECUTION"},
		{"inspection", "INSPECTION"},
		{"meta", "META"},
	}
	var b strings.Builder
	fmt.Fprintf(&b, "specd — spec-driven coding harness %s\n\n", Version)
	for _, cat := range categories {
		fmt.Fprintf(&b, "%s\n", cat.label)
		for _, c := range Commands {
			if c.Category == cat.key {
				bare := strings.TrimPrefix(c.Usage, "specd ")
				line := "  " + bare
				for len(line) < 32 {
					line += " "
				}
				fmt.Fprintf(&b, "%s%s\n", line, c.Description)
			}
		}
		b.WriteByte('\n')
	}
	b.WriteString(`Use "specd help <command>" for detailed usage of a command.`)
	b.WriteByte('\n')
	return b.String()
}

func RenderCommandHelp(cmdName string) (string, error) {
	var c *CommandMeta
	for i, cmd := range Commands {
		if cmd.Command == cmdName {
			c = &Commands[i]
			break
		}
	}
	if c == nil {
		return "", fmt.Errorf("unknown command: %s", cmdName)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "NAME\n  specd %s — %s\n\n", c.Command, c.Description)
	fmt.Fprintf(&b, "SYNOPSIS\n  %s\n\n", c.Usage)
	fmt.Fprintf(&b, "DESCRIPTION\n  %s\n\n", c.LongDescription)
	if len(c.Flags) > 0 {
		fmt.Fprintf(&b, "FLAGS\n")
		for _, f := range c.Flags {
			typeStr := ""
			if f.Type == "string" {
				typeStr = " <val>"
			}
			fmt.Fprintf(&b, "  --%s%s    %s\n", f.Name, typeStr, f.Description)
		}
		b.WriteByte('\n')
	}
	if len(c.ExitCodes) > 0 {
		fmt.Fprintf(&b, "EXIT CODES\n")
		for _, e := range c.ExitCodes {
			fmt.Fprintf(&b, "  %d  %s\n", e.Code, e.Meaning)
		}
		b.WriteByte('\n')
	}
	if len(c.Examples) > 0 {
		fmt.Fprintf(&b, "EXAMPLE\n")
		for _, ex := range c.Examples {
			fmt.Fprintf(&b, "  %s\n", ex)
		}
	}
	return b.String(), nil
}

func RenderHelpJSON() (string, error) {
	b, err := json.MarshalIndent(Commands, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
