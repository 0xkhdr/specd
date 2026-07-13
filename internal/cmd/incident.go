package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

func runIncident(root string, args []string, flags map[string]string) error {
	if len(args) != 2 || args[0] != "seed" {
		return errors.New("usage: specd incident seed <new-spec> --source-spec <spec> --release <id> --deployment <id> --criterion <id> --evidence-ref <ref[,ref]>")
	}
	slug := args[1]
	refs := strings.Split(flags["evidence-ref"], ",")
	seed := core.IncidentSeed{SourceSpec: flags["source-spec"], ReleaseID: flags["release"], DeploymentID: flags["deployment"], CriterionID: flags["criterion"], EvidenceRefs: refs}
	sourceState, err := core.LoadState(core.StatePath(root, seed.SourceSpec))
	if err != nil {
		return fmt.Errorf("incident source %q does not exist: %w", seed.SourceSpec, err)
	}
	if sourceState.Status != core.StatusComplete {
		return fmt.Errorf("incident source %q must be complete", seed.SourceSpec)
	}
	plan, err := core.PlanIncidentSuccessor(slug, seed)
	if err != nil {
		return err
	}
	requirements, design, tasks, memory, err := core.IncidentSpecDocuments(slug, seed)
	if err != nil {
		return err
	}
	dir := filepath.Join(core.SpecdDir(root), "specs", slug)
	_, err = core.WithSpecLock(root, func() (struct{}, error) {
		if _, statErr := os.Stat(dir); statErr == nil {
			return struct{}{}, fmt.Errorf("spec %q already exists", slug)
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return struct{}{}, statErr
		}
		program, err := core.LoadProgram(core.ProgramPath(root))
		if err != nil {
			return struct{}{}, err
		}
		if cycle := program.WouldCycle(plan.Link.From, plan.Link.To); len(cycle) > 0 {
			return struct{}{}, fmt.Errorf("incident successor link would create cycle: %s", strings.Join(cycle, " -> "))
		}
		if err := program.AddTypedLink(plan.Link.From, plan.Link.To, plan.Link.Kind, plan.Link.Reason); err != nil {
			return struct{}{}, err
		}
		provenance, err := json.MarshalIndent(plan.Provenance, "", "  ")
		if err != nil {
			return struct{}{}, err
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return struct{}{}, err
		}
		committed := false
		defer func() {
			if !committed {
				_ = os.RemoveAll(dir)
			}
		}()
		for name, content := range map[string]string{"requirements.md": requirements, "design.md": design, "tasks.md": tasks, "memory.md": memory} {
			if err := core.AtomicWrite(filepath.Join(dir, name), content); err != nil {
				return struct{}{}, err
			}
		}
		if err := core.AtomicWrite(core.ProvenancePath(root, slug), string(provenance)+"\n"); err != nil {
			return struct{}{}, err
		}
		if err := core.SaveState(core.StatePath(root, slug), core.InitialState(slug)); err != nil {
			return struct{}{}, err
		}
		if err := core.SaveProgram(core.ProgramPath(root), program); err != nil {
			return struct{}{}, err
		}
		committed = true
		return struct{}{}, nil
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "seeded incident spec %s from %s/%s\n", slug, seed.ReleaseID, seed.DeploymentID)
	return nil
}
