package mcp

import (
	"fmt"
	"sort"

	"github.com/0xkhdr/specd/internal/core"
)

// commandFlagsByName indexes core.Commands by Command name so
// validateToolArgs can look up a tool's declared flag shape in O(1).
// core.Commands is a package-level var fixed at process start, so this index
// is built once and never invalidated.
var commandFlagsByName = func() map[string][]core.FlagMeta {
	m := make(map[string][]core.FlagMeta, len(core.Commands))
	for _, c := range core.Commands {
		m[c.Command] = c.Flags
	}
	return m
}()

// validateToolArgs rejects a raw-passthrough tool call whose arguments map
// doesn't match command's declared shape: an undeclared key, or a value whose
// shape disagrees with the declared flag kind. Known tool/flag names come
// from the same core.Commands data commandToTool already derives InputSchema
// from (internal/mcp/tools.go), so the dispatch-time gate and the advertised
// schema cannot drift apart. Commands with no core.Commands entry (e.g.
// unknown tool names) are left to the existing unknown-tool rejection in
// callTool/runTool.
//
// CommandMeta's flag vocabulary has only two kinds: "boolean" and everything
// else ("string" in the struct literal, but covering any CLI flag value —
// e.g. max-retries is declared "string" yet legitimately receives a JSON
// number such as 2, because buildArgv stringifies it with fmt.Sprint before
// it reaches argv). So non-boolean flags are validated as "scalar" rather
// than strictly "string": arrays and objects are rejected (they would
// mangle into garbage like "[1 2]" via fmt.Sprint — the real "array where a
// scalar is expected" case from Requirement 1.2), and so is a bare bool
// (buildArgv's bool branch ignores the flag's declared kind entirely and
// emits a value-less flag, silently dropping the intended value).
func validateToolArgs(command string, arguments map[string]any) error {
	flags, ok := commandFlagsByName[command]
	if !ok {
		return nil
	}
	boolFlags := make(map[string]bool, len(flags))
	known := make(map[string]bool, len(flags)+1)
	known["args"] = true
	for _, f := range flags {
		known[f.Name] = true
		boolFlags[f.Name] = f.Type == "boolean"
	}

	keys := make([]string, 0, len(arguments))
	for k := range arguments {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := arguments[k]
		if v == nil {
			continue // omitted flag, same as buildArgv's nil case
		}
		if k == "args" {
			if _, ok := v.([]any); !ok {
				return fmt.Errorf("'args' must be an array of strings")
			}
			continue
		}
		if !known[k] {
			return fmt.Errorf("unknown argument %q for tool %q", k, toolPrefix+command)
		}
		if boolFlags[k] {
			if _, ok := v.(bool); !ok {
				return fmt.Errorf("argument %q must be a boolean", k)
			}
			continue
		}
		switch v.(type) {
		case []any:
			return fmt.Errorf("argument %q must be a scalar, not an array", k)
		case map[string]any:
			return fmt.Errorf("argument %q must be a scalar, not an object", k)
		case bool:
			return fmt.Errorf("argument %q must not be a boolean", k)
		}
	}
	return nil
}
