package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// InitResultSchemaVersion is the schema version stamped onto every InitResult
// returned by ExecuteInitPlan, for consumers parsing the JSON output.
const InitResultSchemaVersion = 1

// InitOptions are the user-supplied flags that drive PlanInit: which root to
// scaffold, which mutually-exclusive mode (force/repair/refresh) to run in,
// dry-run/interactive behavior, and agent/scope selection.
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

// InitAction is one planned filesystem action (write, merge, or skip) for a
// single scaffold asset, including whether it is destructive or required.
type InitAction struct {
	Kind        string `json:"kind"`
	Target      string `json:"target"`
	Description string `json:"description"`
	Destructive bool   `json:"destructive"`
	Required    bool   `json:"required"`
	Template    string `json:"template"`
	Content     string `json:"-"`
}

// InitPlan is the validated, write-free result of PlanInit: the resolved mode,
// whether this is a fresh (.specd/-absent) install, and the ordered list of
// actions to apply.
type InitPlan struct {
	Root     string        `json:"root"`
	Mode     string        `json:"mode"`
	DryRun   bool          `json:"dryRun"`
	Fresh    bool          `json:"fresh"`
	Actions  []InitAction  `json:"actions"`
	Warnings []InitWarning `json:"warnings"`
}

// InitFileResults buckets the relative paths touched by an init run by
// outcome: newly written, updated (merged), skipped (preserved), or failed.
type InitFileResults struct {
	Written []string `json:"written"`
	Updated []string `json:"updated"`
	Skipped []string `json:"skipped"`
	Failed  []string `json:"failed"`
}

// InitAgentResults buckets coding-agent hosts by outcome: passively detected,
// automatically configured, or requiring manual setup.
type InitAgentResults struct {
	Detected   []string `json:"detected"`
	Configured []string `json:"configured"`
	Manual     []string `json:"manual"`
}

// InitVerificationResult reports the outcome of the in-process MCP handshake
// performed during init: the MCP status, negotiated protocol version, and
// discovered tool count.
type InitVerificationResult struct {
	MCP             string `json:"mcp"`
	ProtocolVersion string `json:"protocolVersion"`
	ToolCount       int    `json:"toolCount"`
}

// InitWarning is a non-fatal, machine-readable issue surfaced by an init run,
// identified by a stable code and a human-readable message.
type InitWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// InitNextAction is the single suggested follow-up step returned to the
// caller (typically the host agent) after init completes.
type InitNextAction struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// InitResult is the complete, versioned outcome of an init run: status, file
// and agent results, MCP verification, warnings, and the next suggested
// action. It is the JSON document emitted under --json.
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

// NewInitResult builds the default InitResult for the given root: schema
// version stamped, status "ready", empty file/agent buckets, an
// unattempted MCP verification, and the standard "create a spec" next action.
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

// Normalize replaces any nil result slices with empty slices and sorts every
// file/agent bucket and the warnings list, so JSON output is deterministic
// regardless of the order actions were appended in.
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

// InitExecutor abstracts the filesystem operations ExecuteInitPlan performs,
// so tests can substitute fakes without touching the real disk.
type InitExecutor struct {
	WriteFile   func(path, content string) error
	MergeAgents func(path, content string, force bool) error
	MkdirTemp   func(dir, pattern string) (string, error)
	Rename      func(oldPath, newPath string) error
	RemoveAll   func(path string) error
}

// DefaultInitExecutor returns the production InitExecutor, wired to the real
// filesystem (AtomicWrite, MergeAgentsMD, and the os package primitives).
func DefaultInitExecutor() InitExecutor {
	return InitExecutor{
		WriteFile:   AtomicWrite,
		MergeAgents: MergeAgentsMD,
		MkdirTemp:   os.MkdirTemp,
		Rename:      os.Rename,
		RemoveAll:   os.RemoveAll,
	}
}

// ValidateInitOptions rejects InitOptions where more than one of
// Force, Repair, and Refresh is set, since those modes are mutually
// exclusive.
func ValidateInitOptions(options InitOptions) error {
	modes := 0
	for _, enabled := range []bool{options.Force, options.Repair, options.Refresh} {
		if enabled {
			modes++
		}
	}
	if modes > 1 {
		return fmt.Errorf("--repair, --refresh, and --force are mutually exclusive")
	}
	return nil
}

// InitMode derives the mode label ("force", "repair", "refresh", or "init")
// from InitOptions, used for InitPlan.Mode and InitResult.Mode.
func InitMode(options InitOptions) string {
	switch {
	case options.Force:
		return "force"
	case options.Repair:
		return "repair"
	case options.Refresh:
		return "refresh"
	default:
		return "init"
	}
}

