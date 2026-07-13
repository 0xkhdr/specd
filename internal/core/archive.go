package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const ArchiveSchemaV1 = 1

type ArchiveRequest struct {
	SpecID      string
	SuccessorID string
	Owner       string
	EvidenceRef string
}

type ArchivedFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type ArchiveRecord struct {
	SchemaVersion int            `json:"schema_version"`
	SpecID        string         `json:"spec_id"`
	SuccessorID   string         `json:"successor_id"`
	Owner         string         `json:"owner"`
	EvidenceRef   string         `json:"evidence_ref"`
	Files         []ArchivedFile `json:"files"`
	ManifestHash  string         `json:"manifest_hash"`
}

var archiveAtomicWrite = AtomicWrite

func ArchiveRecordPath(root, slug string) string {
	return filepath.Join(SpecdDir(root), "archive", "specs", slug, "archive.json")
}

// ArchiveSpec relocates immutable history from active discovery and writes a
// content-addressed audit manifest. Repeating the same request is idempotent.
func ArchiveSpec(root string, req ArchiveRequest) (ArchiveRecord, error) {
	for name, value := range map[string]string{"spec id": req.SpecID, "successor id": req.SuccessorID, "owner": req.Owner, "evidence ref": req.EvidenceRef} {
		if strings.TrimSpace(value) == "" {
			return ArchiveRecord{}, fmt.Errorf("archive %s is required", name)
		}
	}
	if req.SpecID == req.SuccessorID {
		return ArchiveRecord{}, errors.New("archive successor must differ from retired spec")
	}
	if err := validateIncidentRef("archive evidence", req.EvidenceRef); err != nil {
		return ArchiveRecord{}, err
	}
	if err := ValidateSlug(req.SpecID); err != nil {
		return ArchiveRecord{}, fmt.Errorf("invalid archive spec: %w", err)
	}
	if err := ValidateSlug(req.SuccessorID); err != nil {
		return ArchiveRecord{}, fmt.Errorf("invalid archive successor: %w", err)
	}
	dst := filepath.Join(SpecdDir(root), "archive", "specs", req.SpecID)
	if raw, err := os.ReadFile(filepath.Join(dst, "archive.json")); err == nil {
		var existing ArchiveRecord
		if json.Unmarshal(raw, &existing) == nil && existing.SpecID == req.SpecID && existing.SuccessorID == req.SuccessorID && existing.Owner == req.Owner && existing.EvidenceRef == req.EvidenceRef {
			if err := verifyArchiveRecord(dst, existing); err != nil {
				return ArchiveRecord{}, err
			}
			return existing, nil
		}
		return ArchiveRecord{}, errors.New("archive replay conflicts with existing record")
	}
	src := filepath.Join(SpecdDir(root), "specs", req.SpecID)
	state, err := LoadState(StatePath(root, req.SpecID))
	if err != nil {
		return ArchiveRecord{}, fmt.Errorf("archive source state: %w", err)
	}
	if state.Status != StatusComplete {
		return ArchiveRecord{}, errors.New("archive source must be complete")
	}
	evidence, err := LoadEvidence(EvidencePath(root, req.SpecID))
	if err != nil {
		return ArchiveRecord{}, fmt.Errorf("archive source evidence: %w", err)
	}
	if len(evidence) == 0 {
		return ArchiveRecord{}, errors.New("archive source requires passing commit-pinned evidence")
	}
	for task, record := range evidence {
		if record.ExitCode != 0 || ResolveGitCommit(root, record.GitHead) != nil {
			return ArchiveRecord{}, fmt.Errorf("archive source evidence for %s is not passing and commit-pinned", task)
		}
	}
	files, err := archiveHashes(src)
	if err != nil {
		return ArchiveRecord{}, err
	}
	record := ArchiveRecord{SchemaVersion: ArchiveSchemaV1, SpecID: req.SpecID, SuccessorID: req.SuccessorID, Owner: req.Owner, EvidenceRef: req.EvidenceRef, Files: files}
	unsigned, err := json.Marshal(record)
	if err != nil {
		return ArchiveRecord{}, err
	}
	record.ManifestHash = Digest(unsigned)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return ArchiveRecord{}, err
	}
	if err := os.Rename(src, dst); err != nil {
		return ArchiveRecord{}, fmt.Errorf("archive relocate: %w", err)
	}
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return ArchiveRecord{}, err
	}
	if err := archiveAtomicWrite(filepath.Join(dst, "archive.json"), string(raw)+"\n"); err != nil {
		if rollbackErr := os.Rename(dst, src); rollbackErr != nil {
			return ArchiveRecord{}, fmt.Errorf("archive manifest: %v; rollback: %w", err, rollbackErr)
		}
		return ArchiveRecord{}, err
	}
	return record, nil
}

func VerifyArchive(root, slug string) (ArchiveRecord, error) {
	if err := ValidateSlug(slug); err != nil {
		return ArchiveRecord{}, err
	}
	dir := filepath.Join(SpecdDir(root), "archive", "specs", slug)
	raw, err := os.ReadFile(filepath.Join(dir, "archive.json"))
	if err != nil {
		return ArchiveRecord{}, err
	}
	var record ArchiveRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return ArchiveRecord{}, err
	}
	if record.SpecID != slug {
		return ArchiveRecord{}, errors.New("archive manifest spec identity mismatch")
	}
	if err := verifyArchiveRecord(dir, record); err != nil {
		return ArchiveRecord{}, err
	}
	return record, nil
}

// RestoreArchive rolls a just-created archive back when coupled program commit fails.
func RestoreArchive(root, slug string) error {
	if err := ValidateSlug(slug); err != nil {
		return err
	}
	src := filepath.Join(SpecdDir(root), "archive", "specs", slug)
	dst := filepath.Join(SpecdDir(root), "specs", slug)
	if err := os.Remove(filepath.Join(src, "archive.json")); err != nil {
		return err
	}
	return os.Rename(src, dst)
}

func verifyArchiveRecord(dir string, record ArchiveRecord) error {
	unsigned := record
	unsigned.ManifestHash = ""
	raw, err := json.Marshal(unsigned)
	if err != nil {
		return err
	}
	if Digest(raw) != record.ManifestHash {
		return errors.New("archive manifest digest mismatch")
	}
	files, err := archiveHashesIgnoringManifest(dir)
	if err != nil {
		return err
	}
	if len(files) != len(record.Files) {
		return errors.New("archive file set mismatch")
	}
	for i := range files {
		if files[i] != record.Files[i] {
			return fmt.Errorf("archive file digest mismatch for %s", files[i].Path)
		}
	}
	return nil
}

func archiveHashesIgnoringManifest(root string) ([]ArchivedFile, error) {
	files, err := archiveHashes(root)
	if err != nil {
		return nil, err
	}
	out := files[:0]
	for _, file := range files {
		if file.Path != "archive.json" {
			out = append(out, file)
		}
	}
	return out, nil
}

func archiveHashes(root string) ([]ArchivedFile, error) {
	var out []ArchivedFile
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("archive refuses symlink %s", path)
		}
		if entry.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out = append(out, ArchivedFile{Path: filepath.ToSlash(rel), SHA256: Digest(raw)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
