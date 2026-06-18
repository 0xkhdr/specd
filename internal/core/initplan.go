package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const InitResultSchemaVersion = 1

type InitOptions struct {
	Root           string   `json:"root"`
	Force          bool     `json:"force"`
	Repair         bool     `json:"repair"`
	Refresh        bool     `json:"refresh"`
	DryRun         bool     `json:"dryRun"`
	Interactive    bool     `json:"interactive"`
	AgentSelection []string `json:"agentSelection"`
	Scope          string   `json:"scope"`
}

type InitAction struct {
	Kind        string `json:"kind"`
	Target      string `json:"target"`
	Description string `json:"description"`
	Destructive bool   `json:"destructive"`
	Required    bool   `json:"required"`
	Template    string `json:"template"`
	Content     string `json:"-"`
}

type InitPlan struct {
	Root     string        `json:"root"`
	Actions  []InitAction  `json:"actions"`
	Warnings []InitWarning `json:"warnings"`
}

type InitFileResults struct {
	Written []string `json:"written"`
	Updated []string `json:"updated"`
	Skipped []string `json:"skipped"`
	Failed  []string `json:"failed"`
}

type InitAgentResults struct {
	Detected   []string `json:"detected"`
	Configured []string `json:"configured"`
	Manual     []string `json:"manual"`
}

type InitVerificationResult struct {
	MCP             string `json:"mcp"`
	ProtocolVersion string `json:"protocolVersion"`
	ToolCount       int    `json:"toolCount"`
}

type InitWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type InitNextAction struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type InitResult struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Status        string                 `json:"status"`
	Root          string                 `json:"root"`
	Mode          string                 `json:"mode"`
	Files         InitFileResults        `json:"files"`
	Agents        InitAgentResults       `json:"agents"`
	Verification  InitVerificationResult `json:"verification"`
	Warnings      []InitWarning          `json:"warnings"`
	NextAction    InitNextAction         `json:"nextAction"`
}

func NewInitResult(root string) InitResult {
	result := InitResult{
		SchemaVersion: InitResultSchemaVersion,
		Status:        "ready",
		Root:          root,
		Mode:          "init",
		Files: InitFileResults{
			Written: []string{},
			Updated: []string{},
			Skipped: []string{},
			Failed:  []string{},
		},
		Agents: InitAgentResults{
			Detected:   []string{},
			Configured: []string{},
			Manual:     []string{},
		},
		Verification: InitVerificationResult{MCP: "not-run"},
		Warnings:     []InitWarning{},
		NextAction: InitNextAction{
			Kind: "agent-prompt",
			Text: "Read specd context and help me create a spec for <feature>.",
		},
	}
	return result
}

func (r *InitResult) Normalize() {
	if r.Files.Written == nil {
		r.Files.Written = []string{}
	}
	if r.Files.Updated == nil {
		r.Files.Updated = []string{}
	}
	if r.Files.Skipped == nil {
		r.Files.Skipped = []string{}
	}
	if r.Files.Failed == nil {
		r.Files.Failed = []string{}
	}
	if r.Agents.Detected == nil {
		r.Agents.Detected = []string{}
	}
	if r.Agents.Configured == nil {
		r.Agents.Configured = []string{}
	}
	if r.Agents.Manual == nil {
		r.Agents.Manual = []string{}
	}
	if r.Warnings == nil {
		r.Warnings = []InitWarning{}
	}
	sort.Strings(r.Files.Written)
	sort.Strings(r.Files.Updated)
	sort.Strings(r.Files.Skipped)
	sort.Strings(r.Files.Failed)
	sort.Strings(r.Agents.Detected)
	sort.Strings(r.Agents.Configured)
	sort.Strings(r.Agents.Manual)
	sort.Slice(r.Warnings, func(i, j int) bool {
		if r.Warnings[i].Code == r.Warnings[j].Code {
			return r.Warnings[i].Message < r.Warnings[j].Message
		}
		return r.Warnings[i].Code < r.Warnings[j].Code
	})
}

type InitExecutor struct {
	WriteFile   func(path, content string) error
	MergeAgents func(path, content string, force bool) error
}

func DefaultInitExecutor() InitExecutor {
	return InitExecutor{
		WriteFile:   AtomicWrite,
		MergeAgents: MergeAgentsMD,
	}
}

// PlanInit validates and resolves every template before returning actions. It
// reads project state but performs no writes.
func PlanInit(options InitOptions, assets []ScaffoldAsset, readTemplate func(string) (string, error)) (InitPlan, error) {
	if options.Root == "" {
		return InitPlan{}, fmt.Errorf("init root is required")
	}
	if err := ValidateScaffoldManifest(assets, readTemplate); err != nil {
		return InitPlan{}, err
	}
	plan := InitPlan{
		Root:     options.Root,
		Actions:  make([]InitAction, 0, len(assets)),
		Warnings: []InitWarning{},
	}
	for _, asset := range assets {
		content, err := readTemplate(asset.Template)
		if err != nil {
			return InitPlan{}, fmt.Errorf("read template %s: %w", asset.Template, err)
		}
		target := filepath.Join(options.Root, filepath.FromSlash(asset.Target))
		action := InitAction{
			Target:   target,
			Required: asset.Required,
			Template: asset.Template,
			Content:  content,
		}
		switch asset.Policy {
		case ScaffoldMarkerMerge:
			action.Kind = "merge"
			action.Description = "merge managed marker section"
			action.Destructive = options.Force
		case ScaffoldCreate:
			if _, err := os.Stat(target); err == nil && !options.Force {
				action.Kind = "skip"
				action.Description = "preserve existing file"
			} else if err != nil && !os.IsNotExist(err) {
				return InitPlan{}, fmt.Errorf("inspect %s: %w", target, err)
			} else {
				action.Kind = "write"
				action.Description = "write embedded scaffold asset"
				action.Destructive = options.Force
			}
		}
		plan.Actions = append(plan.Actions, action)
	}
	return plan, nil
}

func ExecuteInitPlan(plan InitPlan, force bool, executor InitExecutor) InitResult {
	result := NewInitResult(plan.Root)
	result.Warnings = append(result.Warnings, plan.Warnings...)
	for _, action := range plan.Actions {
		rel := initRelativePath(plan.Root, action.Target)
		var err error
		switch action.Kind {
		case "skip":
			result.Files.Skipped = append(result.Files.Skipped, rel)
			continue
		case "write":
			err = executor.WriteFile(action.Target, action.Content)
			if err == nil {
				result.Files.Written = append(result.Files.Written, rel)
			}
		case "merge":
			err = executor.MergeAgents(action.Target, action.Content, force)
			if err == nil {
				result.Files.Updated = append(result.Files.Updated, rel)
			}
		default:
			err = fmt.Errorf("unknown init action %q", action.Kind)
		}
		if err != nil {
			result.Status = "failed"
			result.Files.Failed = append(result.Files.Failed, rel)
			result.Warnings = append(result.Warnings, InitWarning{
				Code:    "write-failed",
				Message: fmt.Sprintf("%s: %v", rel, err),
			})
			if action.Required {
				break
			}
		}
	}
	result.Normalize()
	return result
}

func initRelativePath(root, target string) string {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return filepath.ToSlash(target)
	}
	return filepath.ToSlash(rel)
}
