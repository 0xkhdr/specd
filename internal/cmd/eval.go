package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/0xkhdr/specd/internal/core"
)

type evalStatusReport struct {
	SchemaVersion string                    `json:"schema_version"`
	Count         int                       `json:"count"`
	Records       []core.EvidenceEnvelopeV1 `json:"records"`
}

func runEval(root string, args []string, flags map[string]string) error {
	if len(args) < 2 {
		return usageError("eval")
	}
	switch args[0] {
	case "import":
		if len(args) != 3 {
			return fmt.Errorf("%w: specd eval import <spec> <artifact>", ErrUsage)
		}
		slug, path := args[1], args[2]
		if filepath.IsAbs(path) {
			recovery := "specd eval import " + slug + " <workspace-relative-file>"
			if task := flags["task"]; task != "" {
				recovery += " --task " + task
			}
			if check := flags["check"]; check != "" {
				recovery += " --check " + check
			}
			return core.Refusef("ARTIFACT_PATH_ABSOLUTE", "eval artifact path %q is absolute; a workspace-relative artifact path is required", path).
				WithContext("eval artifact", path, "workspace-relative artifact path").
				WithRecovery(core.RefusalActorAgent, recovery).
				Wrapping(ErrUsage)
		}
		resolved, err := core.SafeJoin(root, path)
		if err != nil {
			return fmt.Errorf("%w: artifact path: %v", ErrUsage, err)
		}
		raw, err := os.ReadFile(resolved)
		if err != nil {
			return fmt.Errorf("%w: read artifact: %v", ErrUsage, err)
		}
		expect := core.ImportExpect{SpecSlug: slug, TaskID: flags["task"]}
		if check := flags["check"]; check != "" {
			expect.CheckIDs = []string{check}
		}
		findings, err := core.ImportEvalsToStore(core.EvalStorePath(root, slug), raw, expect)
		if err != nil {
			return err
		}
		if len(findings) > 0 {
			return fmt.Errorf("%w: %s", ErrUsage, findings[0].Code+": "+findings[0].Message)
		}
		fmt.Fprintf(os.Stdout, "imported eval evidence for %s\n", slug)
		return nil
	case "status":
		if len(args) != 2 {
			return fmt.Errorf("%w: specd eval status <spec>", ErrUsage)
		}
		records, err := core.LoadEvals(core.EvalStorePath(root, args[1]))
		if errors.Is(err, os.ErrNotExist) {
			records = nil
		} else if err != nil {
			return err
		}
		report := evalStatusReport{SchemaVersion: core.EvalSchemaVersion, Count: len(records), Records: records}
		if flagEnabled(flags, "json") {
			return writeJSON(report)
		}
		fmt.Fprintf(os.Stdout, "%s eval records: %d\n", args[1], len(records))
		return nil
	default:
		return fmt.Errorf("%w: unknown eval action %q", ErrUsage, args[0])
	}
}
