package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type ProgramDecisionAction string

const (
	ProgramDecisionStart    ProgramDecisionAction = "start"
	ProgramDecisionWait     ProgramDecisionAction = "wait"
	ProgramDecisionEscalate ProgramDecisionAction = "escalate"
	ProgramDecisionComplete ProgramDecisionAction = "complete"
)

type ProgramChildSnapshot struct {
	Slug           string     `json:"slug"`
	Status         SpecStatus `json:"status"`
	Wave           int        `json:"wave"`
	Depends        []string   `json:"depends"`
	Complete       bool       `json:"complete"`
	Blocked        bool       `json:"blocked"`
	Active         bool       `json:"active"`
	Escalated      bool       `json:"escalated"`
	ChildSessionID string     `json:"childSessionId,omitempty"`
}

type ProgramChildRuntime struct {
	Active         bool
	Escalated      bool
	ChildSessionID string
}

type ProgramSession struct {
	Version         int                        `json:"version"`
	ParentSessionID string                     `json:"parentSessionId"`
	Status          OrchestrationSessionStatus `json:"status"`
	CreatedAt       string                     `json:"createdAt"`
	UpdatedAt       string                     `json:"updatedAt"`
}

type ProgramSnapshot struct {
	Version      int                          `json:"version"`
	Children     []ProgramChildSnapshot       `json:"children"`
	Capacity     int                          `json:"capacity"`
	ActiveCount  int                          `json:"activeCount"`
	Cycle        []string                     `json:"cycle"`
	Orphans      []struct{ Spec, Dep string } `json:"orphans"`
	CriticalPath []string                     `json:"criticalPath"`
}

type ProgramDecision struct {
	Version int                   `json:"version"`
	Action  ProgramDecisionAction `json:"action"`
	Specs   []string              `json:"specs,omitempty"`
	Reason  string                `json:"reason"`
}

func BuildProgramSnapshot(graph ProgramGraph, active map[string]bool, capacity int) (ProgramSnapshot, error) {
	runtime := make(map[string]ProgramChildRuntime, len(active))
	for slug, isActive := range active {
		runtime[slug] = ProgramChildRuntime{Active: isActive}
	}
	return BuildProgramSnapshotWithRuntime(graph, runtime, capacity)
}

func BuildProgramSnapshotWithRuntime(graph ProgramGraph, runtime map[string]ProgramChildRuntime, capacity int) (ProgramSnapshot, error) {
	if capacity < 1 {
		return ProgramSnapshot{}, fmt.Errorf("program orchestration: capacity must be positive")
	}
	children := make([]ProgramChildSnapshot, 0, len(graph.Specs))
	activeCount := 0
	for _, spec := range graph.Specs {
		childRuntime := runtime[spec.Slug]
		if childRuntime.Active {
			activeCount++
		}
		children = append(children, ProgramChildSnapshot{
			Slug:           spec.Slug,
			Status:         spec.Status,
			Wave:           spec.Wave,
			Depends:        append([]string{}, spec.DependsOn...),
			Complete:       spec.Complete,
			Blocked:        spec.Status == StatusBlocked,
			Active:         childRuntime.Active,
			Escalated:      childRuntime.Escalated,
			ChildSessionID: childRuntime.ChildSessionID,
		})
	}
	return ProgramSnapshot{
		Version:      OrchestrationModelVersion,
		Children:     children,
		Capacity:     capacity,
		ActiveCount:  activeCount,
		Cycle:        append([]string{}, graph.Cycle...),
		Orphans:      append([]struct{ Spec, Dep string }{}, graph.Orphans...),
		CriticalPath: CriticalPath(graph.Dag),
	}, nil
}

func DecideProgram(snapshot ProgramSnapshot) (ProgramDecision, error) {
	if snapshot.Version != OrchestrationModelVersion {
		return ProgramDecision{}, fmt.Errorf("program orchestration: unsupported snapshot version %d", snapshot.Version)
	}
	decision := ProgramDecision{Version: OrchestrationModelVersion}
	if len(snapshot.Cycle) > 0 {
		decision.Action = ProgramDecisionEscalate
		decision.Reason = "program graph has cycle"
		return decision, nil
	}
	if len(snapshot.Orphans) > 0 {
		decision.Action = ProgramDecisionEscalate
		decision.Reason = "program graph has orphan dependency"
		return decision, nil
	}
	for _, child := range snapshot.Children {
		if child.Escalated {
			decision.Action = ProgramDecisionEscalate
			decision.Reason = "child spec escalated"
			decision.Specs = []string{child.Slug}
			return decision, nil
		}
		if child.Blocked {
			decision.Action = ProgramDecisionEscalate
			decision.Reason = "child spec blocked"
			decision.Specs = []string{child.Slug}
			return decision, nil
		}
	}
	if allProgramChildrenComplete(snapshot.Children) {
		decision.Action = ProgramDecisionComplete
		decision.Reason = "all child specs complete"
		return decision, nil
	}
	available := snapshot.Capacity - snapshot.ActiveCount
	if available <= 0 {
		decision.Action = ProgramDecisionWait
		decision.Reason = "program capacity reached"
		return decision, nil
	}
	runnable := programRunnableChildren(snapshot.Children)
	if len(runnable) == 0 {
		decision.Action = ProgramDecisionWait
		decision.Reason = "waiting for dependencies"
		return decision, nil
	}
	if len(runnable) > available {
		runnable = runnable[:available]
	}
	decision.Action = ProgramDecisionStart
	decision.Specs = runnable
	decision.Reason = "frontier ready"
	return decision, nil
}

