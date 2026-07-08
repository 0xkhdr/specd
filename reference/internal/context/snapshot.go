package contextpkg

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ContextSnapshot is the file-level record of exactly what a turn loaded into a
// worker: the full mission manifest plus, for each loaded file, its content
// SHA256 and line range, and digests of the steering set and memory.md (R2). On
// resume a worker diffs the SHAs and reloads only what changed, turning
// re-contextualization from O(all files) into O(changed files).
type ContextSnapshot struct {
	Version        int                    `json:"version"`
	Turn           int                    `json:"turn"`
	Phase          string                 `json:"phase"`
	Task           string                 `json:"task"`
	Manifest       MissionContextManifest `json:"manifest"`
	LoadedFiles    []LoadedFile           `json:"loadedFiles"`
	SteeringDigest string                 `json:"steeringDigest,omitempty"`
	MemoryDigest   string                 `json:"memoryDigest,omitempty"`
	Timestamp      string                 `json:"timestamp"`
}

// LoadedFile records one file the turn loaded: its repo-relative path, the
// SHA256 of its content, and the inclusive 1-based line range that was loaded
// (whole-file loads use [1, lineCount]; an empty file uses [0, 0]).
type LoadedFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Lines  [2]int `json:"lines"`
}

// SnapshotDiff is the per-file changed/unchanged verdict a resuming worker uses
// to reload only what actually moved (R2, Req 3).
type SnapshotDiff struct {
	Unchanged       []string `json:"unchanged"`
	Changed         []string `json:"changed"`
	SteeringChanged bool     `json:"steeringChanged"`
	MemoryChanged   bool     `json:"memoryChanged"`
}

// CanonicalSnapshotJSON serializes a snapshot to stable, indented bytes with a
// trailing newline, mirroring core's CanonicalOrchestrationJSON. LoadedFiles are
// sorted by path so two snapshots of the same turn are byte-identical regardless
// of manifest iteration order.
func CanonicalSnapshotJSON(snapshot ContextSnapshot) ([]byte, error) {
	if err := ValidateContextSnapshot(snapshot); err != nil {
		return nil, err
	}
	files := append([]LoadedFile(nil), snapshot.LoadedFiles...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	snapshot.LoadedFiles = files
	raw, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("context snapshot: encode: %w", err)
	}
	return append(raw, '\n'), nil
}

// ValidateContextSnapshot rejects structurally malformed snapshots: a bad
// version, missing turn/task, or a loaded file with an empty path, a non-hex
// SHA256, or an inverted line range.
func ValidateContextSnapshot(snapshot ContextSnapshot) error {
	if snapshot.Version != ManifestVersion {
		return fmt.Errorf("context snapshot: unsupported version %d", snapshot.Version)
	}
	if snapshot.Turn < 0 {
		return fmt.Errorf("context snapshot: turn must be non-negative")
	}
	if snapshot.Task == "" {
		return fmt.Errorf("context snapshot: task is required")
	}
	for _, file := range snapshot.LoadedFiles {
		if file.Path == "" {
			return fmt.Errorf("context snapshot: loaded file has empty path")
		}
		if !isHexSHA256(file.SHA256) {
			return fmt.Errorf("context snapshot: loaded file %s has invalid sha256", file.Path)
		}
		if file.Lines[0] < 0 || file.Lines[1] < file.Lines[0] {
			return fmt.Errorf("context snapshot: loaded file %s has invalid line range", file.Path)
		}
	}
	return nil
}

// BuildContextSnapshot hashes every manifest item that names an on-disk file and
// computes the steering/memory digests, producing the snapshot for one turn.
// Manifest items without a path (commands) or whose file is missing/unreadable
// are skipped — only what was actually loadable is recorded. The caller supplies
// `now` so the timestamp stays a deterministic function of the caller's clock.
func BuildContextSnapshot(root string, turn int, phase, task string, manifest MissionContextManifest, now time.Time) (ContextSnapshot, error) {
	loaded := make([]LoadedFile, 0, len(manifest.Items))
	seen := map[string]bool{}
	for _, item := range manifest.Items {
		if item.Path == "" || seen[item.Path] {
			continue
		}
		sha, lines, ok, err := hashFile(root, item.Path)
		if err != nil {
			return ContextSnapshot{}, err
		}
		if !ok {
			continue
		}
		seen[item.Path] = true
		loaded = append(loaded, LoadedFile{Path: item.Path, SHA256: sha, Lines: lines})
	}
	sort.Slice(loaded, func(i, j int) bool { return loaded[i].Path < loaded[j].Path })

	steering, err := steeringDigest(root)
	if err != nil {
		return ContextSnapshot{}, err
	}
	memory, err := memoryDigest(root)
	if err != nil {
		return ContextSnapshot{}, err
	}
	snapshot := ContextSnapshot{
		Version:        ManifestVersion,
		Turn:           turn,
		Phase:          phase,
		Task:           task,
		Manifest:       manifest,
		LoadedFiles:    loaded,
		SteeringDigest: steering,
		MemoryDigest:   memory,
		Timestamp:      now.UTC().Format(time.RFC3339Nano),
	}
	if err := ValidateContextSnapshot(snapshot); err != nil {
		return ContextSnapshot{}, err
	}
	return snapshot, nil
}