// PlanInit validates and resolves every template before returning actions. It
// reads project state but performs no writes.
//
//nolint:gocyclo // pre-existing complexity debt, out of scope for spec S3 — tracked for a future cleanup pass
func PlanInit(options InitOptions, assets []ScaffoldAsset, readTemplate func(string) (string, error)) (InitPlan, error) {
	if options.Root == "" {
		return InitPlan{}, fmt.Errorf("init root is required")
	}
	if err := ValidateInitOptions(options); err != nil {
		return InitPlan{}, err
	}
	if err := ValidateScaffoldManifest(assets, readTemplate); err != nil {
		return InitPlan{}, err
	}
	_, specdErr := os.Stat(filepath.Join(options.Root, ".specd"))
	plan := InitPlan{
		Root:     options.Root,
		Mode:     InitMode(options),
		DryRun:   options.DryRun,
		Fresh:    os.IsNotExist(specdErr),
		Actions:  make([]InitAction, 0, len(assets)),
		Warnings: []InitWarning{},
	}
	if specdErr != nil && !os.IsNotExist(specdErr) {
		return InitPlan{}, fmt.Errorf("inspect .specd: %w", specdErr)
	}
	if options.Force {
		plan.Warnings = append(plan.Warnings, InitWarning{
			Code:    "destructive-force",
			Message: "--force replaces all scaffold files and resets AGENTS.md, including user content outside managed markers",
		})
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
			_, statErr := os.Stat(target)
			if statErr != nil && !os.IsNotExist(statErr) {
				return InitPlan{}, fmt.Errorf("inspect %s: %w", target, statErr)
			}
			switch {
			case options.Repair && statErr == nil:
				action.Kind = "skip"
				action.Description = "preserve existing file during repair"
			default:
				if statErr == nil && !options.Force {
					if _, err := ValidateAgentsMD(target); err != nil {
						return InitPlan{}, fmt.Errorf("inspect %s: %w", target, err)
					}
				}
				action.Kind = "merge"
				action.Description = "merge managed marker section"
				action.Destructive = options.Force
			}
		case ScaffoldCreate:
			_, statErr := os.Stat(target)
			switch {
			case statErr == nil && !options.Force && (!options.Refresh || !asset.Refresh):
				action.Kind = "skip"
				action.Description = "preserve existing file"
			case statErr != nil && !os.IsNotExist(statErr):
				return InitPlan{}, fmt.Errorf("inspect %s: %w", target, statErr)
			default:
				action.Kind = "write"
				if statErr == nil {
					action.Description = "replace managed scaffold asset"
				} else {
					action.Description = "write embedded scaffold asset"
				}
				action.Destructive = options.Force
			}
		}
		plan.Actions = append(plan.Actions, action)
	}
	return plan, nil
}

// ExecuteInitPlan applies a previously computed InitPlan via executor,
// returning the resulting InitResult. A dry-run plan only categorizes actions
// without writing; a fresh-install plan stages and atomically renames the new
// .specd/ tree via executeFreshInitPlan; otherwise actions are applied
// in place, with the first failure on a required action stopping the run.
func ExecuteInitPlan(plan InitPlan, force bool, executor InitExecutor) InitResult {
	result := NewInitResult(plan.Root)
	result.Mode = plan.Mode
	result.Warnings = append(result.Warnings, plan.Warnings...)
	if plan.DryRun {
		for _, action := range plan.Actions {
			rel := initRelativePath(plan.Root, action.Target)
			switch action.Kind {
			case "skip":
				result.Files.Skipped = append(result.Files.Skipped, rel)
			case "write":
				result.Files.Written = append(result.Files.Written, rel)
			case "merge":
				result.Files.Updated = append(result.Files.Updated, rel)
			}
		}
		result.Status = "planned"
		result.Normalize()
		return result
	}
	if plan.Fresh {
		return executeFreshInitPlan(plan, force, executor, result)
	}
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

func executeFreshInitPlan(plan InitPlan, force bool, executor InitExecutor, result InitResult) InitResult {
	stage, err := executor.MkdirTemp(plan.Root, ".specd.init-*")
	if err != nil {
		return failedInitResult(result, ".specd", "stage-failed", err)
	}
	defer func() { _ = executor.RemoveAll(stage) }()

	staged := make([]string, 0, len(plan.Actions))
	for _, action := range plan.Actions {
		rel := initRelativePath(plan.Root, action.Target)
		if action.Kind == "skip" || !strings.HasPrefix(rel, ".specd/") {
			continue
		}
		stageTarget := filepath.Join(stage, filepath.FromSlash(strings.TrimPrefix(rel, ".specd/")))
		if action.Kind != "write" {
			return failedInitResult(result, rel, "stage-failed", fmt.Errorf("unsupported staged action %q", action.Kind))
		}
		if err := executor.WriteFile(stageTarget, action.Content); err != nil {
			return failedInitResult(result, rel, "stage-failed", err)
		}
		staged = append(staged, rel)
	}
	if err := executor.Rename(stage, filepath.Join(plan.Root, ".specd")); err != nil {
		return failedInitResult(result, ".specd", "commit-failed", err)
	}
	result.Files.Written = append(result.Files.Written, staged...)

	for _, action := range plan.Actions {
		rel := initRelativePath(plan.Root, action.Target)
		if strings.HasPrefix(rel, ".specd/") {
			continue
		}
		switch action.Kind {
		case "skip":
			result.Files.Skipped = append(result.Files.Skipped, rel)
		case "write":
			// External (non-.specd) scaffold assets, e.g. .claude/agents/*.
			// Written directly to their target (AtomicWrite creates parents);
			// they live outside the staged .specd tree.
			if err := executor.WriteFile(action.Target, action.Content); err != nil {
				return failedInitResult(result, rel, "external-write-failed", err)
			}
			result.Files.Written = append(result.Files.Written, rel)
		case "merge":
			if err := executor.MergeAgents(action.Target, action.Content, force); err != nil {
				result = failedInitResult(result, rel, "external-merge-failed", err)
				result.Warnings = append(result.Warnings, InitWarning{
					Code:    "residual-scaffold",
					Message: ".specd was committed successfully; the original external file was preserved and can be repaired with `specd init --repair`",
				})
				result.Normalize()
				return result
			}
			result.Files.Updated = append(result.Files.Updated, rel)
		}
	}
	result.Normalize()
	return result
}

func failedInitResult(result InitResult, path, code string, err error) InitResult {
	result.Status = "failed"
	result.Files.Failed = append(result.Files.Failed, path)
	result.Warnings = append(result.Warnings, InitWarning{
		Code:    code,
		Message: fmt.Sprintf("%s: %v", path, err),
	})
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