func allProgramChildrenComplete(children []ProgramChildSnapshot) bool {
	if len(children) == 0 {
		return true
	}
	for _, child := range children {
		if !child.Complete {
			return false
		}
	}
	return true
}

func programRunnableChildren(children []ProgramChildSnapshot) []string {
	bySlug := make(map[string]ProgramChildSnapshot, len(children))
	for _, child := range children {
		bySlug[child.Slug] = child
	}
	out := []string{}
	for _, child := range children {
		if child.Complete || child.Blocked || child.Active {
			continue
		}
		ready := true
		for _, dep := range child.Depends {
			if !bySlug[dep].Complete {
				ready = false
				break
			}
		}
		if ready {
			out = append(out, child.Slug)
		}
	}
	return out
}

type ProgramChildLeaseStatus string

const (
	ProgramChildLeaseActive    ProgramChildLeaseStatus = "active"
	ProgramChildLeaseReleased  ProgramChildLeaseStatus = "released"
	ProgramChildLeaseEscalated ProgramChildLeaseStatus = "escalated"
)

type ProgramChildLease struct {
	Version         int                     `json:"version"`
	ParentSessionID string                  `json:"parentSessionId"`
	ChildSessionID  string                  `json:"childSessionId"`
	Slug            string                  `json:"slug"`
	Status          ProgramChildLeaseStatus `json:"status"`
	AcquiredAt      string                  `json:"acquiredAt"`
	LeaseUntil      string                  `json:"leaseUntil"`
	ReleasedAt      string                  `json:"releasedAt,omitempty"`
	EscalatedAt     string                  `json:"escalatedAt,omitempty"`
}

type ProgramChildStep struct {
	Slug      string                  `json:"slug"`
	SessionID string                  `json:"sessionId"`
	Result    OrchestrationStepResult `json:"result"`
}

type ProgramStepResult struct {
	Snapshot ProgramSnapshot     `json:"snapshot"`
	Decision ProgramDecision     `json:"decision"`
	Started  []ProgramChildLease `json:"started"`
	Stepped  []ProgramChildStep  `json:"stepped"`
	Leases   []ProgramChildLease `json:"leases"`
}

type ProgramCounts struct {
	Total     int `json:"total"`
	Complete  int `json:"complete"`
	Active    int `json:"active"`
	Blocked   int `json:"blocked"`
	Escalated int `json:"escalated"`
}

type ProgramWaveSummary struct {
	Wave     int      `json:"wave"`
	Specs    []string `json:"specs"`
	Complete int      `json:"complete"`
	Active   int      `json:"active"`
}

type ProgramStatusReport struct {
	Session    ProgramSession       `json:"session"`
	Snapshot   ProgramSnapshot      `json:"snapshot"`
	Decision   ProgramDecision      `json:"decision"`
	Counts     ProgramCounts        `json:"counts"`
	Frontier   []string             `json:"frontier"`
	Waves      []ProgramWaveSummary `json:"waves"`
	Escalation []string             `json:"escalation"`
}

var (
	programChildLeaseLocksMu sync.Mutex
	programChildLeaseLocks   = map[string]*sync.Mutex{}
)

func SenseProgramOrchestration(root, parentSessionID string, cfg OrchestrationCfg) (ProgramStatusReport, error) {
	if err := validateACPOpaqueID("program session ID", parentSessionID); err != nil {
		return ProgramStatusReport{}, err
	}
	if err := ValidateOrchestrationConfig(&cfg); err != nil {
		return ProgramStatusReport{}, err
	}
	session, err := LoadProgramSession(root, parentSessionID)
	if err != nil {
		return ProgramStatusReport{}, err
	}
	graph, err := BuildProgram(root, nil)
	if err != nil {
		return ProgramStatusReport{}, err
	}
	runtime, err := programChildRuntime(root)
	if err != nil {
		return ProgramStatusReport{}, err
	}
	snapshot, err := BuildProgramSnapshotWithRuntime(graph, runtime, cfg.Program.MaxConcurrentSpecs)
	if err != nil {
		return ProgramStatusReport{}, err
	}
	decision, err := programStatusDecision(session, snapshot)
	if err != nil {
		return ProgramStatusReport{}, err
	}
	return buildProgramStatusReport(session, snapshot, decision), nil
}

