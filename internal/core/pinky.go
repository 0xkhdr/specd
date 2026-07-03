package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	contextpkg "github.com/0xkhdr/specd/internal/context"
)

// PinkyMission is the dispatch payload sent to a worker for one task attempt:
// identity (session/worker/spec/task/attempt), the deadline and heartbeat
// cadence, the role and authoring contract/acceptance/verify text pulled from
// tasks.md, the context manifest, and an optional resume checkpoint.
type PinkyMission struct {
	Version         int                               `json:"version"`
	SessionID       string                            `json:"sessionId"`
	WorkerID        string                            `json:"workerId"`
	Spec            string                            `json:"spec"`
	TaskID          string                            `json:"taskId"`
	Attempt         int                               `json:"attempt"`
	Deadline        string                            `json:"deadline"`
	HeartbeatEvery  int                               `json:"heartbeatEverySeconds"`
	Role            string                            `json:"role"`
	Title           string                            `json:"title"`
	ContextCommand  string                            `json:"contextCommand"`
	ContextManifest contextpkg.MissionContextManifest `json:"contextManifest"`
	Contract        string                            `json:"contract"`
	Files           []string                          `json:"files"`
	Acceptance      string                            `json:"acceptance"`
	VerifyCommand   string                            `json:"verifyCommand"`
	Dependencies    []string                          `json:"dependencies"`
	Requirements    []int                             `json:"requirements"`
	Authority       ACPAuthority                      `json:"authority"`
	DispatchDigest  string                            `json:"dispatchDigest"`
	// Resume, when present, carries the prior mid-task checkpoint a worker is
	// being handed so it continues from recorded progress instead of restarting
	// (R1, R4). It is omitempty and excluded from the dispatch digest, so a
	// fresh-dispatch and a resume mission for the same (task, attempt) share a
	// digest and a non-resume mission stays byte-identical to today.
	Resume *PinkyResume `json:"resume,omitempty"`
	// Tier is the routing tier the Brain dispatched this mission at (V4), carried
	// through the ACP handoff so a downstream host can attribute cost. Omitempty
	// and excluded from the dispatch digest — a mission without it is byte-identical.
	Tier string `json:"tier,omitempty"`
	// Handoff records an inter-role handoff (P3.3): when a prior worker (e.g. a
	// scout) passes its work to this mission's role (e.g. a craftsman), it names
	// the origin role, the reason, and the artifacts produced. Nil for a fresh
	// dispatch. Omitempty and excluded from the dispatch digest.
	Handoff *ACPHandoff `json:"handoff,omitempty"`
}

// PinkyResume is the resume payload threaded into a mission when the Brain
// decides resume-from-checkpoint: the progress the prior worker reached, its
// free-form working notes, the files it already touched, the git head it
// observed, and the context manifest it was given. The brief turns this into a
// "do not restart" header so the fresh worker continues the same work.
type PinkyResume struct {
	ProgressPercent int      `json:"progressPercent"`
	WorkingNotes    string   `json:"workingNotes,omitempty"`
	ChangedFiles    []string `json:"changedFiles,omitempty"`
	GitHead         string   `json:"gitHead,omitempty"`
	PriorManifest   string   `json:"priorManifest,omitempty"`
	// ContextDelta, when present, is the per-file reference/reload verdict from
	// diffing the latest context snapshot against the working tree (R2). It lets
	// the resumed worker reload only what changed. nil when snapshots are off or
	// none exist, so a resume without snapshots renders identically to before.
	ContextDelta *contextpkg.SnapshotDiff `json:"contextDelta,omitempty"`
}

// PinkyClaim pairs a mission with the lease a worker acquired for it.
type PinkyClaim struct {
	Mission PinkyMission `json:"mission"`
	Lease   ACPLease     `json:"lease"`
}

