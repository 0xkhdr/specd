package core

import (
	"testing"
	"time"
)

const (
	digestA = "aaaa1111"
	digestB = "bbbb2222"
	digestC = "cccc3333"
)

func writeAuthority(t *testing.T) AuthorityV1 {
	t.Helper()
	task := TaskRow{ID: "T1", Role: "craftsman", DeclaredFiles: []string{"internal/core/thing.go"}, Verify: "go test ./..."}
	authority, err := BuildAuthority(task, "actor", "worker", "demo", "execute", "abc123", "policy", "none",
		time.Now(), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if authority.Mode != "write" {
		t.Fatalf("fixture authority is %q, want write", authority.Mode)
	}
	return authority
}

// R3.1: the receipt binds the manifest, the supplied and missing lanes, the
// host token count, and both identities.
func TestContextReceiptRecordsEveryRequiredField(t *testing.T) {
	receipt, err := BuildContextReceipt("manifest-1", "host-1", "driver-1",
		[]string{digestA, digestB}, []string{digestA, digestB}, 1234)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if receipt.ManifestDigest != "manifest-1" {
		t.Errorf("manifest_digest = %q", receipt.ManifestDigest)
	}
	if receipt.HostID != "host-1" || receipt.DriverID != "driver-1" {
		t.Errorf("identities not recorded: %+v", receipt)
	}
	if receipt.ReportedTokens != 1234 {
		t.Errorf("reported_tokens = %d, want 1234", receipt.ReportedTokens)
	}
	if len(receipt.SuppliedDigests) != 2 {
		t.Errorf("supplied = %v, want both", receipt.SuppliedDigests)
	}
	if !receipt.Complete() {
		t.Errorf("receipt with every lane supplied reports incomplete: %+v", receipt)
	}
	if err := receipt.Validate(); err != nil {
		t.Errorf("freshly built receipt does not validate: %v", err)
	}
}

// The harness derives the missing set. A host cannot hide a gap by omitting it
// from its own report, which is the attack R3.2 exists to stop.
func TestContextReceiptDerivesMissingRatherThanTrustingHost(t *testing.T) {
	receipt, err := BuildContextReceipt("manifest-1", "host-1", "driver-1",
		[]string{digestA, digestB, digestC}, []string{digestA}, 10)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if receipt.Complete() {
		t.Fatal("receipt missing two required lanes reports complete")
	}
	if len(receipt.MissingDigests) != 2 {
		t.Fatalf("missing = %v, want two lanes", receipt.MissingDigests)
	}
}

// Loading extra context is allowed, but unrelated material cannot fill a
// required lane.
func TestContextReceiptIgnoresUnrequiredSupplied(t *testing.T) {
	receipt, err := BuildContextReceipt("manifest-1", "host-1", "driver-1",
		[]string{digestA}, []string{digestB, digestC}, 10)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if receipt.Complete() {
		t.Fatal("unrelated context satisfied a required lane")
	}
	if len(receipt.SuppliedDigests) != 0 {
		t.Fatalf("supplied = %v, want none of the required lanes", receipt.SuppliedDigests)
	}
}

func TestContextReceiptRefusesIncompleteConstruction(t *testing.T) {
	cases := []struct {
		name                   string
		manifest, host, driver string
		tokens                 int
	}{
		{"no manifest digest", "", "host-1", "driver-1", 0},
		{"no host identity", "manifest-1", "", "driver-1", 0},
		{"no driver identity", "manifest-1", "host-1", "", 0},
		{"negative tokens", "manifest-1", "host-1", "driver-1", -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := BuildContextReceipt(tc.manifest, tc.host, tc.driver, nil, nil, tc.tokens)
			if err == nil {
				t.Fatal("accepted an unattributable receipt")
			}
			if refusal, ok := AsRefusal(err); !ok || refusal.Code != "RECEIPT_INVALID" {
				t.Fatalf("got %v, want RECEIPT_INVALID", err)
			}
		})
	}
}

