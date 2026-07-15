package core

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DeployFileName is the append-only per-spec deploy ledger. Every step and
// rollback result is recorded here so the rollback chain is computed from
// *recorded* successful steps only — no partial-execution ambiguity (V9/P5.1).
const DeployFileName = "deploy.jsonl"

// MaxDeployPlanBytes caps a deploy config file. Deploy plans are small operator
// documents; a huge file is hostile input and rejected before parsing.
const MaxDeployPlanBytes = 64 * 1024

// maxDeployStepTimeoutSeconds bounds a single step's declared timeout so a
// hostile config cannot pin a step open indefinitely.
const maxDeployStepTimeoutSeconds = 3600

// deployGateNames are the gate keywords a deploy plan may require green before
// it will run (V9 §3). They map to recorded state evidence, never re-run here.
var deployGateNames = map[string]bool{"eval": true, "security": true, "review": true}

// DeployStep is one operator-declared deploy action. Command runs through the
// shared sandboxed exec path; RollbackCommand is its recorded inverse (optional,
// but a step with no rollback contributes nothing to the rollback chain).
type DeployStep struct {
	Name            string `json:"name"`
	Command         string `json:"command"`
	RollbackCommand string `json:"rollbackCommand,omitempty"`
	TimeoutSeconds  int    `json:"timeoutSeconds"`
}

// DeployPlan is the parsed .specd/deploy/<env>.json. It is hostile input:
// unknown fields, missing timeouts, and empty step lists are rejected.
type DeployPlan struct {
	Env              string       `json:"env"`
	RequiresGates    []string     `json:"requiresGates,omitempty"`
	Steps            []DeployStep `json:"steps"`
	ApprovalRequired bool         `json:"approvalRequired,omitempty"`
}

// DeployLedgerEntry is one recorded step or rollback result appended to
// deploy.jsonl. RollbackCommand is copied onto step entries so the rollback
// chain is reconstructable from the ledger alone.
type DeployLedgerEntry struct {
	Seq             int64  `json:"seq"`
	At              string `json:"at"`
	Env             string `json:"env"`
	Kind            string `json:"kind"` // "step" | "rollback"
	Step            string `json:"step"`
	RollbackCommand string `json:"rollbackCommand,omitempty"`
	ExitCode        int    `json:"exitCode"`
	Success         bool   `json:"success"`
}

// DeployPlanPath is the operator-authored config for one environment. The env
// segment is validated (slug shape) before it reaches the filesystem so it can
// never traverse out of .specd/deploy/.
func DeployPlanPath(root, env string) string {
	return filepath.Join(root, ".specd", "deploy", env+".json")
}

// DeployLedgerPath is the append-only deploy ledger for one spec.
func DeployLedgerPath(root, slug string) string {
	return filepath.Join(SpecDir(root, slug), DeployFileName)
}

// ValidateEnv rejects env names that are not safe filename segments, closing off
// path traversal through the `--env` flag (V9 §5).
func ValidateEnv(env string) error {
	if !SlugRE.MatchString(env) {
		return UsageError(fmt.Sprintf("invalid --env %q: must match %s", env, SlugRE.String()))
	}
	return nil
}

// LoadDeployPlan reads and validates the deploy config for env. A missing file
// is a not-found error so the feature is inert until an operator opts in.
func LoadDeployPlan(root, env string) (DeployPlan, error) {
	if err := ValidateEnv(env); err != nil {
		return DeployPlan{}, err
	}
	path := DeployPlanPath(root, env)
	info, err := os.Stat(path)
	if err != nil {
		return DeployPlan{}, NotFoundError(fmt.Sprintf("no deploy config for env %q — create %s", env, path))
	}
	if info.Size() > MaxDeployPlanBytes {
		return DeployPlan{}, GateError(fmt.Sprintf("deploy config %s exceeds %d bytes", path, MaxDeployPlanBytes))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return DeployPlan{}, err
	}
	return ParseDeployPlan(env, data)
}

// ParseDeployPlan strictly decodes and validates a deploy plan. Every step must
// name a command and a positive, bounded timeout; requiresGates entries must be
// recognized gate keywords; the declared env must match the filename's env.
func ParseDeployPlan(env string, data []byte) (DeployPlan, error) {
	var plan DeployPlan
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&plan); err != nil {
		return DeployPlan{}, GateError(fmt.Sprintf("deploy config for env %q: %v", env, err))
	}
	if strings.TrimSpace(plan.Env) == "" {
		plan.Env = env
	}
	if plan.Env != env {
		return DeployPlan{}, GateError(fmt.Sprintf("deploy config env %q does not match file env %q", plan.Env, env))
	}
	if len(plan.Steps) == 0 {
		return DeployPlan{}, GateError(fmt.Sprintf("deploy config for env %q: at least one step required", env))
	}
	seen := map[string]bool{}
	for i, s := range plan.Steps {
		if strings.TrimSpace(s.Name) == "" {
			return DeployPlan{}, GateError(fmt.Sprintf("deploy config for env %q: step %d has no name", env, i))
		}
		if seen[s.Name] {
			return DeployPlan{}, GateError(fmt.Sprintf("deploy config for env %q: duplicate step name %q", env, s.Name))
		}
		seen[s.Name] = true
		if strings.TrimSpace(s.Command) == "" {
			return DeployPlan{}, GateError(fmt.Sprintf("deploy config for env %q: step %q has no command", env, s.Name))
		}
		if s.TimeoutSeconds <= 0 {
			return DeployPlan{}, GateError(fmt.Sprintf("deploy config for env %q: step %q needs a positive timeoutSeconds", env, s.Name))
		}
		if s.TimeoutSeconds > maxDeployStepTimeoutSeconds {
			return DeployPlan{}, GateError(fmt.Sprintf("deploy config for env %q: step %q timeoutSeconds %d exceeds max %d", env, s.Name, s.TimeoutSeconds, maxDeployStepTimeoutSeconds))
		}
	}
	for _, g := range plan.RequiresGates {
		if !deployGateNames[g] {
			return DeployPlan{}, GateError(fmt.Sprintf("deploy config for env %q: unknown gate %q in requiresGates (allowed: eval, security, review)", env, g))
		}
	}
	return plan, nil
}

