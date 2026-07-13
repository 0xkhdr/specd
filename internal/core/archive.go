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
	dst := filepath.Join(SpecdDir(root), "archive", "specs", req.SpecID)
	if raw, err := os.ReadFile(filepath.Join(dst, "archive.json")); err == nil {
		var existing ArchiveRecord
		if json.Unmarshal(raw, &existing) == nil && existing.SpecID == req.SpecID && existing.SuccessorID == req.SuccessorID && existing.Owner == req.Owner && existing.EvidenceRef == req.EvidenceRef {
			return existing, nil
		}
		return ArchiveRecord{}, errors.New("archive replay conflicts with existing record")
	}
	src := filepath.Join(SpecdDir(root), "specs", req.SpecID)
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
	if err := AtomicWrite(filepath.Join(dst, "archive.json"), string(raw)+"\n"); err != nil {
		return ArchiveRecord{}, err
	}
	return record, nil
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
