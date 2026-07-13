package context

import (
	"errors"
	"fmt"
	"strings"
)

const EfficiencySchemaV1 = "context-efficiency/v1"

// TaskEfficiency keeps measured quantities nullable. nil means unknown; a
// pointer to zero means a known zero. This prevents absent telemetry from being
// presented as free or instantaneous.
type TaskEfficiency struct {
	TaskID               string     `json:"task_id"`
	EstimatedInputTokens *int       `json:"estimated_input_tokens,omitempty"`
	ActualInputTokens    *int       `json:"actual_input_tokens,omitempty"`
	OmittedItems         []Omission `json:"omitted_items,omitempty"`
	RetryCount           int        `json:"retry_count"`
	FirstPassResult      string     `json:"first_pass_result"`
	DurationMS           *int       `json:"duration_ms,omitempty"`
	Cost                 *string    `json:"cost,omitempty"`
}

type EfficiencyReport struct {
	SchemaVersion string           `json:"schema_version"`
	SpecID        string           `json:"spec_id"`
	Tasks         []TaskEfficiency `json:"tasks"`
}

func knownInt(v *int) string {
	if v == nil {
		return "unknown"
	}
	return fmt.Sprint(*v)
}
func knownString(v *string) string {
	if v == nil {
		return "unknown"
	}
	return *v
}

// RenderEfficiency emits stable task-order text with every absent measurement
// spelled "unknown". Callers own task ordering; omitted items retain manifest
// order, which is already canonical.
func RenderEfficiency(r EfficiencyReport) (string, error) {
	if r.SchemaVersion != EfficiencySchemaV1 {
		return "", fmt.Errorf("unknown context-efficiency schema version %q", r.SchemaVersion)
	}
	if r.SpecID == "" {
		return "", errors.New("context-efficiency report requires spec_id")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "context_efficiency schema=%s spec=%s\n", r.SchemaVersion, r.SpecID)
	for _, task := range r.Tasks {
		if task.TaskID == "" || task.FirstPassResult == "" {
			return "", errors.New("context-efficiency task requires task_id and first_pass_result")
		}
		omitted := "none"
		if len(task.OmittedItems) > 0 {
			parts := make([]string, 0, len(task.OmittedItems))
			for _, item := range task.OmittedItems {
				parts = append(parts, item.Kind+":"+item.Source+":"+item.Reason)
			}
			omitted = strings.Join(parts, ",")
		}
		fmt.Fprintf(&b, "task=%s estimated_tokens=%s actual_tokens=%s omitted=%s retries=%d first_pass=%s duration_ms=%s cost=%s\n", task.TaskID, knownInt(task.EstimatedInputTokens), knownInt(task.ActualInputTokens), omitted, task.RetryCount, task.FirstPassResult, knownInt(task.DurationMS), knownString(task.Cost))
	}
	return b.String(), nil
}