func programStatusDecision(session ProgramSession, snapshot ProgramSnapshot) (ProgramDecision, error) {
	switch session.Status {
	case OrchestrationSessionPaused:
		return programControlDecision(ProgramDecisionWait, "program paused — new child dispatch suspended"), nil
	case OrchestrationSessionCancelling:
		return programControlDecision(ProgramDecisionWait, "program cancelling — cooperative cancel in progress"), nil
	case OrchestrationSessionComplete:
		return programControlDecision(ProgramDecisionComplete, "program session complete"), nil
	case OrchestrationSessionFailed:
		return programControlDecision(ProgramDecisionEscalate, "program session failed — no new child dispatch"), nil
	default:
		return DecideProgram(snapshot)
	}
}

func buildProgramStatusReport(session ProgramSession, snapshot ProgramSnapshot, decision ProgramDecision) ProgramStatusReport {
	counts := ProgramCounts{Total: len(snapshot.Children), Active: snapshot.ActiveCount}
	frontier := programRunnableChildren(snapshot.Children)
	escalation := []string{}
	waveIndex := map[int]int{}
	waves := []ProgramWaveSummary{}
	for _, child := range snapshot.Children {
		if child.Complete {
			counts.Complete++
		}
		if child.Blocked {
			counts.Blocked++
			escalation = append(escalation, child.Slug)
		}
		if child.Escalated {
			counts.Escalated++
			escalation = append(escalation, child.Slug)
		}
		idx, ok := waveIndex[child.Wave]
		if !ok {
			waves = append(waves, ProgramWaveSummary{Wave: child.Wave, Specs: []string{}})
			idx = len(waves) - 1
			waveIndex[child.Wave] = idx
		}
		waves[idx].Specs = append(waves[idx].Specs, child.Slug)
		if child.Complete {
			waves[idx].Complete++
		}
		if child.Active {
			waves[idx].Active++
		}
	}
	if frontier == nil {
		frontier = []string{}
	}
	if escalation == nil {
		escalation = []string{}
	}
	return ProgramStatusReport{Session: session, Snapshot: snapshot, Decision: decision, Counts: counts, Frontier: frontier, Waves: waves, Escalation: escalation}
}

