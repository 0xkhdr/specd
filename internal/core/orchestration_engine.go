package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// errOrchestrationSessionNotFound is the sentinel returned by
// LoadOrchestrationSession when a session.json does not yet exist, so callers can
// distinguish "no session" from a corrupt or unreadable one.
var errOrchestrationSessionNotFound = errors.New("orchestration engine: session not found")

type OrchestrationStepResult struct {
	Snapshot OrchestrationSnapshot `json:"snapshot"`
	Decision OrchestrationDecision `json:"decision"`
	Event    *ACPEnvelope          `json:"event,omitempty"`
}

func StepOrchestration(root, slug, sessionID string, policy OrchestrationPolicy, cfg OrchestrationCfg) (OrchestrationStepResult, error) {
	var result OrchestrationStepResult
	_, err := WithSpecLock[struct{}](root, slug, func() (struct{}, error) {
		snapshot, err := SenseOrchestration(root, slug, sessionID, policy)
		if err != nil {
			return struct{}{}, err
		}
		result.Snapshot = snapshot

		// A step respects the persisted lifecycle. Absent a session file the loop
		// behaves as a plain running controller (back-compatible). Paused stops new
		// dispatch; cancelling issues cooperative directives; terminal sessions idle.
		session, hasSession, err := loadOrchestrationSessionIfExists(root, sessionID)
		if err != nil {
			return struct{}{}, err
		}
		if hasSession {
			switch session.Status {
			case OrchestrationSessionPaused:
				result.Decision = sessionControlDecision(slug, snapshot, OrchestrationWait, "session paused — new dispatch suspended", "paused")
				return struct{}{}, nil
			case OrchestrationSessionCancelling:
				decision, event, err := stepCancelling(root, sessionID, slug, snapshot, cfg)
				if err != nil {
					return struct{}{}, err
				}
				result.Decision = decision
				result.Event = event
				return struct{}{}, nil
			case OrchestrationSessionComplete, OrchestrationSessionFailed:
				result.Decision = sessionControlDecision(slug, snapshot, OrchestrationIdle, fmt.Sprintf("session %s — no further action", session.Status), "terminal")
				return struct{}{}, nil
			}
		}

		decision, err := DecideOrchestration(snapshot, policy)
		if err != nil {
			return struct{}{}, err
		}
		result.Decision = decision
		event, err := recordOrchestrationDecision(root, sessionID, decision, cfg)
		if err != nil {
			return struct{}{}, err
		}
		result.Event = event
		// advance-phase is an effecting decision with no worker dispatch: ratchet
		// the planning phase here, under the same gate `specd approve` enforces.
		if decision.Action == OrchestrationAdvancePhase {
			if _, _, err := AdvancePlanningPhase(root, slug); err != nil {
				return struct{}{}, err
			}
		}
		if hasSession {
			switch decision.Action {
			case OrchestrationCompleteSession:
				if _, err := markOrchestrationSessionStatus(root, sessionID, OrchestrationSessionComplete); err != nil {
					return struct{}{}, err
				}
			case OrchestrationEscalate:
				if _, err := markOrchestrationSessionStatus(root, sessionID, OrchestrationSessionFailed); err != nil {
					return struct{}{}, err
				}
			}
		}
		return struct{}{}, nil
	})
	return result, err
}

