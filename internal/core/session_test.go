package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func driverSessionRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specd", "specs", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// validBinding is the binding a well-behaved host would send for session.
func validBinding(session DriverSession, nonce string, revision int64) OperationBinding {
	return OperationBinding{
		SessionID:            session.ID,
		ExpectedRevision:     revision,
		HandshakeDigest:      session.HandshakeDigest,
		AuthorityDigest:      session.AuthorityDigest,
		ContextReceiptDigest: "receipt-digest",
		BaselineRevision:     session.BaselineRevision,
		Nonce:                nonce,
	}
}

func TestDriverSessionOpenBindsSpecDriverAndBaseline(t *testing.T) {
	root := driverSessionRoot(t)
	now := time.Now()

	session, err := OpenDriverSession(root, "demo", "claude-code", "handshake-digest", "", 7, now)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if session.ID == "" {
		t.Fatal("open minted no session id")
	}
	if session.Slug != "demo" || session.Driver != "claude-code" || session.BaselineRevision != 7 {
		t.Fatalf("session not bound to spec/driver/baseline: %+v", session)
	}
	if !session.ExpiresAt.After(session.IssuedAt) {
		t.Fatal("session carries no expiry, so a crashed host would never self-heal")
	}

	// R2.1: the binding must survive a reload, not just live in memory.
	loaded, err := LoadDriverSession(DriverSessionPath(root, "demo"))
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if loaded.ID != session.ID || loaded.BaselineRevision != 7 {
		t.Fatalf("reloaded session lost its binding: %+v", loaded)
	}
}

func TestDriverSessionRefusesEveryStaleBinding(t *testing.T) {
	root := driverSessionRoot(t)
	now := time.Now()

	session, err := OpenDriverSession(root, "demo", "claude-code", "handshake-digest", "", 3, now)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	bound, err := BindAuthority(root, "demo", session.ID, "authority-digest", now)
	if err != nil {
		t.Fatalf("bind authority: %v", err)
	}

	mutate := func(fn func(*OperationBinding)) OperationBinding {
		binding := validBinding(bound, "nonce-1", 3)
		fn(&binding)
		return binding
	}

	cases := []struct {
		name    string
		binding OperationBinding
		want    string
	}{
		{"unknown session", mutate(func(b *OperationBinding) { b.SessionID = "ds-other" }), "SESSION_UNKNOWN"},
		{"stale revision", mutate(func(b *OperationBinding) { b.ExpectedRevision = 2 }), "REVISION_CONFLICT"},
		{"drifted baseline", mutate(func(b *OperationBinding) { b.BaselineRevision = 1 }), "BASELINE_DRIFTED"},
		{"wrong handshake digest", mutate(func(b *OperationBinding) { b.HandshakeDigest = "other" }), "HANDSHAKE_MISMATCH"},
		{"wrong authority digest", mutate(func(b *OperationBinding) { b.AuthorityDigest = "other" }), "AUTHORITY_DENIED"},
		{"absent handshake digest", mutate(func(b *OperationBinding) { b.HandshakeDigest = "" }), "BINDING_MISSING"},
		{"absent authority digest", mutate(func(b *OperationBinding) { b.AuthorityDigest = "" }), "BINDING_MISSING"},
		{"absent context receipt", mutate(func(b *OperationBinding) { b.ContextReceiptDigest = "" }), "BINDING_MISSING"},
		{"absent nonce", mutate(func(b *OperationBinding) { b.Nonce = "" }), "BINDING_MISSING"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := bound.ValidateOperation(tc.binding, 3, now)
			if err == nil {
				t.Fatalf("%s was accepted; every stale binding must refuse", tc.name)
			}
			refusal, ok := AsRefusal(err)
			if !ok {
				t.Fatalf("got bare error %v, want a typed refusal", err)
			}
			if refusal.Code != tc.want {
				t.Fatalf("got code %q, want %q (%v)", refusal.Code, tc.want, err)
			}
			if refusal.RecoveryCommand == "" || refusal.ActorRequired == "" {
				t.Fatalf("refusal %q names no recovery or actor: %+v", refusal.Code, refusal)
			}
		})
	}
}