// DeployPreconditions returns the human-readable reasons a spec may not deploy
// under plan: it must be complete, the human deploy gate must be current when
// the plan (or a production env) requires it, and every required gate must have
// recorded green evidence. Empty slice = clear to deploy. Pure and deterministic.
func DeployPreconditions(state *State, plan DeployPlan, productionEnv bool) []string {
	var problems []string
	if state == nil {
		return []string{"spec state not found"}
	}
	if state.Status != StatusComplete {
		problems = append(problems, fmt.Sprintf("spec is %q, not complete — deploy requires a completed spec", state.Status))
	}
	if plan.ApprovalRequired || productionEnv {
		if state.DeployApproval == nil || state.DeployApproval.Env != plan.Env {
			problems = append(problems, fmt.Sprintf("no human deploy approval recorded for env %q — run `specd approve %s --deploy --env %s`", plan.Env, state.Spec, plan.Env))
		}
	}
	for _, g := range plan.RequiresGates {
		if reason := deployGateEvidenceGap(state, g); reason != "" {
			problems = append(problems, reason)
		}
	}
	return problems
}

// deployGateEvidenceGap reports why a required gate's recorded evidence is not
// green, or "" when it is. It reads recorded state only — it never re-runs a gate.
func deployGateEvidenceGap(state *State, gate string) string {
	switch gate {
	case "eval":
		if !stateHasPassingEval(state) {
			return "required gate 'eval': no passing eval run recorded"
		}
	case "security":
		if state.Security == nil {
			return "required gate 'security': no security scan recorded"
		}
		if state.Security.Blocking > 0 {
			return fmt.Sprintf("required gate 'security': %d blocking finding(s) unresolved", state.Security.Blocking)
		}
	case "review":
		if state.Review == nil {
			return "required gate 'review': no review recorded"
		}
		if state.Review.Verdict != string(ReviewApprove) {
			return fmt.Sprintf("required gate 'review': verdict is %q, not approve", state.Review.Verdict)
		}
	}
	return ""
}

// stateHasPassingEval reports whether any recorded eval run passed its minScore.
func stateHasPassingEval(state *State) bool {
	for _, e := range state.Evals {
		if e.Pass {
			return true
		}
	}
	return false
}

// AppendDeployEntry appends one result to the spec's deploy ledger, assigning
// the next sequence number. The ledger is append-only (dual-write discipline).
func AppendDeployEntry(root, slug string, entry DeployLedgerEntry) error {
	if err := ValidateSlug(slug); err != nil {
		return err
	}
	path := DeployLedgerPath(root, slug)
	entries, err := ReadDeployLedger(root, slug)
	if err != nil {
		return err
	}
	entry.Seq = int64(len(entries)) + 1
	if entry.At == "" {
		entry.At = NowISO()
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return AppendFile(path, string(line)+"\n")
}

// ReadDeployLedger reads all recorded entries in order. A missing ledger is an
// empty slice, not an error.
func ReadDeployLedger(root, slug string) ([]DeployLedgerEntry, error) {
	path := DeployLedgerPath(root, slug)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []DeployLedgerEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), MaxDeployPlanBytes)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e DeployLedgerEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, fmt.Errorf("deploy ledger %s: corrupt entry: %w", path, err)
		}
		out = append(out, e)
	}
	return out, sc.Err()
}

// RollbackChain computes the inverse chain for the most recent deploy run: the
// successful step entries since the last rollback marker, in reverse order, each
// carrying a non-empty rollback command. It reads recorded successful steps only
// (V9 §5) so a mid-chain failure yields exactly the steps that actually ran.
func RollbackChain(entries []DeployLedgerEntry) []DeployLedgerEntry {
	// Walk backwards to the last rollback entry; everything after it is the
	// current run's step record.
	start := 0
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Kind == "rollback" {
			start = i + 1
			break
		}
	}
	var chain []DeployLedgerEntry
	for i := len(entries) - 1; i >= start; i-- {
		e := entries[i]
		if e.Kind == "step" && e.Success && strings.TrimSpace(e.RollbackCommand) != "" {
			chain = append(chain, e)
		}
	}
	return chain
}

// SortedDeployEnvs lists the configured deploy environments (files under
// .specd/deploy/) in deterministic order, for help/diagnostics.
func SortedDeployEnvs(root string) []string {
	dir := filepath.Join(root, ".specd", "deploy")
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		out = append(out, strings.TrimSuffix(e.Name(), ".json"))
	}
	sort.Strings(out)
	return out
}