// stepCancelling implements cooperative cancellation. Each step issues at most
// one cancel directive to an active lease that has not been directed yet
// (preserving the one-action-per-step invariant); once every active lease has a
// directive it waits for expiry or acknowledgement; once no active leases remain
// it marks the session complete. It never claims host-process termination.
func stepCancelling(root, sessionID, slug string, snapshot OrchestrationSnapshot, cfg OrchestrationCfg) (OrchestrationDecision, *ACPEnvelope, error) {
	store, err := NewACPStore(root)
	if err != nil {
		return OrchestrationDecision{}, nil, err
	}
	events, err := store.readAllEvents(sessionID)
	if err != nil {
		return OrchestrationDecision{}, nil, err
	}
	directed := map[string]bool{}
	for _, event := range events {
		if event.Type == ACPMessageDirective {
			directed[leaseKey(event.Task, event.Attempt)] = true
		}
	}
	for _, lease := range snapshot.ActiveLeases {
		if directed[leaseKey(lease.TaskID, lease.Attempt)] {
			continue
		}
		decision := OrchestrationDecision{
			Version:    OrchestrationModelVersion,
			Action:     OrchestrationCancel,
			Spec:       slug,
			TaskID:     lease.TaskID,
			Attempt:    lease.Attempt,
			Reason:     "session cancelling — cooperative cancel requested",
			Escalation: OrchestrationEscalation{Code: EscalationNone},
		}
		// Keyed by session/task/attempt (not revision) so repeated steps collapse
		// onto the same directive instead of re-issuing it.
		decision.IdempotencyKey = fmt.Sprintf("%s:cancel:%s:%d", snapshot.SessionID, lease.TaskID, lease.Attempt)
		if err := ValidateOrchestrationDecision(decision); err != nil {
			return OrchestrationDecision{}, nil, err
		}
		event, err := recordOrchestrationDecision(root, sessionID, decision, cfg)
		if err != nil {
			return OrchestrationDecision{}, nil, err
		}
		return decision, event, nil
	}

	if len(snapshot.ActiveLeases) > 0 {
		// All active leases already directed: wait for cooperative stop.
		return sessionControlDecision(slug, snapshot, OrchestrationWait, "session cancelling — awaiting lease expiry or acknowledgement", "cancel-wait"), nil, nil
	}

	if _, err := markOrchestrationSessionStatus(root, sessionID, OrchestrationSessionComplete); err != nil {
		return OrchestrationDecision{}, nil, err
	}
	return sessionControlDecision(slug, snapshot, OrchestrationCompleteSession, "session cancelled — no active leases remain", "cancel-complete"), nil, nil
}

// sessionControlDecision builds a valid, recordless decision used by the
// lifecycle branches (pause/cancel-wait/terminal). It carries no escalation and a
// stable idempotency key so the same control state yields the same decision.
func sessionControlDecision(slug string, snapshot OrchestrationSnapshot, action OrchestrationAction, reason, tag string) OrchestrationDecision {
	return OrchestrationDecision{
		Version:        OrchestrationModelVersion,
		Action:         action,
		Spec:           slug,
		Reason:         reason,
		IdempotencyKey: fmt.Sprintf("%s:%d:%s", snapshot.SessionID, snapshot.Revision, tag),
		Escalation:     OrchestrationEscalation{Code: EscalationNone},
	}
}

func leaseKey(taskID string, attempt int) string {
	return fmt.Sprintf("%s-a%d", strings.ToLower(taskID), attempt)
}

func recordOrchestrationDecision(root, sessionID string, decision OrchestrationDecision, cfg OrchestrationCfg) (*ACPEnvelope, error) {
	if err := ValidateOrchestrationDecision(decision); err != nil {
		return nil, err
	}
	switch decision.Action {
	case OrchestrationDispatch, OrchestrationDispatchAuthor, OrchestrationRetry, OrchestrationCancel:
	default:
		return nil, nil
	}
	store, err := NewACPStore(root)
	if err != nil {
		return nil, err
	}
	events, err := store.readAllEvents(sessionID)
	if err != nil {
		return nil, err
	}
	for _, event := range events {
		if event.MessageID == orchestrationDecisionMessageID(decision) {
			return &event, nil
		}
	}
	var messageType ACPMessageType
	var payload any
	switch decision.Action {
	case OrchestrationDispatch, OrchestrationRetry, OrchestrationDispatchAuthor:
		var mission PinkyMission
		var err error
		if decision.Action == OrchestrationDispatchAuthor {
			mission, err = BuildAuthoringMission(root, decision.Spec, sessionID, orchestrationWorkerID(decision), decision.Artifact, cfg)
		} else {
			mission, err = BuildPinkyMission(root, decision.Spec, sessionID, orchestrationWorkerID(decision), decision.TaskID, decision.Attempt, cfg)
		}
		if err != nil {
			return nil, err
		}
		messageType = ACPMessageMission
		payload = ACPMissionPayload{
			DispatchDigest: mission.DispatchDigest,
			Role:           mission.Role,
			ContextCommand: mission.ContextCommand,
			Contract:       mission.Contract,
			Files:          append([]string{}, mission.Files...),
			Acceptance:     mission.Acceptance,
			VerifyCommand:  mission.VerifyCommand,
			Dependencies:   append([]string{}, mission.Dependencies...),
			Authority:      mission.Authority,
		}
	case OrchestrationCancel:
		messageType = ACPMessageDirective
		payload = ACPDirectivePayload{Action: "cancel", Reason: decision.Reason}
	default:
		return nil, nil
	}
	envelope, err := NewACPEnvelope(messageType, payload)
	if err != nil {
		return nil, err
	}
	now := Clock().UTC()
	envelope.MessageID = orchestrationDecisionMessageID(decision)
	envelope.SessionID = sessionID
	envelope.CreatedAt = now.Format(time.RFC3339Nano)
	envelope.ExpiresAt = now.Add(time.Duration(cfg.Transport.MessageTTLSeconds) * time.Second).Format(time.RFC3339Nano)
	envelope.From = "brain"
	envelope.To = "pinky-" + orchestrationWorkerID(decision)
	envelope.Spec = decision.Spec
	envelope.Task = decision.TaskID
	envelope.Attempt = decision.Attempt
	envelope.Decision = &decision
	written, err := store.WriteEvent(envelope)
	if err != nil {
		return nil, fmt.Errorf("orchestration engine: record decision: %w", err)
	}
	return &written, nil
}