// DiffContextSnapshot compares a snapshot against the current working tree,
// returning which loaded files are unchanged (reference, do not reload) versus
// changed (reload), and whether the steering set or memory.md moved. A file that
// has since vanished or become unreadable counts as changed: re-contextualizing
// it is the safe degradation.
func DiffContextSnapshot(snapshot ContextSnapshot, root string) (SnapshotDiff, error) {
	diff := SnapshotDiff{Unchanged: []string{}, Changed: []string{}}
	for _, file := range snapshot.LoadedFiles {
		sha, _, ok, err := hashFile(root, file.Path)
		if err != nil {
			return SnapshotDiff{}, err
		}
		if ok && sha == file.SHA256 {
			diff.Unchanged = append(diff.Unchanged, file.Path)
		} else {
			diff.Changed = append(diff.Changed, file.Path)
		}
	}
	sort.Strings(diff.Unchanged)
	sort.Strings(diff.Changed)

	steering, err := steeringDigest(root)
	if err != nil {
		return SnapshotDiff{}, err
	}
	memory, err := memoryDigest(root)
	if err != nil {
		return SnapshotDiff{}, err
	}
	diff.SteeringChanged = steering != snapshot.SteeringDigest
	diff.MemoryChanged = memory != snapshot.MemoryDigest
	return diff, nil
}

// hashFile returns the SHA256 (hex) and 1-based inclusive line range of a
// repo-relative file. ok=false (no error) means the file is absent — the caller
// treats that as "not loaded" when building and as "changed" when diffing.
func hashFile(root, relPath string) (string, [2]int, bool, error) {
	content, err := os.ReadFile(filepath.Join(root, relPath))
	if os.IsNotExist(err) {
		return "", [2]int{}, false, nil
	}
	if err != nil {
		return "", [2]int{}, false, fmt.Errorf("context snapshot: read %s: %w", relPath, err)
	}
	sum := sha256.Sum256(content)
	lines := [2]int{0, 0}
	if len(content) > 0 {
		lines = [2]int{1, lineCount(content)}
	}
	return hex.EncodeToString(sum[:]), lines, true, nil
}

// steeringDigest hashes the concatenation of the steering set — every *.md in
// .specd/steering except memory.md, in sorted order — so any byte change to the
// steering corpus flips the digest. memory.md is digested separately.
func steeringDigest(root string) (string, error) {
	dir := filepath.Join(root, ".specd", "steering")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("context snapshot: read steering dir: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || name == "memory.md" || !strings.HasSuffix(name, ".md") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	hasher := sha256.New()
	for _, name := range names {
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", fmt.Errorf("context snapshot: read steering %s: %w", name, err)
		}
		// Frame each file by name+length so concatenation is unambiguous (no two
		// distinct file sets can collide on the same byte stream).
		fmt.Fprintf(hasher, "%s\x00%d\x00", name, len(content))
		hasher.Write(content)
	}
	if len(names) == 0 {
		return "", nil
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// memoryDigest hashes .specd/steering/memory.md, returning "" when absent.
func memoryDigest(root string) (string, error) {
	content, err := os.ReadFile(filepath.Join(root, ".specd", "steering", "memory.md"))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("context snapshot: read memory.md: %w", err)
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]), nil
}

func lineCount(content []byte) int {
	count := 1
	for _, b := range content {
		if b == '\n' {
			count++
		}
	}
	// A trailing newline does not start a new line.
	if len(content) > 0 && content[len(content)-1] == '\n' {
		count--
	}
	return count
}

func isHexSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