// R3.2: a missing required lane keeps authority read-only.
func TestAuthorizeWithReceiptWithholdsWriteOnMissingLane(t *testing.T) {
	authority := writeAuthority(t)
	receipt, err := BuildContextReceipt("manifest-1", "host-1", "driver-1",
		[]string{digestA, digestB}, []string{digestA}, 10)
	if err != nil {
		t.Fatal(err)
	}

	got, err := AuthorizeWithReceipt(authority, receipt, "manifest-1")
	if err != nil {
		t.Fatalf("withholding must downgrade, not error: %v", err)
	}
	if len(got.DeclaredWritePaths) != 0 {
		t.Fatalf("write paths survived the withholding: %v", got.DeclaredWritePaths)
	}
	// The downgrade must be real at the enforcement point, not just in the
	// reported mode.
	err = AuthorizeTool(got, "verify", []string{"internal/core/thing.go"}, time.Now(), "execute", true)
	if err == nil {
		t.Fatal("withheld authority still authorized a mutable tool")
	}
	// The hole a paths-only withholding would leave: no declared paths means
	// the write-path loop never runs, so the tool itself must be denied.
	if err := AuthorizeTool(got, "verify", nil, time.Now(), "execute", true); err == nil {
		t.Fatal("withheld authority authorized a mutable tool when the caller declared no paths")
	}
	// Read authority survives, so the agent can go load what it missed.
	if err := AuthorizeTool(got, "context", nil, time.Now(), "execute", false); err != nil {
		t.Fatalf("withholding also revoked read authority: %v", err)
	}
}

// R3.2: a complete receipt leaves authority untouched.
func TestAuthorizeWithReceiptGrantsWriteOnCompleteReceipt(t *testing.T) {
	authority := writeAuthority(t)
	receipt, err := BuildContextReceipt("manifest-1", "host-1", "driver-1",
		[]string{digestA, digestB}, []string{digestA, digestB}, 10)
	if err != nil {
		t.Fatal(err)
	}
	got, err := AuthorizeWithReceipt(authority, receipt, "manifest-1")
	if err != nil {
		t.Fatalf("complete receipt refused: %v", err)
	}
	if got.Mode != "write" {
		t.Fatalf("mode = %q, want write on a complete receipt", got.Mode)
	}
	if err := AuthorizeTool(got, "verify", []string{"internal/core/thing.go"}, time.Now(), "execute", true); err != nil {
		t.Fatalf("complete receipt did not authorize a declared write: %v", err)
	}
}

// A receipt for a different manifest is stale: the host acknowledged context
// that is no longer the context it would be acting on.
func TestAuthorizeWithReceiptRefusesStaleManifest(t *testing.T) {
	authority := writeAuthority(t)
	receipt, err := BuildContextReceipt("manifest-old", "host-1", "driver-1",
		[]string{digestA}, []string{digestA}, 10)
	if err != nil {
		t.Fatal(err)
	}
	_, err = AuthorizeWithReceipt(authority, receipt, "manifest-new")
	if err == nil {
		t.Fatal("stale receipt was accepted")
	}
	refusal, ok := AsRefusal(err)
	if !ok || refusal.Code != "RECEIPT_STALE" {
		t.Fatalf("got %v, want RECEIPT_STALE", err)
	}
	if refusal.RecoveryCommand == "" {
		t.Fatalf("refusal names no recovery: %+v", refusal)
	}
}

// A forged or edited receipt fails its own digest check, so a host cannot hand
// back a receipt claiming lanes it never loaded.
func TestAuthorizeWithReceiptRefusesTamperedReceipt(t *testing.T) {
	authority := writeAuthority(t)
	receipt, err := BuildContextReceipt("manifest-1", "host-1", "driver-1",
		[]string{digestA, digestB}, []string{digestA}, 10)
	if err != nil {
		t.Fatal(err)
	}
	// Claim the missing lane was supplied after the fact.
	receipt.MissingDigests = nil
	receipt.SuppliedDigests = []string{digestA, digestB}

	_, err = AuthorizeWithReceipt(authority, receipt, "manifest-1")
	if err == nil {
		t.Fatal("tampered receipt granted authority")
	}
	if refusal, ok := AsRefusal(err); !ok || refusal.Code != "RECEIPT_INVALID" {
		t.Fatalf("got %v, want RECEIPT_INVALID", err)
	}
}

func TestContextReceiptIsDeterministic(t *testing.T) {
	first, err := BuildContextReceipt("manifest-1", "host-1", "driver-1",
		[]string{digestB, digestA, digestC}, []string{digestC, digestA}, 42)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		next, err := BuildContextReceipt("manifest-1", "host-1", "driver-1",
			[]string{digestB, digestA, digestC}, []string{digestC, digestA}, 42)
		if err != nil {
			t.Fatal(err)
		}
		if next.ReceiptDigest != first.ReceiptDigest {
			t.Fatalf("receipt digest varies between builds: %s then %s", first.ReceiptDigest, next.ReceiptDigest)
		}
	}
}