func orchestrationDecisionMessageID(decision OrchestrationDecision) string {
	sum := sha256.Sum256([]byte(decision.IdempotencyKey))
	return hex.EncodeToString(sum[:16])
}

func orchestrationWorkerID(decision OrchestrationDecision) string {
	if decision.Attempt < 1 {
		return ""
	}
	return fmt.Sprintf("%s-a%d", strings.ToLower(decision.TaskID), decision.Attempt)
}

// StartOrchestrationSession persists a new running session under session.json.
// It fails closed if a session with the same ID already exists, so a session is
// created exactly once and recovery has a single source of truth.
func StartOrchestrationSession(root, slug, sessionID, owner string, policy OrchestrationPolicy) (OrchestrationSession, error) {
	if err := ValidateOrchestrationPolicy(policy); err != nil {
		return OrchestrationSession{}, err
	}
	if _, err := LoadSpec(root, slug); err != nil {
		return OrchestrationSession{}, err
	}
	store, err := NewACPStore(root)
	if err != nil {
		return OrchestrationSession{}, err
	}
	return WithSpecLock[OrchestrationSession](root, slug, func() (OrchestrationSession, error) {
		var created OrchestrationSession
		err = store.withSessionLock(sessionID, func() error {
			if _, err := LoadOrchestrationSession(root, sessionID); err == nil {
				return fmt.Errorf("orchestration engine: session %s already exists", sessionID)
			} else if !errors.Is(err, errOrchestrationSessionNotFound) {
				return err
			}
			active, err := activeOrchestrationSessionForSpec(root, slug, sessionID)
			if err != nil {
				return err
			}
			if active != nil {
				return fmt.Errorf("orchestration engine: spec %s already has active session %s", slug, active.SessionID)
			}
			now := Clock().UTC()
			session := OrchestrationSession{
				Version:      OrchestrationModelVersion,
				SessionID:    sessionID,
				Spec:         slug,
				Owner:        owner,
				Status:       OrchestrationSessionRunning,
				Policy:       policy,
				CreatedAt:    now.Format(time.RFC3339Nano),
				UpdatedAt:    now.Format(time.RFC3339Nano),
				ExpiresAt:    orchestrationSessionExpiry(now, policy).Format(time.RFC3339Nano),
				LastSequence: 0,
			}
			if err := saveOrchestrationSession(root, session); err != nil {
				return err
			}
			created = session
			return nil
		})
		return created, err
	})
}

// PauseOrchestration suspends new dispatch. It is idempotent (paused→paused) and
// refuses to pause a cancelling or terminal session.
func PauseOrchestration(root, sessionID string) (OrchestrationSession, error) {
	return updateOrchestrationSession(root, sessionID, func(session *OrchestrationSession) error {
		switch session.Status {
		case OrchestrationSessionRunning, OrchestrationSessionPaused:
			session.Status = OrchestrationSessionPaused
			return nil
		default:
			return fmt.Errorf("orchestration engine: cannot pause a %s session", session.Status)
		}
	})
}