// BuildPinkyMission assembles and validates a PinkyMission for the given
// task and attempt: it loads the spec and task metadata, fills in the
// contract/acceptance/verify fields from tasks.md, builds the context
// manifest, computes the dispatch deadline and digest, and — when the
// resilience checkpoint feature is enabled and a matching checkpoint exists —
// attaches a Resume payload so a fresh worker continues prior progress.
func BuildPinkyMission(root, slug, sessionID, workerID, taskID string, attempt int, cfg OrchestrationCfg) (PinkyMission, error) {
	if err := validateACPOpaqueID("session ID", sessionID); err != nil {
		return PinkyMission{}, err
	}
	if err := validateACPRuntimeSegment("worker ID", workerID); err != nil {
		return PinkyMission{}, err
	}
	loaded, err := LoadSpec(root, slug)
	if err != nil {
		return PinkyMission{}, err
	}
	task, ok := loaded.State.Tasks[taskID]
	if !ok {
		return PinkyMission{}, fmt.Errorf("pinky: unknown task %s", taskID)
	}
	if attempt < 1 {
		return PinkyMission{}, fmt.Errorf("pinky: attempt must be positive")
	}
	deadline := Clock().UTC().Add(time.Duration(cfg.Transport.MessageTTLSeconds) * time.Second)
	mission := PinkyMission{
		Version:        OrchestrationModelVersion,
		SessionID:      sessionID,
		WorkerID:       workerID,
		Spec:           slug,
		TaskID:         task.ID,
		Attempt:        attempt,
		Deadline:       deadline.Format(time.RFC3339Nano),
		HeartbeatEvery: cfg.Transport.HeartbeatSeconds,
		Role:           task.Role,
		Title:          task.Title,
		ContextCommand: fmt.Sprintf("specd context %s", slug),
		Contract:       taskField(loaded.Doc, task.ID, "contract"),
		Files:          splitCSV(taskField(loaded.Doc, task.ID, "files")),
		Acceptance:     taskField(loaded.Doc, task.ID, "acceptance"),
		VerifyCommand:  taskField(loaded.Doc, task.ID, "verify"),
		Dependencies:   append([]string{}, task.Depends...),
		Requirements:   append([]int{}, task.Requirements...),
		Authority: ACPAuthority{
			ReadOnly:       IsReadonlyRole(task.Role),
			AllowedActions: pinkyAllowedActions(task.Role),
		},
	}
	sort.Strings(mission.Files)
	sort.Strings(mission.Dependencies)
	mission.ContextManifest = BuildMissionContextManifest(mission, specArtifactReader(root, slug))
	// Thread a matching mid-task checkpoint into the mission so a resumed worker
	// continues from recorded progress (Req 5). Gated on the resilience flag and
	// strict (task, attempt) match, so a fresh dispatch — or any mission with the
	// feature off — is byte-identical to today. The digest excludes Resume, so a
	// resumed mission keeps the same dispatch digest as its fresh counterpart.
	if cfg.Resilience != nil && cfg.Resilience.CheckpointEnabled {
		if rec, ok, err := loadCheckpointForAttempt(root, sessionID, task.ID, attempt); err != nil {
			return PinkyMission{}, err
		} else if ok {
			mission.Resume = &PinkyResume{
				ProgressPercent: rec.ProgressPercent,
				WorkingNotes:    rec.WorkingNotes,
				ChangedFiles:    append([]string{}, rec.ChangedFiles...),
				GitHead:         rec.GitHead,
				PriorManifest:   rec.ContextManifest,
			}
			// Attach the O(changed-files) context delta when a snapshot exists, so
			// the resumed worker reloads only what moved (R2). Guarded: absent
			// snapshot or feature off leaves ContextDelta nil — a no-op hook.
			if cfg.Resilience.ContextSnapshotEnabled {
				if delta, ok2, derr := latestContextSnapshotDiff(root, sessionID); derr != nil {
					return PinkyMission{}, derr
				} else if ok2 {
					mission.Resume.ContextDelta = delta
				}
			}
		}
	}
	mission.DispatchDigest = pinkyMissionDigest(mission)
	if err := validatePinkyMission(mission); err != nil {
		return PinkyMission{}, err
	}
	return mission, nil
}

// ClaimPinkyMission validates a mission and acquires a lease for it in the
// ACP store, returning both as a PinkyClaim.
func ClaimPinkyMission(root string, mission PinkyMission, cfg OrchestrationCfg) (PinkyClaim, error) {
	if err := validatePinkyMission(mission); err != nil {
		return PinkyClaim{}, err
	}
	store, err := NewACPStore(root)
	if err != nil {
		return PinkyClaim{}, err
	}
	deadline, err := parseACPTime("deadline", mission.Deadline)
	if err != nil {
		return PinkyClaim{}, err
	}
	lease, err := store.ClaimLease(
		mission.SessionID,
		mission.WorkerID,
		mission.Spec,
		mission.TaskID,
		mission.Attempt,
		time.Duration(cfg.Transport.LeaseSeconds)*time.Second,
		deadline,
	)
	if err != nil {
		return PinkyClaim{}, err
	}
	return PinkyClaim{Mission: mission, Lease: lease}, nil
}

// HeartbeatPinkyClaim renews a worker's lease for the given attempt, extending
// it by the configured lease duration.
func HeartbeatPinkyClaim(root, sessionID, workerID string, attempt int, cfg OrchestrationCfg) (ACPLease, error) {
	store, err := NewACPStore(root)
	if err != nil {
		return ACPLease{}, err
	}
	return store.RenewLease(sessionID, workerID, attempt, time.Duration(cfg.Transport.LeaseSeconds)*time.Second)
}

// ReleasePinkyClaim releases a worker's lease for the given attempt, freeing
// the task for re-dispatch.
func ReleasePinkyClaim(root, sessionID, workerID string, attempt int) error {
	store, err := NewACPStore(root)
	if err != nil {
		return err
	}
	return store.ReleaseLease(sessionID, workerID, attempt)
}

// SuspendPinkyClaim suspends a worker's lease for `resumeAfterSeconds`, holding
// the task instead of failing it (R3). The heartbeat interval is added as a
// buffer so the worker has slack to call resume, and the cumulative-suspension
// cap is resolved from config (default 600s).
func SuspendPinkyClaim(root, sessionID, workerID string, attempt int, reason string, resumeAfterSeconds int, cfg OrchestrationCfg) (ACPLease, error) {
	store, err := NewACPStore(root)
	if err != nil {
		return ACPLease{}, err
	}
	return store.SuspendLease(
		sessionID, workerID, attempt, reason,
		time.Duration(resumeAfterSeconds)*time.Second,
		time.Duration(cfg.Transport.HeartbeatSeconds)*time.Second,
		time.Duration(cfg.EffectiveMaxSuspendSeconds())*time.Second,
	)
}

