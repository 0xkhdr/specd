package core

import (
	"encoding/json"
	"fmt"
)

// PrintJSON marshals v with two-space indentation and writes it to stdout
// followed by a newline. It is the single JSON-emission path for every command:
// machine-readable results always go to stdout (diagnostics go to stderr via
// Error/Warn). Callers are responsible for ensuring list fields are non-nil so
// the agent never has to parse both `null` and `[]` — see the package
// convention in docs/command-reference.md.
func PrintJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