func StepProgramOrchestration(root, parentSessionID string, policy OrchestrationPolicy, cfg OrchestrationCfg) (ProgramStepResult, error) {
	if err := validateACPOpaqueID("parent session ID", parentSessionID); err != nil {
		return ProgramStepResult{}, err
	}
	if err := ValidateOrchestrationPolicy(policy); err != nil {
		return ProgramStepResult{}, err
	}
	if err := ValidateOrchestrationConfig(&cfg); err != nil {
		return ProgramStepResult{}, err
	}
	programSession, err := ensureProgramSession(root, parentSessionID)
	if err != nil {
		return ProgramStepResult{}, err
	}

	graph, err := BuildProgram(root, nil)
	if err != nil {
		return ProgramStepResult{}, err
	}
	if err := releaseCompleteProgramChildren(root, graph); err != nil {
		return ProgramStepResult{}, err
	}
	runtime, err := programChildRuntime(root)
	if err != nil {
		return ProgramStepResult{}, err
	}
	snapshot, err := BuildProgramSnapshotWithRuntime(graph, runtime, cfg.Program.MaxConcurrentSpecs)
	if err != nil {
		return ProgramStepResult{}, err
	}
	result := ProgramStepResult{Snapshot: snapshot, Started: []ProgramChildLease{}, Stepped: []ProgramChildStep{}, Leases: []ProgramChildLease{}}

	switch programSession.Status {
	case OrchestrationSessionPaused:
		result.Decision = programControlDecision(ProgramDecisionWait, "program paused — new child dispatch suspended")
		result.Leases, err = LoadProgramChildLeases(root)
		return result, err
	case OrchestrationSessionCancelling:
		if err := propagateProgramControl(root, parentSessionID, CancelOrchestration); err != nil {
			return ProgramStepResult{}, err
		}
		leases, err := LoadProgramChildLeases(root)
		if err != nil {
			return ProgramStepResult{}, err
		}
		result.Leases = leases
		active := programLeasesToStep(graph, leases, parentSessionID, cfg.Program.MaxConcurrentSpecs)
		if len(active) == 0 {
			if _, err := markProgramSessionStatus(root, parentSessionID, OrchestrationSessionComplete); err != nil {
				return ProgramStepResult{}, err
			}
			result.Decision = programControlDecision(ProgramDecisionComplete, "program cancelled — no active child leases remain")
			return result, nil
		}
		result.Decision = programControlDecision(ProgramDecisionWait, "program cancelling — cooperative cancel propagated")
		for _, lease := range active {
			step, err := StepOrchestration(root, lease.Slug, lease.ChildSessionID, policy, cfg)
			if err != nil {
				return ProgramStepResult{}, err
			}
			result.Stepped = append(result.Stepped, ProgramChildStep{Slug: lease.Slug, SessionID: lease.ChildSessionID, Result: step})
		}
		return result, nil
	case OrchestrationSessionComplete:
		result.Decision = programControlDecision(ProgramDecisionComplete, "program session complete")
		result.Leases, err = LoadProgramChildLeases(root)
		return result, err
	case OrchestrationSessionFailed:
		result.Decision = programControlDecision(ProgramDecisionEscalate, "program session failed — no new child dispatch")
		result.Leases, err = LoadProgramChildLeases(root)
		return result, err
	}

	decision, err := DecideProgram(snapshot)
	if err != nil {
		return ProgramStepResult{}, err
	}
	result.Decision = decision
	if decision.Action == ProgramDecisionStart {
		for _, slug := range decision.Specs {
			lease, err := AcquireProgramChildLease(root, parentSessionID, slug, cfg)
			if err != nil {
				return ProgramStepResult{}, err
			}
			if err := ensureProgramChildSession(root, lease, policy); err != nil {
				return ProgramStepResult{}, err
			}
			result.Started = append(result.Started, lease)
		}
	}

	leases, err := LoadProgramChildLeases(root)
	if err != nil {
		return ProgramStepResult{}, err
	}
	result.Leases = leases
	if decision.Action == ProgramDecisionEscalate {
		if _, err := markProgramSessionStatus(root, parentSessionID, OrchestrationSessionFailed); err != nil {
			return ProgramStepResult{}, err
		}
		return result, nil
	}
	if decision.Action == ProgramDecisionComplete {
		if _, err := markProgramSessionStatus(root, parentSessionID, OrchestrationSessionComplete); err != nil {
			return ProgramStepResult{}, err
		}
		return result, nil
	}
	for _, lease := range programLeasesToStep(graph, leases, parentSessionID, cfg.Program.MaxConcurrentSpecs) {
		step, err := StepOrchestration(root, lease.Slug, lease.ChildSessionID, policy, cfg)
		if err != nil {
			return ProgramStepResult{}, err
		}
		result.Stepped = append(result.Stepped, ProgramChildStep{Slug: lease.Slug, SessionID: lease.ChildSessionID, Result: step})
		if step.Decision.Action == OrchestrationEscalate {
			if _, err := markProgramChildLeaseEscalated(root, parentSessionID, lease.Slug); err != nil {
				return ProgramStepResult{}, err
			}
			if _, err := markProgramSessionStatus(root, parentSessionID, OrchestrationSessionFailed); err != nil {
				return ProgramStepResult{}, err
			}
			break
		}
	}
	result.Leases, err = LoadProgramChildLeases(root)
	return result, err
}

func programControlDecision(action ProgramDecisionAction, reason string) ProgramDecision {
	return ProgramDecision{Version: OrchestrationModelVersion, Action: action, Reason: reason}
}

func PauseProgramOrchestration(root, parentSessionID string) (ProgramSession, error) {
	session, err := updateProgramSession(root, parentSessionID, OrchestrationSessionPaused)
	if err != nil {
		return ProgramSession{}, err
	}
	if err := propagateProgramControl(root, parentSessionID, PauseOrchestration); err != nil {
		return ProgramSession{}, err
	}
	return session, nil
}

func ResumeProgramOrchestration(root, parentSessionID string) (ProgramSession, error) {
	session, err := updateProgramSession(root, parentSessionID, OrchestrationSessionRunning)
	if err != nil {
		return ProgramSession{}, err
	}
	if err := propagateProgramControl(root, parentSessionID, ResumeOrchestration); err != nil {
		return ProgramSession{}, err
	}
	return session, nil
}

func CancelProgramOrchestration(root, parentSessionID string) (ProgramSession, error) {
	session, err := updateProgramSession(root, parentSessionID, OrchestrationSessionCancelling)
	if err != nil {
		return ProgramSession{}, err
	}
	if err := propagateProgramControl(root, parentSessionID, CancelOrchestration); err != nil {
		return ProgramSession{}, err
	}
	return session, nil
}

