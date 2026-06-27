package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	ACPRuntimeIDMaxBytes  = 128
	acpEventSequenceWidth = 20
)

var acpRuntimeSegmentRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// ACPRuntimePaths derives paths beneath .specd/runtime after validating every
// untrusted segment and rejecting symlinks in existing runtime components.
type ACPRuntimePaths struct {
	root       string
	runtimeDir string
}

func NewACPRuntimePaths(root string) (ACPRuntimePaths, error) {
	if root == "" {
		return ACPRuntimePaths{}, fmt.Errorf("acp runtime: project root is required")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return ACPRuntimePaths{}, fmt.Errorf("acp runtime: resolve project root: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return ACPRuntimePaths{}, fmt.Errorf("acp runtime: resolve project root: %w", err)
	}
	paths := ACPRuntimePaths{
		root:       resolvedRoot,
		runtimeDir: filepath.Join(resolvedRoot, ".specd", "runtime"),
	}
	if err := paths.validate(paths.runtimeDir); err != nil {
		return ACPRuntimePaths{}, err
	}
	return paths, nil
}

func RuntimeDir(root string) (string, error) {
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return "", err
	}
	return paths.RuntimeDir()
}

func (p ACPRuntimePaths) RuntimeDir() (string, error) {
	return p.checked(p.runtimeDir)
}

func (p ACPRuntimePaths) SessionsDir() (string, error) {
	return p.join("sessions")
}

func (p ACPRuntimePaths) ArchivesDir() (string, error) {
	return p.join("archives")
}

func (p ACPRuntimePaths) SessionDir(sessionID string) (string, error) {
	if err := validateACPOpaqueID("session ID", sessionID); err != nil {
		return "", err
	}
	return p.join("sessions", sessionID)
}

func (p ACPRuntimePaths) SessionPath(sessionID string) (string, error) {
	return p.sessionJoin(sessionID, "session.json")
}

func (p ACPRuntimePaths) EventsDir(sessionID string) (string, error) {
	return p.sessionJoin(sessionID, "events")
}

func (p ACPRuntimePaths) EventPath(sessionID string, sequence uint64, messageID string) (string, error) {
	name, err := ACPEventFilename(sequence, messageID)
	if err != nil {
		return "", err
	}
	return p.sessionJoin(sessionID, "events", name)
}

func (p ACPRuntimePaths) WorkersDir(sessionID string) (string, error) {
	return p.sessionJoin(sessionID, "workers")
}

func (p ACPRuntimePaths) WorkerDir(sessionID, workerID string) (string, error) {
	if err := validateACPRuntimeSegment("worker ID", workerID); err != nil {
		return "", err
	}
	return p.sessionJoin(sessionID, "workers", workerID)
}

func (p ACPRuntimePaths) LeasePath(sessionID, workerID string) (string, error) {
	return p.workerJoin(sessionID, workerID, "lease.json")
}

func (p ACPRuntimePaths) CursorPath(sessionID, workerID string) (string, error) {
	return p.workerJoin(sessionID, workerID, "cursor.json")
}

func (p ACPRuntimePaths) ArtifactsDir(sessionID string) (string, error) {
	return p.sessionJoin(sessionID, "artifacts")
}

func (p ACPRuntimePaths) ArtifactPath(sessionID, artifactID string) (string, error) {
	if err := validateACPRuntimeSegment("artifact ID", artifactID); err != nil {
		return "", err
	}
	return p.sessionJoin(sessionID, "artifacts", artifactID)
}

// CheckpointDir returns the validated directory holding a session's checkpoint
// records: sessions/<id>/checkpoints. It sits beside events/, workers/, and
// artifacts/ in the runtime layout.
func (p ACPRuntimePaths) CheckpointDir(sessionID string) (string, error) {
	return p.sessionJoin(sessionID, "checkpoints")
}

// CheckpointPath returns the deterministic record path for one task attempt:
// checkpoints/<task>-<attempt>.json. The filename is keyed on (taskID, attempt)
// so a re-issued attempt overwrites its own checkpoint rather than accumulating
// duplicates, and the attempt-guard can compare attempts by filename. Task IDs
// are uppercase (e.g. T1) so they use the ACP task-ID grammar, not the lowercase
// runtime-segment validator.
func (p ACPRuntimePaths) CheckpointPath(sessionID, taskID string, attempt int) (string, error) {
	if !acpTaskIDRE.MatchString(taskID) {
		return "", fmt.Errorf("acp runtime: invalid checkpoint task ID")
	}
	if attempt < 1 {
		return "", fmt.Errorf("acp runtime: checkpoint attempt must be >= 1")
	}
	name := fmt.Sprintf("%s-%d.json", taskID, attempt)
	return p.sessionJoin(sessionID, "checkpoints", name)
}

// ContextSnapshotDir returns the validated directory holding a session's
// per-turn context snapshots: sessions/<id>/context-snapshots. It sits beside
// events/, workers/, artifacts/, and checkpoints/ in the runtime layout (R2).
func (p ACPRuntimePaths) ContextSnapshotDir(sessionID string) (string, error) {
	return p.sessionJoin(sessionID, "context-snapshots")
}

// ContextSnapshotPath returns the deterministic snapshot path for one turn:
// context-snapshots/<turn>.json. Keying on the turn means re-emitting a turn's
// snapshot overwrites it rather than accumulating duplicates.
func (p ACPRuntimePaths) ContextSnapshotPath(sessionID string, turn int) (string, error) {
	if turn < 0 {
		return "", fmt.Errorf("acp runtime: context snapshot turn must be >= 0")
	}
	name := fmt.Sprintf("%d.json", turn)
	return p.sessionJoin(sessionID, "context-snapshots", name)
}

