package core

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"
)

const OrchestrationModelVersion = 1

type OrchestrationAction string

const (
	OrchestrationIdle            OrchestrationAction = "idle"
	OrchestrationRequestApproval OrchestrationAction = "request-approval"
	OrchestrationDispatch        OrchestrationAction = "dispatch"
	OrchestrationDispatchAuthor  OrchestrationAction = "dispatch-authoring"
	OrchestrationAdvancePhase    OrchestrationAction = "advance-phase"
	OrchestrationWait            OrchestrationAction = "wait"
	OrchestrationRetry           OrchestrationAction = "retry"
	OrchestrationCancel          OrchestrationAction = "cancel"
	OrchestrationReplan          OrchestrationAction = "replan"
	OrchestrationEscalate        OrchestrationAction = "escalate"
	OrchestrationCompleteSession OrchestrationAction = "complete-session"
	// OrchestrationCompact instructs the host to shed conversation context
	// (a `/clear`) between planning ratchets or under token pressure. It is an
	// effecting decision with no worker dispatch: the engine writes a phase
	// summary and a ledger checkpoint; the host performs the real clear.
	OrchestrationCompact OrchestrationAction = "compact"
)

// Compaction policy modes. Empty is treated as CompactionNone.
const (
	CompactionNone   = "none"
	CompactionPhase  = "phase"
	CompactionBudget = "budget"
	CompactionBoth   = "both"
)

// ContextLedgerEntry is one persistent record in a session's context ledger. It
// is the per-session counterpart of the ephemeral manifest token estimate: a
// pre-dispatch entry records the budget the manifest computed, a post-task entry
// records the host-reported actuals, and a compact entry marks a checkpoint
// where the host shed context. All entries together form the token high-water
// trail the Brain reasons over.
type ContextLedgerEntry struct {
	StepSequence       uint64 `json:"stepSequence"`
	Phase              Phase  `json:"phase"`
	Action             string `json:"action"` // dispatch | compact | approve | step
	EstimatedTokens    int    `json:"estimatedTokens"`
	HostReportedTokens int    `json:"hostReportedTokens,omitempty"`
	Budget             int    `json:"budget"`
	SoftCeiling        int    `json:"softCeiling"`
	Compacted          bool   `json:"compacted,omitempty"`
	CompactedAt        string `json:"compactedAt,omitempty"`
	Reason             string `json:"reason"` // phase-complete | budget-threshold | manual-clear
}

type OrchestrationSessionStatus string

const (
	OrchestrationSessionRunning    OrchestrationSessionStatus = "running"
	OrchestrationSessionPaused     OrchestrationSessionStatus = "paused"
	OrchestrationSessionCancelling OrchestrationSessionStatus = "cancelling"
	OrchestrationSessionComplete   OrchestrationSessionStatus = "complete"
	OrchestrationSessionFailed     OrchestrationSessionStatus = "failed"
)

type OrchestrationEscalationCode string

const (
	EscalationNone              OrchestrationEscalationCode = "none"
	EscalationUnknownState      OrchestrationEscalationCode = "unknown-state"
	EscalationInvalidGraph      OrchestrationEscalationCode = "invalid-graph"
	EscalationConflictingLease  OrchestrationEscalationCode = "conflicting-lease"
	EscalationCASExhausted      OrchestrationEscalationCode = "cas-exhausted"
	EscalationPolicyViolation   OrchestrationEscalationCode = "policy-violation"
	EscalationRetriesExhausted  OrchestrationEscalationCode = "retries-exhausted"
	EscalationHumanIntervention OrchestrationEscalationCode = "human-intervention"
)

type OrchestrationPolicy struct {
	ApprovalPolicy           string  `json:"approvalPolicy"`
	MaxWorkers               int     `json:"maxWorkers"`
	MaxRetries               int     `json:"maxRetries"`
	SessionTimeoutSeconds    int     `json:"sessionTimeoutSeconds"`
	HostReportedCostLimitUSD float64 `json:"hostReportedCostLimitUSD"`
	// CompactionPolicy selects automatic context compaction: none|phase|budget|
	// both. Empty is treated as none. omitempty keeps existing session.json
	// byte-identical.
	CompactionPolicy string `json:"compactionPolicy,omitempty"`
	// CompactionBudgetThreshold is the fraction of the manifest budget at which
	// budget-driven compaction fires, in [0,1]. omitempty keeps the zero value
	// (no budget trigger) out of byte-stable session.json.
	CompactionBudgetThreshold float64 `json:"compactionBudgetThreshold,omitempty"`
}