func LoadProgramSession(root, parentSessionID string) (ProgramSession, error) {
	if err := validateACPOpaqueID("program session ID", parentSessionID); err != nil {
		return ProgramSession{}, err
	}
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return ProgramSession{}, err
	}
	path, err := paths.ProgramSessionPath(parentSessionID)
	if err != nil {
		return ProgramSession{}, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ProgramSession{}, fmt.Errorf("%w: %s", errOrchestrationSessionNotFound, parentSessionID)
	}
	if err != nil {
		return ProgramSession{}, fmt.Errorf("program orchestration: read session: %w", err)
	}
	var session ProgramSession
	if err := decodeACPStrict(raw, &session); err != nil {
		return ProgramSession{}, fmt.Errorf("program orchestration: corrupt session: %w", err)
	}
	if err := validateProgramSession(session); err != nil {
		return ProgramSession{}, fmt.Errorf("program orchestration: corrupt session: %w", err)
	}
	return session, nil
}

func ensureProgramSession(root, parentSessionID string) (ProgramSession, error) {
	session, err := LoadProgramSession(root, parentSessionID)
	if err == nil {
		return session, nil
	}
	if !errors.Is(err, errOrchestrationSessionNotFound) {
		return ProgramSession{}, err
	}
	now := Clock().UTC().Format(time.RFC3339Nano)
	session = ProgramSession{
		Version:         OrchestrationModelVersion,
		ParentSessionID: parentSessionID,
		Status:          OrchestrationSessionRunning,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := saveProgramSession(root, session); err != nil {
		return ProgramSession{}, err
	}
	return session, nil
}

func updateProgramSession(root, parentSessionID string, status OrchestrationSessionStatus) (ProgramSession, error) {
	if !validSessionStatus(status) || status == OrchestrationSessionFailed {
		return ProgramSession{}, fmt.Errorf("program orchestration: invalid program session status %q", status)
	}
	session, err := ensureProgramSession(root, parentSessionID)
	if err != nil {
		return ProgramSession{}, err
	}
	switch session.Status {
	case OrchestrationSessionComplete, OrchestrationSessionFailed:
		return ProgramSession{}, fmt.Errorf("program orchestration: cannot update a %s session", session.Status)
	}
	session.Status = status
	session.UpdatedAt = Clock().UTC().Format(time.RFC3339Nano)
	if err := saveProgramSession(root, session); err != nil {
		return ProgramSession{}, err
	}
	return session, nil
}

func markProgramSessionStatus(root, parentSessionID string, status OrchestrationSessionStatus) (ProgramSession, error) {
	if !validSessionStatus(status) {
		return ProgramSession{}, fmt.Errorf("program orchestration: invalid program session status %q", status)
	}
	session, err := ensureProgramSession(root, parentSessionID)
	if err != nil {
		return ProgramSession{}, err
	}
	session.Status = status
	session.UpdatedAt = Clock().UTC().Format(time.RFC3339Nano)
	if err := saveProgramSession(root, session); err != nil {
		return ProgramSession{}, err
	}
	return session, nil
}

func saveProgramSession(root string, session ProgramSession) error {
	if err := validateProgramSession(session); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("program orchestration: encode session: %w", err)
	}
	raw = append(raw, '\n')
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return err
	}
	path, err := paths.ProgramSessionPath(session.ParentSessionID)
	if err != nil {
		return err
	}
	if err := atomicWritePrivate(path, raw); err != nil {
		return fmt.Errorf("program orchestration: save session: %w", err)
	}
	return nil
}

func validateProgramSession(session ProgramSession) error {
	if session.Version != OrchestrationModelVersion {
		return fmt.Errorf("unsupported version %d", session.Version)
	}
	if err := validateACPOpaqueID("program session ID", session.ParentSessionID); err != nil {
		return err
	}
	if !validSessionStatus(session.Status) {
		return fmt.Errorf("unsupported status %q", session.Status)
	}
	created, err := parseACPTime("program session createdAt", session.CreatedAt)
	if err != nil {
		return err
	}
	updated, err := parseACPTime("program session updatedAt", session.UpdatedAt)
	if err != nil {
		return err
	}
	if updated.Before(created) {
		return fmt.Errorf("updatedAt precedes createdAt")
	}
	return nil
}