// ResumePinkyClaim returns a suspended lease to active and emits a `resume` ACP
// event recording how long the worker was suspended (R3, Req 3). The lease keeps
// the same attempt and task, so the worker continues rather than re-claiming.
func ResumePinkyClaim(root, sessionID, workerID string, attempt int, cfg OrchestrationCfg) (ACPLease, time.Duration, error) {
	store, err := NewACPStore(root)
	if err != nil {
		return ACPLease{}, 0, err
	}
	lease, suspendedFor, err := store.ResumeLease(
		sessionID, workerID, attempt,
		time.Duration(cfg.Transport.LeaseSeconds)*time.Second,
	)
	if err != nil {
		return ACPLease{}, 0, err
	}
	payload := ACPResumePayload{SuspendedSeconds: int(suspendedFor / time.Second)}
	envelope, err := NewACPEnvelope(ACPMessageResume, payload)
	if err != nil {
		return ACPLease{}, 0, err
	}
	messageID, err := NewACPID()
	if err != nil {
		return ACPLease{}, 0, err
	}
	now := Clock().UTC()
	envelope.MessageID = messageID
	envelope.SessionID = lease.SessionID
	envelope.CreatedAt = now.Format(time.RFC3339Nano)
	envelope.ExpiresAt = now.Add(time.Duration(cfg.Transport.MessageTTLSeconds) * time.Second).Format(time.RFC3339Nano)
	envelope.From = "pinky-" + workerID
	envelope.To = "brain"
	envelope.Spec = lease.Spec
	envelope.Task = lease.Task
	envelope.Attempt = attempt
	if _, err := store.WriteEvent(envelope); err != nil {
		return ACPLease{}, 0, fmt.Errorf("resume: append event: %w", err)
	}
	return lease, suspendedFor, nil
}

func taskField(doc ParsedTasks, taskID, key string) string {
	for _, task := range doc.Tasks {
		if task.ID == taskID {
			return task.Meta[key]
		}
	}
	return ""
}

// SplitCSV splits a comma-separated meta field (e.g. a task's `files:` list)
// into trimmed, non-empty tokens, discarding the "—"/"-" placeholders. It is the
// exported entry point command surfaces use to feed ContextRequest.Files.
func SplitCSV(value string) []string { return splitCSV(value) }

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" && part != "—" && part != "-" {
			out = append(out, part)
		}
	}
	return out
}

func pinkyAllowedActions(role string) []string {
	if IsReadonlyRole(role) {
		return []string{"read", "verify", "report"}
	}
	return []string{"read", "edit", "verify", "report"}
}

func pinkyMissionDigest(m PinkyMission) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		m.Spec,
		m.TaskID,
		m.Role,
		m.Contract,
		m.Acceptance,
		m.VerifyCommand,
		strings.Join(m.Files, "\x00"),
		strings.Join(m.Dependencies, "\x00"),
	}, "\x1f")))
	return hex.EncodeToString(sum[:])
}

func validatePinkyMission(m PinkyMission) error {
	if m.Version != OrchestrationModelVersion {
		return fmt.Errorf("pinky: unsupported mission version %d", m.Version)
	}
	if err := validateACPOpaqueID("session ID", m.SessionID); err != nil {
		return err
	}
	if err := validateACPRuntimeSegment("worker ID", m.WorkerID); err != nil {
		return err
	}
	if err := ValidateSlug(m.Spec); err != nil {
		return err
	}
	if !acpTaskIDRE.MatchString(m.TaskID) || m.Attempt < 1 {
		return fmt.Errorf("pinky: invalid task attempt")
	}
	if !IsValidRole(m.Role) {
		return fmt.Errorf("pinky: invalid role %q", m.Role)
	}
	if _, err := parseACPTime("deadline", m.Deadline); err != nil {
		return err
	}
	if m.HeartbeatEvery < minHeartbeatSeconds || m.HeartbeatEvery > maxHeartbeatSeconds {
		return fmt.Errorf("pinky: heartbeat interval outside policy bounds")
	}
	if err := validateMissionContextManifest(m.ContextManifest, true); err != nil {
		return err
	}
	if err := validateACPText("contract", m.Contract, true); err != nil {
		return err
	}
	if err := validateACPText("acceptance", m.Acceptance, true); err != nil {
		return err
	}
	if err := validateACPPaths("files", m.Files); err != nil {
		return err
	}
	if err := validateACPTaskIDs("dependencies", m.Dependencies); err != nil {
		return err
	}
	if !acpDigestRE.MatchString(m.DispatchDigest) || m.DispatchDigest != pinkyMissionDigest(m) {
		return fmt.Errorf("pinky: invalid dispatchDigest")
	}
	if err := ValidateHandoff(m.Handoff); err != nil {
		return err
	}
	return nil
}