// effectiveCompactionPolicy maps the empty policy to CompactionNone so callers
// never branch on the empty string.
func (p OrchestrationPolicy) effectiveCompactionPolicy() string {
	if p.CompactionPolicy == "" {
		return CompactionNone
	}
	return p.CompactionPolicy
}

func (p OrchestrationPolicy) compactsOnPhase() bool {
	c := p.effectiveCompactionPolicy()
	return c == CompactionPhase || c == CompactionBoth
}

func (p OrchestrationPolicy) compactsOnBudget() bool {
	c := p.effectiveCompactionPolicy()
	return c == CompactionBudget || c == CompactionBoth
}

type OrchestrationTaskSnapshot struct {
	ID       string     `json:"id"`
	Wave     int        `json:"wave"`
	Status   TaskStatus `json:"status"`
	Attempt  int        `json:"attempt"`
	Role     string     `json:"role"`
	Depends  []string   `json:"depends"`
	Verified bool       `json:"verified"`
}

type OrchestrationLeaseSnapshot struct {
	WorkerID   string `json:"workerId"`
	TaskID     string `json:"taskId"`
	Attempt    int    `json:"attempt"`
	LeaseUntil string `json:"leaseUntil"`
}

type OrchestrationFailure struct {
	TaskID    string `json:"taskId"`
	Attempt   int    `json:"attempt"`
	Kind      string `json:"kind"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type OrchestrationSnapshot struct {
	Version          int                          `json:"version"`
	SessionID        string                       `json:"sessionId"`
	Spec             string                       `json:"spec"`
	Revision         int                          `json:"revision"`
	Status           SpecStatus                   `json:"status"`
	Phase            Phase                        `json:"phase"`
	Gate             Gate                         `json:"gate"`
	HumanOnlyGate    bool                         `json:"humanOnlyGate"`
	Runnable         []OrchestrationTaskSnapshot  `json:"runnable"`
	ActiveLeases     []OrchestrationLeaseSnapshot `json:"activeLeases"`
	RecentFailures   []OrchestrationFailure       `json:"recentFailures"`
	SessionExpiresAt string                       `json:"sessionExpiresAt"`
	// Authoring is the planning-phase frontier: present when the spec is in a
	// planning status (requirements/design/tasks) and the phase artifact is
	// absent or fails `specd check`. It is the authoring counterpart of
	// Runnable for the execution DAG. PlanningReady is true when the spec is in
	// a planning status and the current artifact already passes its gate (the
	// phase is ready to advance).
	Authoring     *OrchestrationAuthoring `json:"authoring,omitempty"`
	PlanningReady bool                    `json:"planningReady"`
	// AccumulatedCostUSD is the sum of host-reported cost across the session's
	// evidence events. It is hostReported and untrusted — it never gates
	// completion — but it drives the advisory cost-limit escalation (GAP-4).
	AccumulatedCostUSD float64 `json:"accumulatedCostUSD"`
	// SessionExpired is true when the session's fixed wall-clock deadline
	// (session.ExpiresAt, set at start from sessionTimeoutSeconds) has passed.
	// It forces a terminal escalation rather than relying on lease expiry alone.
	SessionExpired bool `json:"sessionExpired"`
	// LastCompactionStep, LedgerEstimatedTokens, and LedgerBudget carry the
	// compaction inputs DecideOrchestration needs without breaking its pure
	// (snapshot, policy) signature: they are read from the persisted session's
	// ledger tail in SenseOrchestration. omitempty keeps snapshots without a
	// session (plain-controller mode) byte-identical to the pre-compaction shape.
	LastCompactionStep    uint64 `json:"lastCompactionStep,omitempty"`
	LedgerEstimatedTokens int    `json:"ledgerEstimatedTokens,omitempty"`
	LedgerBudget          int    `json:"ledgerBudget,omitempty"`
}

// OrchestrationAuthoring is a synthetic, single authoring work item describing
// the phase artifact a worker must produce to clear the current planning gate.
// Planning is sequential (one artifact at a time), so the frontier is at most
// one item.
type OrchestrationAuthoring struct {
	WorkID   string   `json:"workId"`   // reserved authoring ID (A1/A2/A3)
	Artifact string   `json:"artifact"` // e.g. "requirements.md"
	Gate     string   `json:"gate"`     // gate the artifact must clear
	Role     string   `json:"role"`     // worker role for the mission
	Issues   []string `json:"issues"`   // current `specd check`-shaped reasons
}

type OrchestrationEscalation struct {
	Code    OrchestrationEscalationCode `json:"code"`
	Message string                      `json:"message"`
}

type OrchestrationDecision struct {
	Version int                 `json:"version"`
	Action  OrchestrationAction `json:"action"`
	Spec    string              `json:"spec"`
	TaskID  string              `json:"taskId,omitempty"`
	Attempt int                 `json:"attempt,omitempty"`
	// Artifact is set on dispatch-authoring / advance-phase decisions: the
	// planning artifact (e.g. "design.md") the decision concerns.
	Artifact       string                  `json:"artifact,omitempty"`
	Reason         string                  `json:"reason"`
	IdempotencyKey string                  `json:"idempotencyKey"`
	Escalation     OrchestrationEscalation `json:"escalation"`
}

type OrchestrationSession struct {
	Version      int                        `json:"version"`
	SessionID    string                     `json:"sessionId"`
	Spec         string                     `json:"spec"`
	Owner        string                     `json:"owner"`
	Status       OrchestrationSessionStatus `json:"status"`
	Policy       OrchestrationPolicy        `json:"policy"`
	CreatedAt    string                     `json:"createdAt"`
	UpdatedAt    string                     `json:"updatedAt"`
	ExpiresAt    string                     `json:"expiresAt"`
	LastSequence uint64                     `json:"lastSequence"`
	// ContextLedger is the persistent token-accounting trail (R1). LastCompactionStep
	// is the snapshot revision at which the most recent compaction fired, guarding
	// the phase-boundary trigger from re-emitting. PeakTokens is the high-water mark
	// across estimated and host-reported entries. All omitempty so pre-ledger
	// session.json round-trips byte-identical.
	ContextLedger      []ContextLedgerEntry `json:"contextLedger,omitempty"`
	LastCompactionStep uint64               `json:"lastCompactionStep,omitempty"`
	PeakTokens         int                  `json:"peakTokens,omitempty"`
}

func NewOrchestrationPolicy(cfg OrchestrationCfg) (OrchestrationPolicy, error) {
	if err := ValidateOrchestrationConfig(&cfg); err != nil {
		return OrchestrationPolicy{}, err
	}
	policy := OrchestrationPolicy{
		ApprovalPolicy:           cfg.ApprovalPolicy,
		MaxWorkers:               cfg.MaxWorkers,
		MaxRetries:               cfg.MaxRetries,
		SessionTimeoutSeconds:    cfg.SessionTimeoutMinutes * 60,
		HostReportedCostLimitUSD: cfg.HostReportedCostLimitUSD,
		CompactionPolicy:          cfg.CompactionPolicy,
		CompactionBudgetThreshold: cfg.CompactionBudgetThreshold,
	}
	if err := ValidateOrchestrationPolicy(policy); err != nil {
		return OrchestrationPolicy{}, err
	}
	return policy, nil
}

func ValidateOrchestrationPolicy(policy OrchestrationPolicy) error {
	if !oneOf(policy.ApprovalPolicy, "manual", "planning", "session") {
		return fmt.Errorf("orchestration model: unsupported approval policy %q", policy.ApprovalPolicy)
	}
	if policy.MaxWorkers < minMaxWorkers || policy.MaxWorkers > maxMaxWorkers {
		return fmt.Errorf("orchestration model: maxWorkers outside [%d,%d]", minMaxWorkers, maxMaxWorkers)
	}
	if policy.MaxRetries < minMaxRetries || policy.MaxRetries > maxMaxRetries {
		return fmt.Errorf("orchestration model: maxRetries outside [%d,%d]", minMaxRetries, maxMaxRetries)
	}
	minTimeout := minSessionTimeoutMinutes * 60
	maxTimeout := maxSessionTimeoutMinutes * 60
	if policy.SessionTimeoutSeconds < minTimeout || policy.SessionTimeoutSeconds > maxTimeout {
		return fmt.Errorf("orchestration model: sessionTimeoutSeconds outside [%d,%d]", minTimeout, maxTimeout)
	}
	if math.IsNaN(policy.HostReportedCostLimitUSD) ||
		math.IsInf(policy.HostReportedCostLimitUSD, 0) ||
		policy.HostReportedCostLimitUSD < 0 {
		return fmt.Errorf("orchestration model: hostReportedCostLimitUSD must be finite and non-negative")
	}
	if !oneOf(policy.effectiveCompactionPolicy(), CompactionNone, CompactionPhase, CompactionBudget, CompactionBoth) {
		return fmt.Errorf("orchestration model: unsupported compaction policy %q", policy.CompactionPolicy)
	}
	if math.IsNaN(policy.CompactionBudgetThreshold) ||
		math.IsInf(policy.CompactionBudgetThreshold, 0) ||
		policy.CompactionBudgetThreshold < 0 || policy.CompactionBudgetThreshold > 1 {
		return fmt.Errorf("orchestration model: compactionBudgetThreshold must be finite and within [0,1]")
	}
	return nil
}

func ValidateOrchestrationSnapshot(snapshot OrchestrationSnapshot) error {
	if snapshot.Version != OrchestrationModelVersion {
		return fmt.Errorf("orchestration model: unsupported snapshot version %d", snapshot.Version)
	}
	if err := validateACPOpaqueID("session ID", snapshot.SessionID); err != nil {
		return err
	}
	if err := ValidateSlug(snapshot.Spec); err != nil {
		return fmt.Errorf("orchestration model: invalid spec: %w", err)
	}
	if snapshot.Revision < 0 {
		return fmt.Errorf("orchestration model: revision must be non-negative")
	}
	if !validSpecStatus(snapshot.Status) || !validPhase(snapshot.Phase) || !validGate(snapshot.Gate) {
		return fmt.Errorf("orchestration model: unsupported lifecycle state")
	}
	if _, err := parseACPTime("sessionExpiresAt", snapshot.SessionExpiresAt); err != nil {
		return err
	}
	if math.IsNaN(snapshot.AccumulatedCostUSD) ||
		math.IsInf(snapshot.AccumulatedCostUSD, 0) ||
		snapshot.AccumulatedCostUSD < 0 {
		return fmt.Errorf("orchestration model: accumulatedCostUSD must be finite and non-negative")
	}
	runnableIDs := make(map[string]struct{}, len(snapshot.Runnable))
	for _, task := range snapshot.Runnable {
		if !acpTaskIDRE.MatchString(task.ID) || task.Wave < 0 || task.Attempt < 1 ||
			!validTaskStatus(task.Status) || !IsValidRole(task.Role) {
			return fmt.Errorf("orchestration model: invalid runnable task %q", task.ID)
		}
		if _, duplicate := runnableIDs[task.ID]; duplicate {
			return fmt.Errorf("orchestration model: duplicate runnable task %q", task.ID)
		}
		runnableIDs[task.ID] = struct{}{}
		dependencies := make(map[string]struct{}, len(task.Depends))
		for _, dependency := range task.Depends {
			if !acpTaskIDRE.MatchString(dependency) {
				return fmt.Errorf("orchestration model: invalid dependency %q", dependency)
			}
			if _, duplicate := dependencies[dependency]; duplicate {
				return fmt.Errorf("orchestration model: duplicate dependency %q", dependency)
			}
			dependencies[dependency] = struct{}{}
		}
	}
	workers := make(map[string]struct{}, len(snapshot.ActiveLeases))
	for _, lease := range snapshot.ActiveLeases {
		if err := validateACPRuntimeSegment("worker ID", lease.WorkerID); err != nil {
			return err
		}
		if _, duplicate := workers[lease.WorkerID]; duplicate {
			return fmt.Errorf("orchestration model: duplicate active worker %q", lease.WorkerID)
		}
		workers[lease.WorkerID] = struct{}{}
		if !acpTaskIDRE.MatchString(lease.TaskID) || lease.Attempt < 1 {
			return fmt.Errorf("orchestration model: invalid active lease")
		}
		if _, err := parseACPTime("leaseUntil", lease.LeaseUntil); err != nil {
			return err
		}
	}
	for _, failure := range snapshot.RecentFailures {
		if !acpTaskIDRE.MatchString(failure.TaskID) || failure.Attempt < 1 ||
			failure.Kind == "" || failure.Message == "" {
			return fmt.Errorf("orchestration model: invalid failure record")
		}
	}
	if snapshot.HumanOnlyGate && snapshot.Gate != GateAwaitingApproval {
		return fmt.Errorf("orchestration model: humanOnlyGate requires awaiting-approval")
	}
	return nil
}

func ValidateOrchestrationDecision(decision OrchestrationDecision) error {
	if decision.Version != OrchestrationModelVersion {
		return fmt.Errorf("orchestration model: unsupported decision version %d", decision.Version)
	}
	if !validOrchestrationAction(decision.Action) {
		return fmt.Errorf("orchestration model: unsupported action %q", decision.Action)
	}
	if err := ValidateSlug(decision.Spec); err != nil {
		return fmt.Errorf("orchestration model: invalid spec: %w", err)
	}
	if decision.TaskID != "" && !acpTaskIDRE.MatchString(decision.TaskID) {
		return fmt.Errorf("orchestration model: invalid taskId")
	}
	if decision.Attempt < 0 || (decision.Attempt > 0 && decision.TaskID == "") {
		return fmt.Errorf("orchestration model: invalid attempt")
	}
	switch decision.Action {
	case OrchestrationDispatchAuthor, OrchestrationAdvancePhase:
		if authoringArtifactID(decision.Artifact) == "" {
			return fmt.Errorf("orchestration model: %s requires a known planning artifact", decision.Action)
		}
	default:
		if decision.Artifact != "" {
			return fmt.Errorf("orchestration model: artifact only valid on authoring decisions")
		}
	}
	if err := validateACPText("decision reason", decision.Reason, true); err != nil {
		return fmt.Errorf("orchestration model: %w", err)
	}
	if err := validateACPText("decision idempotencyKey", decision.IdempotencyKey, true); err != nil {
		return fmt.Errorf("orchestration model: %w", err)
	}
	if !validEscalationCode(decision.Escalation.Code) {
		return fmt.Errorf("orchestration model: unsupported escalation code %q", decision.Escalation.Code)
	}
	if decision.Action == OrchestrationEscalate && decision.Escalation.Code == EscalationNone {
		return fmt.Errorf("orchestration model: escalate requires an escalation code")
	}
	if decision.Action != OrchestrationEscalate && decision.Escalation.Code != EscalationNone {
		return fmt.Errorf("orchestration model: non-escalation decision has escalation code")
	}
	if err := validateACPText(
		"escalation message",
		decision.Escalation.Message,
		decision.Action == OrchestrationEscalate,
	); err != nil {
		return fmt.Errorf("orchestration model: %w", err)
	}
	return nil
}

func ValidateOrchestrationSession(session OrchestrationSession) error {
	if session.Version != OrchestrationModelVersion {
		return fmt.Errorf("orchestration model: unsupported session version %d", session.Version)
	}
	if err := validateACPOpaqueID("session ID", session.SessionID); err != nil {
		return err
	}
	if err := ValidateSlug(session.Spec); err != nil {
		return fmt.Errorf("orchestration model: invalid spec: %w", err)
	}
	if err := validateACPText("session owner", session.Owner, true); err != nil {
		return fmt.Errorf("orchestration model: %w", err)
	}
	if !validSessionStatus(session.Status) {
		return fmt.Errorf("orchestration model: invalid session status")
	}
	if err := ValidateOrchestrationPolicy(session.Policy); err != nil {
		return err
	}
	created, err := parseACPTime("createdAt", session.CreatedAt)
	if err != nil {
		return err
	}
	updated, err := parseACPTime("updatedAt", session.UpdatedAt)
	if err != nil {
		return err
	}
	expires, err := parseACPTime("expiresAt", session.ExpiresAt)
	if err != nil {
		return err
	}
	if updated.Before(created) || !expires.After(created) {
		return fmt.Errorf("orchestration model: invalid session time ordering")
	}
	return nil
}

func CanonicalOrchestrationJSON(value any) ([]byte, error) {
	normalized, err := normalizeOrchestrationValue(value)
	if err != nil {
		return nil, err
	}
	raw, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func normalizeOrchestrationValue(value any) (any, error) {
	switch typed := value.(type) {
	case OrchestrationSnapshot:
		if err := ValidateOrchestrationSnapshot(typed); err != nil {
			return nil, err
		}
		typed.Runnable = append([]OrchestrationTaskSnapshot{}, typed.Runnable...)
		typed.ActiveLeases = append([]OrchestrationLeaseSnapshot{}, typed.ActiveLeases...)
		typed.RecentFailures = append([]OrchestrationFailure{}, typed.RecentFailures...)
		for i := range typed.Runnable {
			typed.Runnable[i].Depends = append([]string{}, typed.Runnable[i].Depends...)
			sort.Strings(typed.Runnable[i].Depends)
		}
		sort.Slice(typed.Runnable, func(i, j int) bool { return taskOrdinalLess(typed.Runnable[i].ID, typed.Runnable[j].ID) })
		sort.Slice(typed.ActiveLeases, func(i, j int) bool {
			if typed.ActiveLeases[i].TaskID != typed.ActiveLeases[j].TaskID {
				return taskOrdinalLess(typed.ActiveLeases[i].TaskID, typed.ActiveLeases[j].TaskID)
			}
			return typed.ActiveLeases[i].WorkerID < typed.ActiveLeases[j].WorkerID
		})
		sort.Slice(typed.RecentFailures, func(i, j int) bool {
			if typed.RecentFailures[i].TaskID != typed.RecentFailures[j].TaskID {
				return taskOrdinalLess(typed.RecentFailures[i].TaskID, typed.RecentFailures[j].TaskID)
			}
			return typed.RecentFailures[i].Attempt < typed.RecentFailures[j].Attempt
		})
		return typed, nil
	case OrchestrationDecision:
		if err := ValidateOrchestrationDecision(typed); err != nil {
			return nil, err
		}
		return typed, nil
	case OrchestrationSession:
		if err := ValidateOrchestrationSession(typed); err != nil {
			return nil, err
		}
		return typed, nil
	case OrchestrationPolicy:
		if err := ValidateOrchestrationPolicy(typed); err != nil {
			return nil, err
		}
		return typed, nil
	default:
		return nil, fmt.Errorf("orchestration model: unsupported canonical JSON type %T", value)
	}
}

func validSpecStatus(status SpecStatus) bool {
	switch status {
	case StatusRequirements, StatusDesign, StatusTasks, StatusExecuting, StatusVerifying, StatusComplete, StatusBlocked:
		return true
	}
	return false
}

func validPhase(phase Phase) bool {
	switch phase {
	case PhasePerceive, PhaseAnalyze, PhasePlan, PhaseExecute, PhaseVerify, PhaseReflect:
		return true
	}
	return false
}

func validGate(gate Gate) bool {
	return gate == GateNone || gate == GateAwaitingApproval
}

func validTaskStatus(status TaskStatus) bool {
	switch status {
	case TaskPending, TaskRunning, TaskComplete, TaskBlocked:
		return true
	}
	return false
}

func validOrchestrationAction(action OrchestrationAction) bool {
	switch action {
	case OrchestrationIdle, OrchestrationRequestApproval, OrchestrationDispatch,
		OrchestrationDispatchAuthor, OrchestrationAdvancePhase,
		OrchestrationWait, OrchestrationRetry, OrchestrationCancel,
		OrchestrationReplan, OrchestrationEscalate, OrchestrationCompleteSession,
		OrchestrationCompact:
		return true
	}
	return false
}

func validEscalationCode(code OrchestrationEscalationCode) bool {
	switch code {
	case EscalationNone, EscalationUnknownState, EscalationInvalidGraph,
		EscalationConflictingLease, EscalationCASExhausted, EscalationPolicyViolation,
		EscalationRetriesExhausted, EscalationHumanIntervention:
		return true
	}
	return false
}

func validSessionStatus(status OrchestrationSessionStatus) bool {
	switch status {
	case OrchestrationSessionRunning, OrchestrationSessionPaused,
		OrchestrationSessionCancelling, OrchestrationSessionComplete,
		OrchestrationSessionFailed:
		return true
	}
	return false
}

func taskOrdinalLess(left, right string) bool {
	leftOrdinal := ordinal(left)
	rightOrdinal := ordinal(right)
	if leftOrdinal == rightOrdinal {
		return left < right
	}
	return leftOrdinal < rightOrdinal
}

func orchestrationSessionExpiry(created time.Time, policy OrchestrationPolicy) time.Time {
	return created.Add(time.Duration(policy.SessionTimeoutSeconds) * time.Second)
}