func propagateProgramControl(root, parentSessionID string, control func(string, string) (OrchestrationSession, error)) error {
	leases, err := LoadProgramChildLeases(root)
	if err != nil {
		return err
	}
	now := Clock().UTC()
	for _, lease := range leases {
		if lease.ParentSessionID != parentSessionID || !programChildLeaseIsActive(lease, now) {
			continue
		}
		session, err := LoadOrchestrationSession(root, lease.ChildSessionID)
		if errors.Is(err, errOrchestrationSessionNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if session.Status == OrchestrationSessionComplete || session.Status == OrchestrationSessionFailed {
			continue
		}
		if _, err := control(root, lease.ChildSessionID); err != nil {
			return err
		}
	}
	return nil
}

func AcquireProgramChildLease(root, parentSessionID, slug string, cfg OrchestrationCfg) (ProgramChildLease, error) {
	if err := validateACPOpaqueID("parent session ID", parentSessionID); err != nil {
		return ProgramChildLease{}, err
	}
	if err := ValidateSlug(slug); err != nil {
		return ProgramChildLease{}, fmt.Errorf("program orchestration: invalid child slug: %w", err)
	}
	if err := ValidateOrchestrationConfig(&cfg); err != nil {
		return ProgramChildLease{}, err
	}
	var acquired ProgramChildLease
	err := withProgramChildLeaseLock(root, slug, func() error {
		now := Clock().UTC()
		existing, ok, err := loadProgramChildLease(root, slug)
		if err != nil {
			return err
		}
		if ok && programChildLeaseIsActive(existing, now) {
			if existing.ParentSessionID == parentSessionID {
				acquired = existing
				return nil
			}
			return fmt.Errorf("program orchestration: child %s is owned by parent session %s", slug, existing.ParentSessionID)
		}
		childSessionID, err := NewACPID()
		if err != nil {
			return fmt.Errorf("program orchestration: create child session ID: %w", err)
		}
		lease := ProgramChildLease{
			Version:         OrchestrationModelVersion,
			ParentSessionID: parentSessionID,
			ChildSessionID:  childSessionID,
			Slug:            slug,
			Status:          ProgramChildLeaseActive,
			AcquiredAt:      now.Format(time.RFC3339Nano),
			LeaseUntil:      now.Add(time.Duration(cfg.Transport.LeaseSeconds) * time.Second).Format(time.RFC3339Nano),
		}
		if err := saveProgramChildLease(root, lease); err != nil {
			return err
		}
		acquired = lease
		return nil
	})
	return acquired, err
}

func ReleaseProgramChildLease(root, parentSessionID, slug string) (ProgramChildLease, error) {
	if err := validateACPOpaqueID("parent session ID", parentSessionID); err != nil {
		return ProgramChildLease{}, err
	}
	var released ProgramChildLease
	err := withProgramChildLeaseLock(root, slug, func() error {
		lease, ok, err := loadProgramChildLease(root, slug)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("program orchestration: child %s has no lease", slug)
		}
		if lease.ParentSessionID != parentSessionID {
			return fmt.Errorf("program orchestration: child %s is owned by parent session %s", slug, lease.ParentSessionID)
		}
		if lease.Status == ProgramChildLeaseReleased {
			released = lease
			return nil
		}
		lease.Status = ProgramChildLeaseReleased
		lease.ReleasedAt = Clock().UTC().Format(time.RFC3339Nano)
		lease.EscalatedAt = ""
		if err := saveProgramChildLease(root, lease); err != nil {
			return err
		}
		released = lease
		return nil
	})
	return released, err
}

func markProgramChildLeaseEscalated(root, parentSessionID, slug string) (ProgramChildLease, error) {
	var escalated ProgramChildLease
	err := withProgramChildLeaseLock(root, slug, func() error {
		lease, ok, err := loadProgramChildLease(root, slug)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("program orchestration: child %s has no lease", slug)
		}
		if lease.ParentSessionID != parentSessionID {
			return fmt.Errorf("program orchestration: child %s is owned by parent session %s", slug, lease.ParentSessionID)
		}
		if lease.Status == ProgramChildLeaseEscalated {
			escalated = lease
			return nil
		}
		if lease.Status == ProgramChildLeaseReleased {
			return fmt.Errorf("program orchestration: child %s lease already released", slug)
		}
		lease.Status = ProgramChildLeaseEscalated
		lease.EscalatedAt = Clock().UTC().Format(time.RFC3339Nano)
		if err := saveProgramChildLease(root, lease); err != nil {
			return err
		}
		escalated = lease
		return nil
	})
	return escalated, err
}

func LoadProgramChildLeases(root string) ([]ProgramChildLease, error) {
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return nil, err
	}
	dir, err := paths.ProgramChildrenDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []ProgramChildLease{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("program orchestration: read child leases: %w", err)
	}
	leases := make([]ProgramChildLease, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		lease, ok, err := loadProgramChildLease(root, entry.Name())
		if err != nil {
			return nil, err
		}
		if ok {
			leases = append(leases, lease)
		}
	}
	sortProgramChildLeases(leases)
	return leases, nil
}

func releaseCompleteProgramChildren(root string, graph ProgramGraph) error {
	complete := make(map[string]bool, len(graph.Specs))
	for _, spec := range graph.Specs {
		complete[spec.Slug] = spec.Complete
	}
	leases, err := LoadProgramChildLeases(root)
	if err != nil {
		return err
	}
	now := Clock().UTC()
	for _, lease := range leases {
		if !complete[lease.Slug] || lease.Status == ProgramChildLeaseReleased {
			continue
		}
		if !programChildLeaseIsActive(lease, now) && lease.Status != ProgramChildLeaseEscalated {
			continue
		}
		if _, err := releaseProgramChildLeaseAnyParent(root, lease.Slug); err != nil {
			return err
		}
	}
	return nil
}