// R2.3: a refused operation must not mutate trusted state. Checked against the
// session file itself, since that is the state this package owns.
func TestDriverSessionRefusalDoesNotMutateState(t *testing.T) {
	root := driverSessionRoot(t)
	now := time.Now()
	session, err := OpenDriverSession(root, "demo", "claude-code", "handshake-digest", "", 3, now)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	bound, err := BindAuthority(root, "demo", session.ID, "authority-digest", now)
	if err != nil {
		t.Fatalf("bind: %v", err)
	}

	before, err := os.ReadFile(DriverSessionPath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}

	stale := validBinding(bound, "nonce-1", 999)
	if _, err := SpendNonce(root, "demo", stale, 3, now); err == nil {
		t.Fatal("stale revision was accepted by SpendNonce")
	}

	after, err := os.ReadFile(DriverSessionPath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("refused operation mutated the session file:\nbefore %s\nafter  %s", before, after)
	}
}

// R2.4: a completed operation invalidates its nonce.
func TestDriverSessionNonceIsSingleUse(t *testing.T) {
	root := driverSessionRoot(t)
	now := time.Now()
	session, err := OpenDriverSession(root, "demo", "claude-code", "handshake-digest", "", 3, now)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	bound, err := BindAuthority(root, "demo", session.ID, "authority-digest", now)
	if err != nil {
		t.Fatalf("bind: %v", err)
	}

	binding := validBinding(bound, "nonce-1", 3)
	if _, err := SpendNonce(root, "demo", binding, 3, now); err != nil {
		t.Fatalf("first use refused: %v", err)
	}

	_, err = SpendNonce(root, "demo", binding, 3, now)
	if err == nil {
		t.Fatal("replayed nonce was accepted")
	}
	refusal, ok := AsRefusal(err)
	if !ok || refusal.Code != "NONCE_REPLAYED" {
		t.Fatalf("got %v, want NONCE_REPLAYED", err)
	}

	// A fresh nonce on the same session still works: spending one nonce must
	// not wedge the session.
	if _, err := SpendNonce(root, "demo", validBinding(bound, "nonce-2", 3), 3, now); err != nil {
		t.Fatalf("second distinct nonce refused: %v", err)
	}
}

// R2.2: authority must be bound before any mutable operation is accepted.
func TestDriverSessionWithoutAuthorityRefusesMutableOperation(t *testing.T) {
	root := driverSessionRoot(t)
	now := time.Now()
	session, err := OpenDriverSession(root, "demo", "claude-code", "handshake-digest", "", 3, now)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	binding := validBinding(session, "nonce-1", 3)
	binding.AuthorityDigest = "authority-digest" // host claims authority it was never issued
	err = session.ValidateOperation(binding, 3, now)
	if err == nil {
		t.Fatal("operation accepted against a session holding no authority packet")
	}
	if refusal, ok := AsRefusal(err); !ok || refusal.Code != "AUTHORITY_DENIED" {
		t.Fatalf("got %v, want AUTHORITY_DENIED", err)
	}
}

func TestDriverSessionExpiryRefusesFurtherOperations(t *testing.T) {
	root := driverSessionRoot(t)
	issued := time.Now()
	session, err := OpenDriverSession(root, "demo", "claude-code", "handshake-digest", "", 3, issued)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	bound, err := BindAuthority(root, "demo", session.ID, "authority-digest", issued)
	if err != nil {
		t.Fatalf("bind: %v", err)
	}

	later := issued.Add(DriverSessionTTL + time.Minute)
	err = bound.ValidateOperation(validBinding(bound, "nonce-1", 3), 3, later)
	if err == nil {
		t.Fatal("expired session accepted an operation")
	}
	if refusal, ok := AsRefusal(err); !ok || refusal.Code != "SESSION_EXPIRED" {
		t.Fatalf("got %v, want SESSION_EXPIRED", err)
	}
}

func TestDriverSessionCloseIsIdempotent(t *testing.T) {
	root := driverSessionRoot(t)
	if _, err := OpenDriverSession(root, "demo", "claude-code", "handshake-digest", "", 1, time.Now()); err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := CloseDriverSession(root, "demo"); err != nil {
		t.Fatalf("close: %v", err)
	}
	// A crashed host may close twice; the desired end state already holds.
	if err := CloseDriverSession(root, "demo"); err != nil {
		t.Fatalf("second close: %v", err)
	}
	session, err := LoadDriverSession(DriverSessionPath(root, "demo"))
	if err != nil {
		t.Fatalf("load after close: %v", err)
	}
	if session.ID != "" {
		t.Fatal("session survived close")
	}
}