// ResumeOrchestration restores dispatch. It is idempotent (running→running) and
// refuses to resume a cancelling or terminal session.
func ResumeOrchestration(root, sessionID string) (OrchestrationSession, error) {
	return updateOrchestrationSession(root, sessionID, func(session *OrchestrationSession) error {
		switch session.Status {
		case OrchestrationSessionPaused, OrchestrationSessionRunning:
			session.Status = OrchestrationSessionRunning
			return nil
		default:
			return fmt.Errorf("orchestration engine: cannot resume a %s session", session.Status)
		}
	})
}

// CancelOrchestration moves a session into cooperative cancellation. The actual
// directives are issued by subsequent steps; this only records intent and is
// idempotent (cancelling→cancelling). Terminal sessions cannot be cancelled.
func CancelOrchestration(root, sessionID string) (OrchestrationSession, error) {
	return updateOrchestrationSession(root, sessionID, func(session *OrchestrationSession) error {
		switch session.Status {
		case OrchestrationSessionRunning, OrchestrationSessionPaused, OrchestrationSessionCancelling:
			session.Status = OrchestrationSessionCancelling
			return nil
		default:
			return fmt.Errorf("orchestration engine: cannot cancel a %s session", session.Status)
		}
	})
}

// RecoverOrchestration rebuilds session state purely from on-disk session.json
// and the committed event log. It reconciles LastSequence to the actual event
// count and re-persists only when it changed, so recovering at any event boundary
// converges to the same state and recovering twice is a no-op.
func RecoverOrchestration(root, sessionID string) (OrchestrationSession, error) {
	store, err := NewACPStore(root)
	if err != nil {
		return OrchestrationSession{}, err
	}
	var recovered OrchestrationSession
	err = store.withSessionLock(sessionID, func() error {
		session, err := LoadOrchestrationSession(root, sessionID)
		if err != nil {
			return err
		}
		count, err := sessionEventCount(store, sessionID)
		if err != nil {
			return err
		}
		if session.LastSequence == count {
			recovered = session
			return nil
		}
		session.LastSequence = count
		session.UpdatedAt = Clock().UTC().Format(time.RFC3339Nano)
		if err := saveOrchestrationSession(root, session); err != nil {
			return err
		}
		recovered = session
		return nil
	})
	return recovered, err
}

// ReclaimExpiredLeases releases active leases whose deadline has passed so the
// underlying tasks become re-dispatchable at the next attempt. It is privileged
// Brain reclamation, distinct from a worker's cooperative ReleaseLease, and
// returns the number of leases reclaimed. Reclaiming twice reclaims nothing.
func ReclaimExpiredLeases(root, sessionID string) (int, error) {
	store, err := NewACPStore(root)
	if err != nil {
		return 0, err
	}
	now := Clock().UTC()
	reclaimed := 0
	err = store.withSessionLock(sessionID, func() error {
		leases, err := store.loadSessionLeases(sessionID)
		if err != nil {
			return err
		}
		sort.Slice(leases, func(i, j int) bool { return leases[i].WorkerID < leases[j].WorkerID })
		for _, lease := range leases {
			if lease.Status != ACPLeaseActive || leaseIsActive(lease, now) {
				continue
			}
			lease.Status = ACPLeaseReleased
			lease.ReleasedAt = now.Format(time.RFC3339Nano)
			if err := store.saveLease(lease); err != nil {
				return err
			}
			reclaimed++
		}
		return nil
	})
	return reclaimed, err
}

