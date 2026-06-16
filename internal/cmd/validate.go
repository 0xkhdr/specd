package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// RunValidate checks a spec's on-disk state.json against the embedded JSON
// Schema. It is a *format* conformance mode — structural shape, required keys,
// closed property sets, enums — explicitly independent of the seven semantic
// gates (`specd check`). The `--schema` flag is required to select this mode so
// the command can grow other validation modes later without changing defaults.
// Read-only: it never writes state. `--version` selects a schema version.
func RunValidate(args cli.Args) int {
	if !args.Bool("schema") {
		return usageExit("usage: specd validate <slug> --schema [--version <v>]")
	}
	root, slug, code, ok := requireRootAndSlug(args, "usage: specd validate <slug> --schema [--version <v>]")
	if !ok {
		return code
	}
	if err := core.RequireSpec(root, slug); err != nil {
		return specdExit(err)
	}

	raw := core.ReadOrNull(filepath.Join(core.SpecDir(root, slug), "state.json"))
	if raw == nil {
		return specdExit(core.NotFoundError(fmt.Sprintf("no state.json for spec '%s'", slug)))
	}

	viols, err := core.ValidateState([]byte(*raw), args.Str("version"))
	if err != nil {
		return specdExit(err)
	}

	if core.IsJSONMode() {
		if viols == nil {
			viols = []string{}
		}
		if err := core.PrintJSON(struct {
			Spec       string   `json:"spec"`
			Schema     string   `json:"schema"`
			Conformant bool     `json:"conformant"`
			Violations []string `json:"violations"`
		}{slug, core.SchemaVersionID, len(viols) == 0, viols}); err != nil {
			return specdExit(err)
		}
		if len(viols) > 0 {
			return core.ExitGate
		}
		return core.ExitOK
	}

	if len(viols) == 0 {
		core.Info(fmt.Sprintf("schema: %s conforms to open spec format v%s", slug, core.SchemaVersionID))
		return core.ExitOK
	}
	core.Error(fmt.Sprintf("schema: %s has %d conformance violation(s):", slug, len(viols)))
	for _, v := range viols {
		core.Error("  • " + v)
	}
	return core.ExitGate
}
