package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// runRelease implements `specd release candidate <spec>` (spec 08 R6.1). It
// freezes an immutable, reproducible candidate identity — spec revision, git
// HEAD, evidence-set digest, artifact digest, SBOM/provenance refs, bootstrap
// digest — into releases.jsonl under the spec lock. It reads state and appends a
// ledger record only: it builds nothing and uploads nothing. The artifact/SBOM/
// provenance values are external references supplied by the caller, never
// produced here.
func runRelease(root string, args []string, flags map[string]string) error {
	if len(args) < 2 || args[0] != "candidate" {
		return usageError("release")
	}
	slug := args[1]

	artifact := strings.TrimSpace(flags["artifact-digest"])
	sbom := strings.TrimSpace(flags["sbom-ref"])
	provenance := strings.TrimSpace(flags["provenance-ref"])
	for name, value := range map[string]string{"artifact-digest": artifact, "sbom-ref": sbom, "provenance-ref": provenance} {
		if value == "" {
			return fmt.Errorf("%w: release candidate requires --%s", ErrUsage, name)
		}
	}

	state, err := core.LoadState(core.StatePath(root, slug))
	if err != nil {
		return fmt.Errorf("%w: load spec %q: %v", ErrUsage, slug, err)
	}
	bootstrap, err := core.ManagedDigest(root)
	if err != nil {
		return fmt.Errorf("bootstrap digest: %w", err)
	}
	evidenceDigest, err := evidenceSetDigest(root, slug)
	if err != nil {
		return err
	}

	candidate := core.ReleaseCandidateV1{
		Schema:                core.ReleaseCandidateSchemaV1,
		SpecID:                slug,
		SpecRevision:          int(state.Revision),
		GitHead:               gitHead(root),
		TaskEvidenceSetDigest: evidenceDigest,
		ArtifactDigest:        artifact,
		SBOMRef:               sbom,
		ProvenanceRef:         provenance,
		BootstrapDigest:       bootstrap,
		StateSchema:           strconv.Itoa(core.StateSchemaVersion),
		CreatedAt:             core.Clock().Format(time.RFC3339),
	}
	candidate.ReleaseID = core.ReleaseCandidateID(candidate)

	frozen, err := core.FreezeReleaseCandidate(root, slug, candidate)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUsage, err)
	}
	fmt.Printf("release candidate %s frozen for %s\n", frozen.ReleaseID, slug)
	return nil
}

// evidenceSetDigest is a deterministic content address of a spec's task evidence
// set — the ordered (task, HEAD, exit) triples of every recorded verify. A spec
// with no evidence yields the stable digest of the empty set. Pure and offline.
func evidenceSetDigest(root, slug string) (string, error) {
	records, err := core.LoadEvidenceRecords(core.EvidencePath(root, slug))
	if err != nil {
		return "", fmt.Errorf("load evidence: %w", err)
	}
	lines := make([]string, 0, len(records))
	for _, r := range records {
		lines = append(lines, fmt.Sprintf("%s\x00%s\x00%d", r.TaskID, r.GitHead, r.ExitCode))
	}
	sort.Strings(lines)
	return core.Digest([]byte(strings.Join(lines, "\n"))), nil
}
