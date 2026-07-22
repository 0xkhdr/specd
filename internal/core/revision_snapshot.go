package core

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
)

// ReopenableArtifacts are the spec artifacts an operator may reopen (R4.1).
// The list is also the allowlist every revision path is built from, so an
// artifact name can never carry traversal into the snapshot directory.
var ReopenableArtifacts = []string{"requirements", "design", "tasks"}

// ReopenableArtifact reports whether artifact names one of the three spec
// artifacts a revision may be taken of.
func ReopenableArtifact(artifact string) bool {
	for _, known := range ReopenableArtifacts {
		if artifact == known {
			return true
		}
	}
	return false
}

// RevisionSnapshot is one content-addressed copy of an artifact's bytes as they
// stood before a reopen created the next draft version. Snapshots are immutable
// and additive: a digest is written once and never rewritten (R4.1).
type RevisionSnapshot struct {
	Artifact string `json:"artifact"`
	Digest   string `json:"digest"`
	// Path is spec-relative, so a plan stays a pure value with no root in it.
	Path string `json:"path"`
}

// RevisionSnapshotRelPath is the spec-relative address of one artifact revision.
func RevisionSnapshotRelPath(artifact, digest string) string {
	return path.Join("revisions", artifact, digest+".md")
}

// SpecArtifactPath resolves a spec artifact inside .specd/, refusing an unknown
// artifact name or an invalid slug before any path is built.
func SpecArtifactPath(root, slug, artifact string) (string, error) {
	if err := validArtifactTarget(slug, artifact); err != nil {
		return "", err
	}
	return SafeJoin(SpecdDir(root), path.Join("specs", slug, artifact+".md"))
}

// RevisionSnapshotPath resolves one snapshot inside the spec's revision
// directory. Every component is validated, and SafeJoin refuses anything that
// would escape the base (design: paths normalize within the revision directory).
func RevisionSnapshotPath(root, slug, artifact, digest string) (string, error) {
	if err := validArtifactTarget(slug, artifact); err != nil {
		return "", err
	}
	if !hexDigest(digest) {
		return "", fmt.Errorf("revision digest %q is not a sha256 hex digest", digest)
	}
	return SafeJoin(SpecdDir(root), path.Join("specs", slug, RevisionSnapshotRelPath(artifact, digest)))
}

// SnapshotArtifactRevision preserves the artifact's current bytes under their
// own content address and returns the snapshot. It is idempotent: a snapshot
// that already holds exactly these bytes is reused. A file at the same digest
// holding different bytes cannot happen without corruption or a hand-edit, so it
// fails closed rather than overwriting immutable history.
func SnapshotArtifactRevision(root, slug, artifact string) (RevisionSnapshot, error) {
	src, err := SpecArtifactPath(root, slug, artifact)
	if err != nil {
		return RevisionSnapshot{}, err
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		return RevisionSnapshot{}, fmt.Errorf("snapshot %s: %w", artifact, err)
	}
	digest := Digest(raw)
	dst, err := RevisionSnapshotPath(root, slug, artifact, digest)
	if err != nil {
		return RevisionSnapshot{}, err
	}
	snapshot := RevisionSnapshot{Artifact: artifact, Digest: digest, Path: RevisionSnapshotRelPath(artifact, digest)}
	switch existing, readErr := os.ReadFile(dst); {
	case readErr == nil && bytes.Equal(existing, raw):
		return snapshot, nil
	case readErr == nil:
		return RevisionSnapshot{}, fmt.Errorf("revision snapshot %s does not hold the bytes it is addressed by (%s); repair the revision directory before reopening", dst, digest)
	case !errors.Is(readErr, os.ErrNotExist):
		return RevisionSnapshot{}, readErr
	}
	if err := AtomicWrite(dst, string(raw)); err != nil {
		return RevisionSnapshot{}, fmt.Errorf("snapshot %s: %w", artifact, err)
	}
	return snapshot, nil
}

func validArtifactTarget(slug, artifact string) error {
	if err := ValidateSlug(slug); err != nil {
		return err
	}
	if !ReopenableArtifact(artifact) {
		return fmt.Errorf("unknown spec artifact %q; reopenable artifacts are %s", artifact, strings.Join(ReopenableArtifacts, ", "))
	}
	return nil
}

func hexDigest(digest string) bool {
	if len(digest) != 64 {
		return false
	}
	for _, r := range digest {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