func (p ACPRuntimePaths) ProgramSessionsDir() (string, error) {
	return p.join("program", "sessions")
}

func (p ACPRuntimePaths) ProgramSessionPath(sessionID string) (string, error) {
	if err := validateACPOpaqueID("program session ID", sessionID); err != nil {
		return "", err
	}
	return p.join("program", "sessions", sessionID+".json")
}

func (p ACPRuntimePaths) ProgramChildrenDir() (string, error) {
	return p.join("program", "children")
}

func (p ACPRuntimePaths) ProgramChildDir(slug string) (string, error) {
	if err := ValidateSlug(slug); err != nil {
		return "", fmt.Errorf("acp runtime: invalid program child: %w", err)
	}
	return p.join("program", "children", slug)
}

func (p ACPRuntimePaths) ProgramChildLeasePath(slug string) (string, error) {
	dir, err := p.ProgramChildDir(slug)
	if err != nil {
		return "", err
	}
	return p.checked(filepath.Join(dir, "lease.json"))
}

func (p ACPRuntimePaths) MissionsDir() (string, error) {
	return p.join("missions")
}

// MissionPath returns the canonical, validated runtime path for one mission
// record: missions/<slug>-<taskID>-<attempt>.json. The filename is deterministic
// given (slug, taskID, attempt) so a re-issued attempt overwrites its own record
// rather than creating a duplicate, and two specs (or two attempts of one task)
// never share a filename. Task IDs are uppercase (e.g. T1), so they use the ACP
// task-ID grammar rather than the lowercase runtime-segment validator.
func (p ACPRuntimePaths) MissionPath(slug, taskID string, attempt int) (string, error) {
	if err := ValidateSlug(slug); err != nil {
		return "", fmt.Errorf("acp runtime: invalid mission slug: %w", err)
	}
	if !acpTaskIDRE.MatchString(taskID) {
		return "", fmt.Errorf("acp runtime: invalid mission task ID")
	}
	if attempt < 1 {
		return "", fmt.Errorf("acp runtime: mission attempt must be >= 1")
	}
	name := fmt.Sprintf("%s-%s-%d.json", slug, taskID, attempt)
	return p.join("missions", name)
}

func (p ACPRuntimePaths) ArchivePath(sessionID string) (string, error) {
	if err := validateACPOpaqueID("session ID", sessionID); err != nil {
		return "", err
	}
	return p.join("archives", sessionID)
}

func (p ACPRuntimePaths) sessionJoin(sessionID string, elems ...string) (string, error) {
	if err := validateACPOpaqueID("session ID", sessionID); err != nil {
		return "", err
	}
	return p.join(append([]string{"sessions", sessionID}, elems...)...)
}

func (p ACPRuntimePaths) workerJoin(sessionID, workerID string, elems ...string) (string, error) {
	if err := validateACPRuntimeSegment("worker ID", workerID); err != nil {
		return "", err
	}
	return p.sessionJoin(sessionID, append([]string{"workers", workerID}, elems...)...)
}

func (p ACPRuntimePaths) join(elems ...string) (string, error) {
	path := filepath.Join(append([]string{p.runtimeDir}, elems...)...)
	return p.checked(path)
}

func (p ACPRuntimePaths) checked(path string) (string, error) {
	if err := p.validate(path); err != nil {
		return "", err
	}
	return path, nil
}

func (p ACPRuntimePaths) validate(path string) error {
	if p.root == "" || p.runtimeDir == "" {
		return fmt.Errorf("acp runtime: uninitialized path helper")
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("acp runtime: resolve path: %w", err)
	}
	if !pathWithin(p.runtimeDir, pathAbs) {
		return fmt.Errorf("acp runtime: path escapes runtime directory")
	}

	rel, err := filepath.Rel(p.root, pathAbs)
	if err != nil {
		return fmt.Errorf("acp runtime: resolve relative path: %w", err)
	}
	current := p.root
	for _, component := range strings.Split(rel, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		switch {
		case err == nil && info.Mode()&os.ModeSymlink != 0:
			return fmt.Errorf("acp runtime: path component %s is a symlink", current)
		case err == nil:
			continue
		case os.IsNotExist(err):
			return nil
		default:
			return fmt.Errorf("acp runtime: inspect path component %s: %w", current, err)
		}
	}
	return nil
}

func validateACPOpaqueID(name, value string) error {
	if len(value) != 32 || !acpIDRE.MatchString(value) {
		return fmt.Errorf("acp runtime: invalid %s", name)
	}
	return nil
}

func validateACPRuntimeSegment(name, value string) error {
	if value == "" || len(value) > ACPRuntimeIDMaxBytes || !acpRuntimeSegmentRE.MatchString(value) {
		return fmt.Errorf("acp runtime: invalid %s", name)
	}
	if filepath.IsAbs(value) || filepath.Base(value) != value || strings.ContainsAny(value, `/\`) {
		return fmt.Errorf("acp runtime: invalid %s", name)
	}
	return nil
}

func pathWithin(base, path string) bool {
	rel, err := filepath.Rel(base, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func ACPEventFilename(sequence uint64, messageID string) (string, error) {
	if sequence == 0 {
		return "", fmt.Errorf("acp runtime: event sequence must be greater than zero")
	}
	if err := validateACPOpaqueID("message ID", messageID); err != nil {
		return "", err
	}
	return strings.Repeat("0", acpEventSequenceWidth-len(strconv.FormatUint(sequence, 10))) +
		strconv.FormatUint(sequence, 10) + "-" + messageID + ".json", nil
}
