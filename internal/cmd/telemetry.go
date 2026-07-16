package cmd

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/core"
)

// aggregateTelemetry folds every evidence attempt's annotations into a per-spec
// and per-task report, ordered by the spec's task list so a task that reported
// no telemetry is shown as absent rather than imputed (spec 10 R4).
func aggregateTelemetry(root, slug string, model core.ReportModel) (core.TelemetryReport, error) {
	records, err := core.LoadEvidenceRecords(core.EvidencePath(root, slug))
	if err != nil {
		return core.TelemetryReport{}, err
	}
	order := make([]string, 0, len(model.Tasks))
	for _, task := range model.Tasks {
		order = append(order, task.ID)
	}
	return core.AggregateTelemetry(records, order), nil
}

// parseAnnotations reads the optional --tokens/--cost/--duration-ms flags shared
// by `verify` and `complete-task`. It returns nil when none are supplied and a
// fail-closed usage error (exit 2) on any malformed value (spec 10 R2). specd
// stores these verbatim and never computes them.
func parseAnnotations(flags map[string]string) (*core.Annotations, error) {
	present := func(name string) bool { _, ok := flags[name]; return ok }
	ann, err := core.ParseAnnotationFlags(flags, present)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUsage, err)
	}
	return ann, nil
}
