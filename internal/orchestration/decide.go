package orchestration

import (
	"sort"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

type Action string

const (
	ActionWait     Action = "wait"
	ActionDispatch Action = "dispatch"
	ActionHalt     Action = "halt"
	ActionTimeout  Action = "timeout"
	ActionEscalate Action = "escalate"
)

type Decision struct {
	Action Action
	TaskID string
	Reason string
}

type DecisionLimits struct {
	MaxCost       int
	Deadline      time.Time
	MaxRetries    int
	AllowDispatch bool
}

func Decide(snapshot Snapshot, limits DecisionLimits) Decision {
	if brake := EvaluateBrakes(snapshot, limits); brake.Action != "" {
		return brake
	}
	if escalation := Escalation(snapshot.Leases, limits.MaxRetries, snapshot.Now); escalation.TaskID != "" {
		return Decision{Action: ActionEscalate, TaskID: escalation.TaskID, Reason: escalation.Reason}
	}
	if !limits.AllowDispatch || len(snapshot.Frontier) == 0 {
		return Decision{Action: ActionWait, Reason: "no dispatch authority or no frontier"}
	}

	frontier := append([]core.FrontierTask(nil), snapshot.Frontier...)
	sort.SliceStable(frontier, func(i, j int) bool {
		return frontier[i].ID < frontier[j].ID
	})
	return Decision{Action: ActionDispatch, TaskID: frontier[0].ID, Reason: "frontier ready"}
}