func releaseProgramChildLeaseAnyParent(root, slug string) (ProgramChildLease, error) {
	var released ProgramChildLease
	err := withProgramChildLeaseLock(root, slug, func() error {
		lease, ok, err := loadProgramChildLease(root, slug)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		if lease.Status == ProgramChildLeaseReleased {
			released = lease
			return nil
		}
		lease.Status = ProgramChildLeaseReleased
		lease.ReleasedAt = Clock().UTC().Format(time.RFC3339Nano)
		lease.EscalatedAt = ""
		if err := saveProgramChildLease(root, lease); err != nil {
			return err
		}
		released = lease
		return nil
	})
	return released, err
}

func activeProgramChildren(root string) (map[string]bool, error) {
	runtime, err := programChildRuntime(root)
	if err != nil {
		return nil, err
	}
	active := map[string]bool{}
	for slug, childRuntime := range runtime {
		if childRuntime.Active {
			active[slug] = true
		}
	}
	return active, nil
}

func programChildRuntime(root string) (map[string]ProgramChildRuntime, error) {
	leases, err := LoadProgramChildLeases(root)
	if err != nil {
		return nil, err
	}
	now := Clock().UTC()
	runtime := map[string]ProgramChildRuntime{}
	for _, lease := range leases {
		childRuntime := runtime[lease.Slug]
		childRuntime.ChildSessionID = lease.ChildSessionID
		if programChildLeaseIsActive(lease, now) {
			childRuntime.Active = true
		}
		if lease.Status == ProgramChildLeaseEscalated {
			childRuntime.Escalated = true
		}
		runtime[lease.Slug] = childRuntime
	}
	return runtime, nil
}

func programLeasesToStep(graph ProgramGraph, leases []ProgramChildLease, parentSessionID string, capacity int) []ProgramChildLease {
	now := Clock().UTC()
	waves := make(map[string]int, len(graph.Specs))
	for _, spec := range graph.Specs {
		waves[spec.Slug] = spec.Wave
	}
	out := make([]ProgramChildLease, 0, len(leases))
	for _, lease := range leases {
		if lease.ParentSessionID == parentSessionID && programChildLeaseIsActive(lease, now) {
			out = append(out, lease)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if waves[out[i].Slug] != waves[out[j].Slug] {
			return waves[out[i].Slug] < waves[out[j].Slug]
		}
		return out[i].Slug < out[j].Slug
	})
	if capacity > 0 && len(out) > capacity {
		out = out[:capacity]
	}
	return out
}

func ensureProgramChildSession(root string, lease ProgramChildLease, policy OrchestrationPolicy) error {
	session, err := LoadOrchestrationSession(root, lease.ChildSessionID)
	if errors.Is(err, errOrchestrationSessionNotFound) {
		_, err = StartOrchestrationSession(root, lease.Slug, lease.ChildSessionID, "program:"+lease.ParentSessionID, policy)
		return err
	}
	if err != nil {
		return err
	}
	if session.Spec != lease.Slug {
		return fmt.Errorf("program orchestration: child session %s belongs to %s, not %s", lease.ChildSessionID, session.Spec, lease.Slug)
	}
	return nil
}

func loadProgramChildLease(root, slug string) (ProgramChildLease, bool, error) {
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return ProgramChildLease{}, false, err
	}
	path, err := paths.ProgramChildLeasePath(slug)
	if err != nil {
		return ProgramChildLease{}, false, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ProgramChildLease{}, false, nil
	}
	if err != nil {
		return ProgramChildLease{}, false, fmt.Errorf("program orchestration: read child lease: %w", err)
	}
	var lease ProgramChildLease
	if err := decodeACPStrict(raw, &lease); err != nil {
		return ProgramChildLease{}, false, fmt.Errorf("program orchestration: corrupt child lease: %w", err)
	}
	if lease.Slug != slug {
		return ProgramChildLease{}, false, fmt.Errorf("program orchestration: child lease identity mismatch")
	}
	if err := validateProgramChildLease(lease); err != nil {
		return ProgramChildLease{}, false, fmt.Errorf("program orchestration: corrupt child lease: %w", err)
	}
	return lease, true, nil
}

