package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ReleaseLedgerPath is the per-spec append-only release-candidate ledger
// (spec 08 R6.1). Candidates frozen here are immutable and reproducible.
func ReleaseLedgerPath(root, slug string) string {
	return filepath.Join(SpecdDir(root), "specs", slug, "releases.jsonl")
}

// DeploymentLedgerPath is the per-spec append-only deployment-attempt ledger
// (spec 08 R6.2). Attempts append under the spec lock; the evidence gate is
// neutral to its presence (R6.3).
func DeploymentLedgerPath(root, slug string) string {
	return filepath.Join(SpecdDir(root), "specs", slug, "deployments.jsonl")
}

// ReleaseCandidateID is the reproducible content address of a candidate's
// identity (spec 08 R6.1). It hashes only the identity fields — spec revision,
// git HEAD, evidence-set/artifact/bootstrap digests, SBOM/provenance refs — so
// the same inputs always yield the same id. Metadata (schema, created_at) is
// excluded, making a re-freeze of an identical candidate idempotent.
func ReleaseCandidateID(r ReleaseCandidateV1) string {
	parts := []string{
		r.SpecID,
		strconv.Itoa(r.SpecRevision),
		r.GitHead,
		r.TaskEvidenceSetDigest,
		r.ArtifactDigest,
		r.SBOMRef,
		r.ProvenanceRef,
		r.BootstrapDigest,
		r.StateSchema,
	}
	return Digest([]byte(strings.Join(parts, "\x00")))[:16]
}

// readLedger replays a JSONL ledger. A torn *trailing* line — the signature of a
// crash mid-append (appendLedger writes the record and its newline in one fsynced
// write) — is dropped, so a crash yields the prior complete records rather than a
// decode failure (spec 08 R6.2). Corruption anywhere but the final line, or a
// complete record that fails validation, is a real fail-closed error. A missing
// ledger is not an error: delivery ledgers are additive (R6.3).
func readLedger[T any](path string, validate func(T) error) ([]T, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open ledger %s: %w", path, err)
	}
	lines := bytes.Split(data, []byte{'\n'})
	var out []T
	for i, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var rec T
		if err := json.Unmarshal(line, &rec); err != nil {
			if i == len(lines)-1 {
				break // torn final line from a crash mid-append (R6.2)
			}
			return nil, fmt.Errorf("decode ledger %s line %d: %w", path, i+1, err)
		}
		if err := validate(rec); err != nil {
			return nil, fmt.Errorf("invalid ledger %s line %d: %w", path, i+1, err)
		}
		out = append(out, rec)
	}
	return out, nil
}

// appendLedger appends one record with a single fsynced write, so a crash leaves
// either the prior complete record or one complete new record — never a partial
// line (spec 08 R6.2).
func appendLedger[T any](path string, rec T) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("encode ledger record: %w", err)
	}
	return AppendFile(path, string(data)+"\n")
}

// ReadReleases replays the release ledger, dropping a torn final line.
func ReadReleases(path string) ([]ReleaseCandidateV1, error) {
	return readLedger(path, ValidateReleaseCandidate)
}

// AppendRelease validates and durably appends one release candidate.
func AppendRelease(path string, r ReleaseCandidateV1) error {
	if err := ValidateReleaseCandidate(r); err != nil {
		return err
	}
	return appendLedger(path, r)
}

// ReadDeployments replays the deployment ledger, dropping a torn final line.
func ReadDeployments(path string) ([]DeploymentV1, error) {
	return readLedger(path, ValidateDeployment)
}

// AppendDeployment validates and durably appends one deployment attempt.
func AppendDeployment(path string, d DeploymentV1) error {
	if err := ValidateDeployment(d); err != nil {
		return err
	}
	return appendLedger(path, d)
}

// FreezeReleaseCandidate validates and durably appends a candidate under the
// spec lock (spec 08 R6.1). It is idempotent by release_id: an identical
// candidate already frozen is returned unchanged rather than duplicated, so a
// frozen candidate stays immutable. It builds and uploads nothing.
func FreezeReleaseCandidate(root, slug string, r ReleaseCandidateV1) (ReleaseCandidateV1, error) {
	if err := ValidateReleaseCandidate(r); err != nil {
		return ReleaseCandidateV1{}, err
	}
	return WithSpecLock(root, func() (ReleaseCandidateV1, error) {
		path := ReleaseLedgerPath(root, slug)
		existing, err := ReadReleases(path)
		if err != nil {
			return ReleaseCandidateV1{}, err
		}
		for _, e := range existing {
			if e.ReleaseID == r.ReleaseID {
				return e, nil // immutable: already frozen
			}
		}
		if err := AppendRelease(path, r); err != nil {
			return ReleaseCandidateV1{}, err
		}
		return r, nil
	})
}

// AppendDeploymentAttempt allocates the next monotonic attempt for d's
// deployment_id and durably appends it under the spec lock (spec 08 R6.2). The
// read-derive-append runs inside WithSpecLock, so racing writers cannot
// duplicate a (deployment_id, attempt) pair. The caller supplies every field
// except Attempt, which is derived from the ledger's highest attempt for that
// deployment_id.
func AppendDeploymentAttempt(root, slug string, d DeploymentV1) (DeploymentV1, error) {
	return WithSpecLock(root, func() (DeploymentV1, error) {
		path := DeploymentLedgerPath(root, slug)
		existing, err := ReadDeployments(path)
		if err != nil {
			return DeploymentV1{}, err
		}
		attempt := 0
		for _, e := range existing {
			if e.DeploymentID == d.DeploymentID && e.Attempt > attempt {
				attempt = e.Attempt
			}
		}
		d.Attempt = attempt + 1
		if d.Actor == "" {
			d.Actor = recordActor()
		}
		if d.StartedAt == "" {
			d.StartedAt = Clock().Format(time.RFC3339)
		}
		if err := AppendDeployment(path, d); err != nil {
			return DeploymentV1{}, err
		}
		return d, nil
	})
}
