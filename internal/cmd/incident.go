package cmd

import (
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
	requirements, design, tasks, memory, err := core.IncidentSpecDocuments(slug, seed)
	if err != nil {
		return err
	}
	dir := filepath.Join(core.SpecdDir(root), "specs", slug)
	_, err = core.WithSpecLock(root, func() (struct{}, error) {
		if _, statErr := os.Stat(core.StatePath(root, slug)); statErr == nil {
			return struct{}{}, fmt.Errorf("spec %q already exists", slug)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return struct{}{}, err
		}
		for name, content := range map[string]string{"requirements.md": requirements, "design.md": design, "tasks.md": tasks, "memory.md": memory} {
			if err := core.AtomicWrite(filepath.Join(dir, name), content); err != nil {
				return struct{}{}, err
			}
		}
		return struct{}{}, core.SaveState(core.StatePath(root, slug), core.InitialState(slug))
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "seeded incident spec %s from %s/%s\n", slug, seed.ReleaseID, seed.DeploymentID)
	return nil
}