func saveProgramChildLease(root string, lease ProgramChildLease) error {
	if err := validateProgramChildLease(lease); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(lease, "", "  ")
	if err != nil {
		return fmt.Errorf("program orchestration: encode child lease: %w", err)
	}
	raw = append(raw, '\n')
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return err
	}
	path, err := paths.ProgramChildLeasePath(lease.Slug)
	if err != nil {
		return err
	}
	if err := atomicWritePrivate(path, raw); err != nil {
		return fmt.Errorf("program orchestration: save child lease: %w", err)
	}
	return nil
}

func validateProgramChildLease(lease ProgramChildLease) error {
	if lease.Version != OrchestrationModelVersion {
		return fmt.Errorf("unsupported version %d", lease.Version)
	}
	if err := validateACPOpaqueID("parent session ID", lease.ParentSessionID); err != nil {
		return err
	}
	if err := validateACPOpaqueID("child session ID", lease.ChildSessionID); err != nil {
		return err
	}
	if err := ValidateSlug(lease.Slug); err != nil {
		return fmt.Errorf("invalid child slug: %w", err)
	}
	if lease.Status != ProgramChildLeaseActive && lease.Status != ProgramChildLeaseReleased && lease.Status != ProgramChildLeaseEscalated {
		return fmt.Errorf("unsupported status %q", lease.Status)
	}
	acquired, err := parseACPTime("program child lease acquiredAt", lease.AcquiredAt)
	if err != nil {
		return err
	}
	leaseUntil, err := parseACPTime("program child lease leaseUntil", lease.LeaseUntil)
	if err != nil {
		return err
	}
	if !leaseUntil.After(acquired) {
		return fmt.Errorf("invalid lease time ordering")
	}
	switch lease.Status {
	case ProgramChildLeaseReleased:
		released, err := parseACPTime("program child lease releasedAt", lease.ReleasedAt)
		if err != nil {
			return err
		}
		if released.Before(acquired) {
			return fmt.Errorf("releasedAt precedes acquiredAt")
		}
		if lease.EscalatedAt != "" {
			return fmt.Errorf("released lease has escalatedAt")
		}
	case ProgramChildLeaseEscalated:
		escalated, err := parseACPTime("program child lease escalatedAt", lease.EscalatedAt)
		if err != nil {
			return err
		}
		if escalated.Before(acquired) {
			return fmt.Errorf("escalatedAt precedes acquiredAt")
		}
		if lease.ReleasedAt != "" {
			return fmt.Errorf("escalated lease has releasedAt")
		}
	default:
		if lease.ReleasedAt != "" || lease.EscalatedAt != "" {
			return fmt.Errorf("active lease has terminal time")
		}
	}
	return nil
}

func programChildLeaseIsActive(lease ProgramChildLease, now time.Time) bool {
	if lease.Status != ProgramChildLeaseActive {
		return false
	}
	leaseUntil, err := time.Parse(time.RFC3339Nano, lease.LeaseUntil)
	if err != nil {
		return false
	}
	return now.UTC().Before(leaseUntil)
}

func sortProgramChildLeases(leases []ProgramChildLease) {
	sort.Slice(leases, func(i, j int) bool {
		if leases[i].Slug != leases[j].Slug {
			return leases[i].Slug < leases[j].Slug
		}
		if leases[i].ParentSessionID != leases[j].ParentSessionID {
			return leases[i].ParentSessionID < leases[j].ParentSessionID
		}
		return leases[i].ChildSessionID < leases[j].ChildSessionID
	})
}

func withProgramChildLeaseLock(root, slug string, fn func() error) error {
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return err
	}
	dir, err := paths.ProgramChildDir(slug)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("program orchestration: create child lease directory: %w", err)
	}
	lockPath := filepath.Join(dir, ".lease.lock")

	programChildLeaseLocksMu.Lock()
	mu := programChildLeaseLocks[lockPath]
	if mu == nil {
		mu = &sync.Mutex{}
		programChildLeaseLocks[lockPath] = mu
	}
	programChildLeaseLocksMu.Unlock()

	mu.Lock()
	defer mu.Unlock()

	deadline := time.Now().Add(acpStoreLockTimeout)
	for {
		lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			if _, writeErr := fmt.Fprintf(lock, "%d %d\n", os.Getpid(), time.Now().UnixMilli()); writeErr != nil {
				lock.Close()
				os.Remove(lockPath)
				return fmt.Errorf("program orchestration: write child lease lock: %w", writeErr)
			}
			if closeErr := lock.Close(); closeErr != nil {
				os.Remove(lockPath)
				return fmt.Errorf("program orchestration: close child lease lock: %w", closeErr)
			}
			break
		}
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("program orchestration: acquire child lease lock: %w", err)
		}
		if isStale(lockPath) {
			if err := os.Remove(lockPath); err == nil || errors.Is(err, os.ErrNotExist) {
				continue
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("program orchestration: child %s is locked", slug)
		}
		time.Sleep(retryInterval)
	}
	defer os.Remove(lockPath)
	return fn()
}
