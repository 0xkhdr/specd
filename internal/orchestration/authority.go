package orchestration

import (
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

type Authority struct {
	Enabled bool
}

type GateSeverity string

const (
	GateLow      GateSeverity = "low"
	GateMedium   GateSeverity = "medium"
	GateHigh     GateSeverity = "high"
	GateCritical GateSeverity = "critical"
)

func (authority Authority) CanDispatch() bool {
	return authority.Enabled
}

func (authority Authority) CanClearGate(severity GateSeverity) bool {
	if !authority.Enabled {
		return false
	}
	return severity != GateHigh && severity != GateCritical
}

func DecisionLimitsForAuthority(authority Authority, limits DecisionLimits) DecisionLimits {
	limits.AllowDispatch = authority.CanDispatch()
	return limits
}

type AuthorityDenial struct {
	Time     time.Time `json:"time"`
	WorkerID string    `json:"worker_id"`
	SpecID   string    `json:"spec_id"`
	TaskID   string    `json:"task_id"`
	ToolID   string    `json:"tool_id"`
	Code     string    `json:"code"`
}

func RecordAuthorityDenial(root string, a core.AuthorityV1, tool, code string, now time.Time) error {
	rec := AuthorityDenial{Time: now, WorkerID: a.WorkerID, SpecID: a.SpecID, TaskID: a.TaskID, ToolID: tool, Code: code}
	raw, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return core.AppendFile(filepath.Join(core.SpecdDir(root), "specs", a.SpecID, "authority-denials.jsonl"), string(raw)+"\n")
}
