package integration

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Scope is the configuration boundary an adapter can manage.
type Scope string

const (
	ScopeProject Scope = "project"
	ScopeGlobal  Scope = "global"
)

// Confidence describes how strongly detection evidence identifies a host.
type Confidence string

const (
	ConfidenceNone   Confidence = "none"
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

// Detection is evidence gathered without executing or modifying the host.
type Detection struct {
	Host          string     `json:"host"`
	Detected      bool       `json:"detected"`
	Executable    string     `json:"executable"`
	ProjectConfig string     `json:"projectConfig"`
	Scopes        []Scope    `json:"scopes"`
	Method        string     `json:"method"`
	Confidence    Confidence `json:"confidence"`
	Reason        string     `json:"reason"`
}

// HostAction is one deterministic install step. Command and Args are separate
// so adapters never need shell concatenation.
type HostAction struct {
	Kind        string   `json:"kind"`
	Target      string   `json:"target"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	Description string   `json:"description"`
}

// HostPlan describes a proposed host mutation.
type HostPlan struct {
	Host     string       `json:"host"`
	Root     string       `json:"root"`
	Scope    Scope        `json:"scope"`
	Actions  []HostAction `json:"actions"`
	Warnings []string     `json:"warnings"`
}

// HostResult describes the outcome of applying a HostPlan.
type HostResult struct {
	Host       string   `json:"host"`
	Status     string   `json:"status"`
	Changed    bool     `json:"changed"`
	Targets    []string `json:"targets"`
	Backups    []string `json:"backups"`
	Warnings   []string `json:"warnings"`
	NextAction string   `json:"nextAction"`
}

// HostState is the inspectable registration state of an adapter.
type HostState struct {
	Host        string `json:"host"`
	Scope       Scope  `json:"scope"`
	Registered  bool   `json:"registered"`
	Owned       bool   `json:"owned"`
	Target      string `json:"target"`
	Fingerprint string `json:"fingerprint"`
	Reason      string `json:"reason"`
}

// Verification is the deepest deterministic verification available for a host.
type Verification struct {
	Host   string `json:"host"`
	Status string `json:"status"`
	Reason string `json:"reason"`
	Remedy string `json:"remedy"`
}

// HostAdapter isolates host-specific schemas and commands from init orchestration.
type HostAdapter interface {
	Name() string
	Scopes() []Scope
	Detect(root string) Detection
	Plan(root string, scope Scope) (HostPlan, error)
	Install(ctx context.Context, plan HostPlan) (HostResult, error)
	Inspect(root string, scope Scope) (HostState, error)
	Verify(root string) Verification
}

func normalizeDetection(d Detection) Detection {
	if d.Scopes == nil {
		d.Scopes = []Scope{}
	}
	sort.Slice(d.Scopes, func(i, j int) bool { return d.Scopes[i] < d.Scopes[j] })
	return d
}

func normalizePlan(plan HostPlan) HostPlan {
	if plan.Actions == nil {
		plan.Actions = []HostAction{}
	}
	if plan.Warnings == nil {
		plan.Warnings = []string{}
	}
	for i := range plan.Actions {
		if plan.Actions[i].Args == nil {
			plan.Actions[i].Args = []string{}
		}
	}
	sort.SliceStable(plan.Actions, func(i, j int) bool {
		if plan.Actions[i].Kind != plan.Actions[j].Kind {
			return plan.Actions[i].Kind < plan.Actions[j].Kind
		}
		if plan.Actions[i].Target != plan.Actions[j].Target {
			return plan.Actions[i].Target < plan.Actions[j].Target
		}
		if plan.Actions[i].Command != plan.Actions[j].Command {
			return plan.Actions[i].Command < plan.Actions[j].Command
		}
		return strings.Join(plan.Actions[i].Args, "\x00") < strings.Join(plan.Actions[j].Args, "\x00")
	})
	sort.Strings(plan.Warnings)
	return plan
}

func validatePlan(adapter HostAdapter, root string, scope Scope, plan HostPlan) error {
	if strings.ContainsRune(root, '\x00') {
		return fmt.Errorf("project root contains NUL")
	}
	if plan.Host != adapter.Name() {
		return fmt.Errorf("adapter %q returned plan for host %q", adapter.Name(), plan.Host)
	}
	if plan.Root != root {
		return fmt.Errorf("adapter %q returned plan for root %q, want %q", adapter.Name(), plan.Root, root)
	}
	if plan.Scope != scope {
		return fmt.Errorf("adapter %q returned plan for scope %q, want %q", adapter.Name(), plan.Scope, scope)
	}
	return nil
}
