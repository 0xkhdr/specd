package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/core/gates"
)

// runNew creates a spec workspace: spec.md, tasks.md, and state.json at
// revision 0 (R13.3). Creation is a fresh write under the per-spec lock, not a
// compare-and-swap; SaveStateCAS with expected revision 0 would ratchet to 1.
func runNew(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: specd new <name>")
	}
	slug := args[0]
	if err := core.ValidateSlug(slug); err != nil {
		return err
	}
	specDir := filepath.Join(core.SpecdDir(root), "specs", slug)
	statePath := core.StatePath(root, slug)
	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		if _, err := os.Stat(statePath); err == nil {
			return struct{}{}, fmt.Errorf("spec %q already exists", slug)
		}
		if err := os.MkdirAll(specDir, 0o755); err != nil {
			return struct{}{}, err
		}
		if err := core.AtomicWrite(filepath.Join(specDir, "spec.md"), specStub(slug)); err != nil {
			return struct{}{}, err
		}
		if err := core.AtomicWrite(filepath.Join(specDir, "tasks.md"), tasksStub(slug)); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, core.SaveState(statePath, core.InitialState(slug))
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "created spec %s at %s\n", slug, specDir)
	return nil
}

// runApprove refuses the gate transition when readiness gates emit errors and
// leaves state untouched; on green it ratchets the phase and appends an
// approval record via CAS (R13.4).
func runApprove(root string, args []string, flags map[string]string) error {
	if len(args) != 2 {
		return errors.New("usage: specd approve <spec> <gate>")
	}
	slug, gate := args[0], args[1]
	target := core.Status(gate)
	if !core.ValidStatus(target) {
		return fmt.Errorf("invalid gate %q", gate)
	}
	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		statePath := core.StatePath(root, slug)
		state, err := core.LoadState(statePath)
		if err != nil {
			return struct{}{}, err
		}
		spec, err := loadSpec(root, slug)
		if err != nil {
			return struct{}{}, err
		}
		findings := gates.CoreRegistry().Run(gates.CheckCtx{
			Root:             root,
			Slug:             slug,
			Tasks:            spec.Tasks,
			Status:           taskStatus(spec.Tasks),
			Evidence:         spec.Evidence,
			MaxContextTokens: contextBudget(root),
		})
		if gates.HasErrors(findings) {
			for _, finding := range findings {
				if finding.Severity == gates.Error {
					fmt.Fprintf(os.Stderr, "%s %s: %s\n", finding.Severity, finding.Gate, finding.Message)
				}
			}
			return struct{}{}, errors.New("approve refused: readiness gates failing")
		}
		phase, err := core.AdvanceStatus(state.Status, target)
		if err != nil {
			return struct{}{}, err
		}
		state.Status = target
		state.Phase = phase
		if err := appendRecord(&state, "approval:"+gate, map[string]string{"gate": gate}); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, core.SaveStateCAS(statePath, state.Revision, state)
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "approved %s → %s\n", slug, gate)
	return nil
}

func runMidreq(root string, args []string, flags map[string]string) error {
	return appendScoped(root, args, "midreq", "usage: specd midreq <spec>")
}

func runDecision(root string, args []string, flags map[string]string) error {
	return appendScoped(root, args, "decision", "usage: specd decision <spec>")
}

// appendScoped appends a scoped record to state via CAS without touching
// unrelated core fields (R13.5).
func appendScoped(root string, args []string, kind, usage string) error {
	if len(args) != 1 {
		return errors.New(usage)
	}
	slug := args[0]
	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		statePath := core.StatePath(root, slug)
		state, err := core.LoadState(statePath)
		if err != nil {
			return struct{}{}, err
		}
		key := fmt.Sprintf("%s:%d", kind, countPrefix(state.Records, kind+":"))
		if err := appendRecord(&state, key, map[string]string{"kind": kind}); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, core.SaveStateCAS(statePath, state.Revision, state)
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "recorded %s for %s\n", kind, slug)
	return nil
}

// runHelp renders usage from core.Commands metadata; --json is machine-readable
// (R13.9).
func runHelp(root string, args []string, flags map[string]string) error {
	if len(args) > 1 {
		return errors.New("usage: specd help [command] [--json]")
	}
	if len(args) == 1 {
		command, ok := findCommand(args[0])
		if !ok {
			return fmt.Errorf("unknown command %q", args[0])
		}
		if flagEnabled(flags, "json") {
			return writeJSON(command)
		}
		fmt.Fprintf(os.Stdout, "%s\n  %s\n", command.Usage, command.Description)
		for _, flag := range command.Flags {
			fmt.Fprintf(os.Stdout, "  --%s  %s\n", flag.Name, flag.Description)
		}
		return nil
	}
	if flagEnabled(flags, "json") {
		return writeJSON(core.Commands)
	}
	fmt.Fprintln(os.Stdout, "usage: specd <command> [args] [--flag value|--flag=value]")
	for _, command := range core.Commands {
		fmt.Fprintf(os.Stdout, "  %-10s %s\n", command.Name, command.Description)
	}
	return nil
}

// runTask prints the parsed task row matching id across the project's specs
// (R13.9).
func runTask(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: specd task <id>")
	}
	id := args[0]
	entries, err := os.ReadDir(filepath.Join(core.SpecdDir(root), "specs"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		spec, err := loadSpec(root, entry.Name())
		if err != nil {
			continue
		}
		for _, task := range spec.Tasks {
			if task.ID == id {
				if flagEnabled(flags, "json") {
					return writeJSON(task)
				}
				fmt.Fprintf(os.Stdout, "%s [%s] %s\n", task.ID, entry.Name(), task.Role)
				fmt.Fprintf(os.Stdout, "  files:      %s\n", task.Files)
				fmt.Fprintf(os.Stdout, "  depends-on: %s\n", strings.Join(task.DependsOn, ", "))
				fmt.Fprintf(os.Stdout, "  verify:     %s\n", task.Verify)
				fmt.Fprintf(os.Stdout, "  acceptance: %s\n", task.Acceptance)
				return nil
			}
		}
	}
	return fmt.Errorf("task %s not found", id)
}

func appendRecord(state *core.State, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if state.Records == nil {
		state.Records = map[string]json.RawMessage{}
	}
	state.Records[key] = raw
	return nil
}

func countPrefix(records map[string]json.RawMessage, prefix string) int {
	count := 0
	for key := range records {
		if strings.HasPrefix(key, prefix) {
			count++
		}
	}
	return count
}

func findCommand(name string) (core.Command, bool) {
	for _, command := range core.Commands {
		if command.Name == name {
			return command, true
		}
	}
	return core.Command{}, false
}

func specStub(slug string) string {
	return fmt.Sprintf("# Spec — %s\n\n> Scaffolded by `specd new`. Replace with requirements and design.\n", slug)
}

func tasksStub(slug string) string {
	return fmt.Sprintf(`# Tasks — %s

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T1 | craftsman | spec.md | - | go test ./... | scaffolded task placeholder |
`, slug)
}
