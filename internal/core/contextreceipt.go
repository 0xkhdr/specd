package core

import (
	"encoding/json"
	"slices"
	"sort"
)

const ContextReceiptSchemaVersion = "1"

// ContextReceipt is the host's acknowledgement of the context it actually
// loaded (R3.1).
//
// It is distinct from context.Receipt, which is the harness's own record of
// what it selected. That one answers "what did specd offer?"; this one answers
// "what did the host admit to reading?" — and only the second can be wrong in
// the direction that matters, which is why authority hangs off it.
//
// Content-free by construction: digests, counts, and identities only. No source
// path, selected byte, or prompt fragment crosses this boundary, so a receipt
// can be recorded and audited without carrying the context it describes.
type ContextReceipt struct {
	SchemaVersion  string `json:"schema_version"`
	ManifestDigest string `json:"manifest_digest"`

	// HostID names the process that loaded the context; DriverID names the
	// session driving the spec. They are usually different — a driver may run
	// several hosts — and both are recorded so a receipt is attributable.
	HostID   string `json:"host_id"`
	DriverID string `json:"driver_id"`

	// SuppliedDigests are the required items the host confirms it loaded.
	// MissingDigests are the required items it did not. The host reports what
	// it supplied; the harness derives what is missing, so a host cannot
	// under-report a gap by simply omitting it.
	SuppliedDigests []string `json:"supplied_digests"`
	MissingDigests  []string `json:"missing_digests"`

	// ReportedTokens is the host's own count. It is recorded, never trusted as
	// the harness estimate, and never compared against a budget here.
	ReportedTokens int `json:"reported_tokens"`

	ReceiptDigest string `json:"receipt_digest"`
}

// BuildContextReceipt derives a receipt from what the host claims to have
// loaded. Missing items are computed as the set difference against required,
// not taken from the host: the whole point of R3.2 is that a host cannot
// activate mutable authority by staying quiet about a lane it skipped.
//
// Supplied digests the manifest never required are dropped rather than
// recorded. A host loading extra context is free to do so, but it cannot use
// unrelated material to fill a required lane.
func BuildContextReceipt(manifestDigest, hostID, driverID string, required, supplied []string, reportedTokens int) (ContextReceipt, error) {
	if manifestDigest == "" {
		return ContextReceipt{}, Refuse("RECEIPT_INVALID", "context receipt requires the manifest digest it acknowledges")
	}
	if hostID == "" || driverID == "" {
		return ContextReceipt{}, Refuse("RECEIPT_INVALID", "context receipt requires both a host identity and a driver identity")
	}
	if reportedTokens < 0 {
		return ContextReceipt{}, Refuse("RECEIPT_INVALID", "context receipt cannot report a negative token count")
	}

	suppliedSet := map[string]bool{}
	for _, digest := range supplied {
		suppliedSet[digest] = true
	}
	receipt := ContextReceipt{
		SchemaVersion:  ContextReceiptSchemaVersion,
		ManifestDigest: manifestDigest,
		HostID:         hostID,
		DriverID:       driverID,
		ReportedTokens: reportedTokens,
		// Non-nil so an empty set serializes as [] rather than null: "supplied
		// nothing" and "did not say" must not read alike.
		SuppliedDigests: []string{},
		MissingDigests:  []string{},
	}
	for _, digest := range required {
		if suppliedSet[digest] {
			receipt.SuppliedDigests = append(receipt.SuppliedDigests, digest)
			continue
		}
		receipt.MissingDigests = append(receipt.MissingDigests, digest)
	}
	sort.Strings(receipt.SuppliedDigests)
	sort.Strings(receipt.MissingDigests)
	receipt.SuppliedDigests = slices.Compact(receipt.SuppliedDigests)
	receipt.MissingDigests = slices.Compact(receipt.MissingDigests)
	receipt.ReceiptDigest = contextReceiptDigest(receipt)
	return receipt, nil
}

// Complete reports whether every required lane was acknowledged.
func (r ContextReceipt) Complete() bool { return len(r.MissingDigests) == 0 }

// Validate checks the receipt's own integrity: schema, identities, and a digest
// that still matches its contents.
func (r ContextReceipt) Validate() error {
	if r.SchemaVersion != ContextReceiptSchemaVersion {
		return Refusef("RECEIPT_INVALID", "unsupported context receipt schema_version %q", r.SchemaVersion)
	}
	if r.ManifestDigest == "" || r.HostID == "" || r.DriverID == "" {
		return Refuse("RECEIPT_INVALID", "context receipt is missing its manifest digest or an identity")
	}
	if r.ReceiptDigest == "" || r.ReceiptDigest != contextReceiptDigest(r) {
		return Refuse("RECEIPT_INVALID", "context receipt digest is missing or stale; rebuild the receipt from the current manifest")
	}
	return nil
}

// mutableTools are the tools a withheld packet must not grant. Listed here
// rather than derived, so adding a mutable verb is a deliberate edit to this
// set instead of a silent widening of what a shortfall still permits.
var mutableTools = []string{"verify", "complete-task", "submit"}

// AuthorizeWithReceipt applies R3.2: mutable authority activates only against a
// valid receipt for the current manifest with no required lane missing.
//
// A shortfall withholds rather than errors. The agent keeps the read authority
// it already had and can go load what it missed, which is the recovery; erroring
// would strand a host one file short of being allowed to work.
//
// Withholding strips the mutable tools, not just the write paths. AuthorizeTool
// only walks DeclaredWritePaths when the caller passes paths, so a caller that
// passes none would sail past a paths-only downgrade — the tool has to be denied
// outright for the withholding to hold at the enforcement point. Mode is left
// alone: it is derived from the role and validated against it, so rewriting it
// here would produce a packet that fails its own shape check.
func AuthorizeWithReceipt(authority AuthorityV1, receipt ContextReceipt, manifestDigest string) (AuthorityV1, error) {
	if err := receipt.Validate(); err != nil {
		return AuthorityV1{}, err
	}
	if manifestDigest == "" || receipt.ManifestDigest != manifestDigest {
		return AuthorityV1{}, Refusef("RECEIPT_STALE", "context receipt acknowledges manifest %s but the current manifest is %s", receipt.ManifestDigest, manifestDigest).
			WithRecovery(RefusalActorAgent, "specd context <slug> <task> --json")
	}
	if receipt.Complete() {
		return authority, nil
	}

	withheld := authority
	withheld.DeclaredWritePaths = nil
	withheld.AllowedTools = nil
	for _, tool := range authority.AllowedTools {
		if !slices.Contains(mutableTools, tool.ID) {
			withheld.AllowedTools = append(withheld.AllowedTools, tool)
		}
	}
	for _, tool := range mutableTools {
		if !slices.Contains(withheld.DeniedTools, tool) {
			withheld.DeniedTools = append(withheld.DeniedTools, tool)
		}
	}
	if err := FinalizeAuthority(&withheld); err != nil {
		return AuthorityV1{}, err
	}
	return withheld, nil
}

func contextReceiptDigest(r ContextReceipt) string {
	r.ReceiptDigest = ""
	raw, _ := json.Marshal(r)
	return Digest(raw)
}