// updateOrchestrationSession applies a guarded mutation to a persisted session
// under the session lock, refreshing UpdatedAt and reconciling LastSequence to
// the committed event count.
func updateOrchestrationSession(root, sessionID string, mutate func(*OrchestrationSession) error) (OrchestrationSession, error) {
	store, err := NewACPStore(root)
	if err != nil {
		return OrchestrationSession{}, err
	}
	var updated OrchestrationSession
	err = store.withSessionLock(sessionID, func() error {
		session, err := LoadOrchestrationSession(root, sessionID)
		if err != nil {
			return err
		}
		if err := mutate(&session); err != nil {
			return err
		}
		count, err := sessionEventCount(store, sessionID)
		if err != nil {
			return err
		}
		session.LastSequence = count
		session.UpdatedAt = Clock().UTC().Format(time.RFC3339Nano)
		if err := saveOrchestrationSession(root, session); err != nil {
			return err
		}
		updated = session
		return nil
	})
	return updated, err
}

// markOrchestrationSessionStatus is the unguarded internal transition used by the
// engine itself (e.g. cancelling→complete once all leases drain).
func markOrchestrationSessionStatus(root, sessionID string, status OrchestrationSessionStatus) (OrchestrationSession, error) {
	return updateOrchestrationSession(root, sessionID, func(session *OrchestrationSession) error {
		session.Status = status
		return nil
	})
}

func sessionEventCount(store *ACPStore, sessionID string) (uint64, error) {
	events, err := store.readAllEvents(sessionID)
	if err != nil {
		return 0, err
	}
	return uint64(len(events)), nil
}

// ActiveOrchestrationSessionForSpec returns the running/paused/cancelling
// session for a spec, or nil if none. It lets a driver resume an in-flight
// session instead of failing closed against the one-session-per-spec rule.
func ActiveOrchestrationSessionForSpec(root, slug string) (*OrchestrationSession, error) {
	return activeOrchestrationSessionForSpec(root, slug, "")
}

func activeOrchestrationSessionForSpec(root, slug, exceptSessionID string) (*OrchestrationSession, error) {
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return nil, err
	}
	dir, err := paths.SessionsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("orchestration engine: read sessions: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if err := validateACPOpaqueID("session ID", entry.Name()); err != nil {
			return nil, err
		}
		if entry.Name() == exceptSessionID {
			continue
		}
		session, err := LoadOrchestrationSession(root, entry.Name())
		if errors.Is(err, errOrchestrationSessionNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if session.Spec != slug {
			continue
		}
		switch session.Status {
		case OrchestrationSessionRunning, OrchestrationSessionPaused, OrchestrationSessionCancelling:
			return &session, nil
		}
	}
	return nil, nil
}

func loadOrchestrationSessionIfExists(root, sessionID string) (OrchestrationSession, bool, error) {
	session, err := LoadOrchestrationSession(root, sessionID)
	if errors.Is(err, errOrchestrationSessionNotFound) {
		return OrchestrationSession{}, false, nil
	}
	if err != nil {
		return OrchestrationSession{}, false, err
	}
	return session, true, nil
}

// saveOrchestrationSession validates and atomically writes session.json with
// private permissions.
func saveOrchestrationSession(root string, session OrchestrationSession) error {
	if err := ValidateOrchestrationSession(session); err != nil {
		return err
	}
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return err
	}
	path, err := paths.SessionPath(session.SessionID)
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("orchestration engine: encode session: %w", err)
	}
	raw = append(raw, '\n')
	if err := atomicWritePrivate(path, raw); err != nil {
		return fmt.Errorf("orchestration engine: persist session: %w", err)
	}
	return nil
}

// LoadOrchestrationSession reads and validates session.json, returning
// errOrchestrationSessionNotFound when none exists yet.
func LoadOrchestrationSession(root, sessionID string) (OrchestrationSession, error) {
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return OrchestrationSession{}, err
	}
	path, err := paths.SessionPath(sessionID)
	if err != nil {
		return OrchestrationSession{}, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return OrchestrationSession{}, fmt.Errorf("%w: %s", errOrchestrationSessionNotFound, sessionID)
	}
	if err != nil {
		return OrchestrationSession{}, fmt.Errorf("orchestration engine: read session: %w", err)
	}
	var session OrchestrationSession
	if err := decodeACPStrict(raw, &session); err != nil {
		return OrchestrationSession{}, fmt.Errorf("orchestration engine: corrupt session: %w", err)
	}
	if err := ValidateOrchestrationSession(session); err != nil {
		return OrchestrationSession{}, fmt.Errorf("orchestration engine: corrupt session: %w", err)
	}
	return session, nil
}
