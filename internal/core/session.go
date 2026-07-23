package core

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// DriverSessionTTL bounds how long an opened session stays usable. A host that
// crashes without closing its session does not need manual repair: the session
// simply expires and the next open mints a fresh one (design "Failure").
const DriverSessionTTL = 2 * time.Hour

// DriverSession is the stateful binding between a host and one spec's mutable
// work (R2.1). It is deliberately NOT orchestration.Session: that type is the
// Brain controller's run state (leases, missions, step counter) and is opt-in,
// while this exists whenever a host drives a spec at all. They share a word,
// not a lifecycle.
//
// The session is what makes a mutable operation attributable. Without it, every
// precondition in ValidateOperation would be a value the caller supplies and
// then checks against itself.
type DriverSession struct {
	ID     string `json:"id"`
	Slug   string `json:"slug"`
	Driver string `json:"driver"`

	// BaselineRevision is the spec state revision observed at open. An
	// operation carrying a different baseline was planned against a tree that
	// has since moved (R2.2).
	BaselineRevision int64 `json:"baseline_revision"`

	// HandshakeDigest pins the guidance/palette identity the host bootstrapped
	// against, so a host cannot open under one contract and operate under
	// another.
	HandshakeDigest string `json:"handshake_digest"`

	// BaselineHead is the git HEAD observed at open. It is what the diff-scope
	// check measures a task's changes against when no brain mission pinned a
	// baseline of its own, which is the ordinary case outside the production
	// profile. Empty means the work is ungoverned and reports as advisory.
	BaselineHead string `json:"baseline_head,omitempty"`

	// PreexistingUntracked is the sorted set of untracked paths present when
	// BaselineHead was pinned. Only untracked paths absent from this snapshot
	// are attributable to the task.
	PreexistingUntracked []string `json:"preexisting_untracked,omitempty"`

	// ContextReceipt is the host's acknowledgement of the context it loaded for
	// the task currently in flight (R3.1). Nil means no lane was ever
	// acknowledged, so mutable authority has not activated.
	ContextReceipt *ContextReceipt `json:"context_receipt,omitempty"`

	// AuthorityDigest is the packet currently bound to this session, set by
	// BindAuthority when authority is issued for a task. Empty means no mutable
	// authority is active and every mutable operation refuses.
	AuthorityDigest string `json:"authority_digest,omitempty"`

	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`

	// SpentNonces is the replay ledger (R2.4). Bounded in practice by
	// ExpiresAt: a session cannot outlive its TTL, so the ledger cannot grow
	// past one session's worth of operations.
	// ponytail: linear scan over a slice; a set matters only if a single
	// session ever runs enough operations for the scan to show up.
	SpentNonces []string `json:"spent_nonces,omitempty"`

	// Revision is the CAS counter for this file, independent of spec state
	// revision.
	Revision int64 `json:"revision"`
}

// OperationBinding is the full set of preconditions a mutable operation must
// carry (R2.2). Every field is required; a zero field is a refusal, never a
// default, because "unset" and "deliberately empty" must not be the same thing
// on a security boundary.
type OperationBinding struct {
	SessionID            string `json:"session_id"`
	ExpectedRevision     int64  `json:"expected_revision"`
	HandshakeDigest      string `json:"handshake_digest"`
	AuthorityDigest      string `json:"authority_digest"`
	ContextReceiptDigest string `json:"context_receipt_digest"`
	BaselineRevision     int64  `json:"baseline_revision"`
	Nonce                string `json:"nonce"`
}

func DriverSessionPath(root, slug string) string {
	return filepath.Join(SpecDir(root, slug), "driver-session.json")
}

// NewNonce mints a single-use operation nonce.
func NewNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("mint nonce: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// OpenDriverSession mints a session bound to a spec, a driver identity, and the
// baseline revision (R2.1). Opening again replaces any existing session: a host
// that reopens has, by definition, lost the state the old session tracked.
func OpenDriverSession(root, slug, driver, handshakeDigest, baselineHead string, baselineRevision int64, now time.Time) (DriverSession, error) {
	if slug == "" || driver == "" || handshakeDigest == "" {
		return DriverSession{}, Refuse("SESSION_INVALID", "session requires a spec slug, a driver identity, and a handshake digest")
	}
	id, err := NewNonce()
	if err != nil {
		return DriverSession{}, err
	}
	var untracked []string
	if HeadPinned(baselineHead) {
		untracked, err = SnapshotUntracked(root)
		if err != nil {
			return DriverSession{}, err
		}
	}
	session := DriverSession{
		ID:                   "ds-" + id,
		Slug:                 slug,
		Driver:               driver,
		BaselineRevision:     baselineRevision,
		HandshakeDigest:      handshakeDigest,
		BaselineHead:         baselineHead,
		PreexistingUntracked: untracked,
		IssuedAt:             now.UTC(),
		ExpiresAt:            now.UTC().Add(DriverSessionTTL),
	}
	path := DriverSessionPath(root, slug)
	current, err := LoadDriverSession(path)
	if err != nil {
		return DriverSession{}, err
	}
	if err := SaveDriverSessionCAS(root, path, current.Revision, session); err != nil {
		return DriverSession{}, err
	}
	session.Revision = current.Revision + 1
	return session, nil
}

// SnapshotUntracked returns the deterministic set of untracked paths currently
// present in root.
func SnapshotUntracked(root string) ([]string, error) {
	out, err := exec.Command("git", "-C", root, "ls-files", "--others", "--exclude-standard", "-z").Output()
	if err != nil {
		return nil, fmt.Errorf("snapshot untracked paths: %w", err)
	}
	paths := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	if len(paths) == 1 && paths[0] == "" {
		return nil, nil
	}
	slices.Sort(paths)
	return paths, nil
}

// RotateDriverSession atomically pins the baseline for a newly acknowledged
// task and clears the prior task's receipt and authority. The untracked
// attribution boundary stays fixed at session open: moving it here would let a
// worker exempt files created between open and acknowledgement.
func RotateDriverSession(root, slug, sessionID, baselineHead string, baselineRevision int64, now time.Time) (DriverSession, error) {
	if !HeadPinned(baselineHead) {
		return DriverSession{}, Refuse("BASELINE_UNPINNED", "session ack requires a resolvable git HEAD; commit or initialize git before acknowledging context")
	}
	path := DriverSessionPath(root, slug)
	return WithSpecLock(root, func() (DriverSession, error) {
		session, err := LoadDriverSession(path)
		if err != nil {
			return DriverSession{}, err
		}
		if session.ID == "" || session.ID != sessionID {
			return DriverSession{}, Refusef("SESSION_UNKNOWN", "no open session %q for spec %s", sessionID, slug)
		}
		if session.Expired(now) {
			return DriverSession{}, Refusef("SESSION_EXPIRED", "session %s expired at %s", session.ID, session.ExpiresAt.Format(time.RFC3339))
		}
		next := session
		next.BaselineHead = baselineHead
		next.BaselineRevision = baselineRevision
		next.ContextReceipt = nil
		next.AuthorityDigest = ""
		if err := SaveDriverSessionCAS(root, path, session.Revision, next); err != nil {
			return DriverSession{}, err
		}
		next.Revision = session.Revision + 1
		return next, nil
	})
}

// LoadDriverSession reads the session for a spec. A missing file is not an
// error: it is a spec no host has opened, which every caller must handle
// anyway.
func LoadDriverSession(path string) (DriverSession, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return DriverSession{Revision: 0}, nil
	}
	if err != nil {
		return DriverSession{}, fmt.Errorf("read driver session: %w", err)
	}
	var session DriverSession
	if err := json.Unmarshal(data, &session); err != nil {
		return DriverSession{}, fmt.Errorf("decode driver session: %w", err)
	}
	return session, nil
}

// SaveDriverSessionCAS writes the session under the spec lock, refusing a write
// whose expected revision has moved.
func SaveDriverSessionCAS(root, path string, expectedRevision int64, next DriverSession) error {
	_, err := WithSpecLock(root, func() (struct{}, error) {
		current, err := LoadDriverSession(path)
		if err != nil {
			return struct{}{}, err
		}
		if current.Revision != expectedRevision {
			return struct{}{}, Refusef("REVISION_CONFLICT", "driver session revision moved from %d to %d", expectedRevision, current.Revision)
		}
		next.Revision = expectedRevision + 1
		data, err := json.MarshalIndent(next, "", "  ")
		if err != nil {
			return struct{}{}, fmt.Errorf("encode driver session: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return struct{}{}, fmt.Errorf("mkdir driver session: %w", err)
		}
		return struct{}{}, AtomicWrite(path, string(append(data, '\n')))
	})
	return err
}

// Expired reports whether the session has outlived its TTL.
func (s DriverSession) Expired(now time.Time) bool {
	return !s.ExpiresAt.IsZero() && !now.UTC().Before(s.ExpiresAt)
}

// ValidateOperation checks every binding a mutable operation must carry (R2.2)
// against the session and the current spec revision, and refuses on any stale,
// reused, or mismatched value (R2.3).
//
// It is a pure function: it never writes. Burning the nonce is SpendNonce's
// job, so a caller that validates and then declines to act does not consume
// anything.
func (s DriverSession) ValidateOperation(binding OperationBinding, currentRevision int64, now time.Time) error {
	if s.ID == "" {
		return Refuse("SESSION_UNKNOWN", "no driver session is open for this spec").
			WithRecovery(RefusalActorAgent, "specd session open "+s.Slug+" --driver <host>")
	}
	if binding.SessionID != s.ID {
		return Refusef("SESSION_UNKNOWN", "operation names session %q but session %q is open", binding.SessionID, s.ID).
			WithRecovery(RefusalActorAgent, "specd session open "+s.Slug+" --driver <host>")
	}
	if s.Expired(now) {
		return Refusef("SESSION_EXPIRED", "session %s expired at %s", s.ID, s.ExpiresAt.Format(time.RFC3339)).
			WithRecovery(RefusalActorAgent, "specd session open "+s.Slug+" --driver <host>")
	}
	// Every binding below is required. Checked before comparison so an empty
	// value refuses as "absent" rather than as a mismatch against an empty pin.
	switch {
	case binding.HandshakeDigest == "":
		return Refuse("BINDING_MISSING", "operation carries no handshake digest")
	case binding.AuthorityDigest == "":
		return Refuse("BINDING_MISSING", "operation carries no authority digest")
	case binding.ContextReceiptDigest == "":
		return Refuse("BINDING_MISSING", "operation carries no context receipt; required context must be acknowledged before mutable authority activates").
			WithRecovery(RefusalActorAgent, "specd context "+s.Slug+" <task> --json")
	case binding.Nonce == "":
		return Refuse("BINDING_MISSING", "operation carries no nonce")
	}
	if binding.HandshakeDigest != s.HandshakeDigest {
		return Refuse("HANDSHAKE_MISMATCH", "operation was planned against different guidance than this session bootstrapped with").
			WithRecovery(RefusalActorAgent, "specd handshake bootstrap "+s.Slug+" --json")
	}
	if s.AuthorityDigest == "" {
		return Refuse("AUTHORITY_DENIED", "session holds no authority packet").
			WithRecovery(RefusalActorAgent, "specd context "+s.Slug+" <task> --json")
	}
	if binding.AuthorityDigest != s.AuthorityDigest {
		return Refuse("AUTHORITY_DENIED", "operation carries an authority packet this session did not issue").
			WithRecovery(RefusalActorAgent, "specd context "+s.Slug+" <task> --json")
	}
	if binding.BaselineRevision != s.BaselineRevision {
		return Refusef("BASELINE_DRIFTED", "operation baseline revision %d does not match session baseline %d", binding.BaselineRevision, s.BaselineRevision)
	}
	if binding.ExpectedRevision != currentRevision {
		return Refusef("REVISION_CONFLICT", "operation expected spec revision %d but current revision is %d", binding.ExpectedRevision, currentRevision).
			WithRecovery(RefusalActorAgent, "specd status "+s.Slug+" --json")
	}
	if slices.Contains(s.SpentNonces, binding.Nonce) {
		return Refusef("NONCE_REPLAYED", "nonce %s was already spent by this session", binding.Nonce)
	}
	return nil
}

// SpendNonce validates the operation and, only if it passes, burns the nonce so
// the same action cannot be replayed (R2.4). Validation and spending are one
// CAS write under the spec lock, so two concurrent operations cannot both
// observe the nonce as unspent.
func SpendNonce(root, slug string, binding OperationBinding, currentRevision int64, now time.Time) (DriverSession, error) {
	path := DriverSessionPath(root, slug)
	return WithSpecLock(root, func() (DriverSession, error) {
		session, err := LoadDriverSession(path)
		if err != nil {
			return DriverSession{}, err
		}
		session.Slug = slug
		if err := session.ValidateOperation(binding, currentRevision, now); err != nil {
			return DriverSession{}, err
		}
		next := session
		next.SpentNonces = append(slices.Clone(session.SpentNonces), binding.Nonce)
		if err := SaveDriverSessionCAS(root, path, session.Revision, next); err != nil {
			return DriverSession{}, err
		}
		next.Revision = session.Revision + 1
		return next, nil
	})
}

// BindAuthority pins an issued authority packet to the session, activating
// mutable operations. Until this is called, ValidateOperation refuses every
// mutable operation with AUTHORITY_DENIED.
func BindAuthority(root, slug, sessionID, authorityDigest string, now time.Time) (DriverSession, error) {
	path := DriverSessionPath(root, slug)
	return WithSpecLock(root, func() (DriverSession, error) {
		session, err := LoadDriverSession(path)
		if err != nil {
			return DriverSession{}, err
		}
		if session.ID == "" || session.ID != sessionID {
			return DriverSession{}, Refusef("SESSION_UNKNOWN", "no open session %q for spec %s", sessionID, slug).
				WithRecovery(RefusalActorAgent, "specd session open "+slug+" --driver <host>")
		}
		if session.Expired(now) {
			return DriverSession{}, Refusef("SESSION_EXPIRED", "session %s expired at %s", session.ID, session.ExpiresAt.Format(time.RFC3339)).
				WithRecovery(RefusalActorAgent, "specd session open "+slug+" --driver <host>")
		}
		if authorityDigest == "" {
			return DriverSession{}, Refuse("BINDING_MISSING", "cannot bind an empty authority digest")
		}
		next := session
		next.AuthorityDigest = authorityDigest
		if err := SaveDriverSessionCAS(root, path, session.Revision, next); err != nil {
			return DriverSession{}, err
		}
		next.Revision = session.Revision + 1
		return next, nil
	})
}

// RecordContextReceipt stores the host's acknowledgement on the session,
// activating mutable authority when every required lane was supplied (R3.1,
// R3.2). An incomplete receipt is still recorded: the gap is what the refusal
// later needs to name, and discarding it would leave the host guessing.
func RecordContextReceipt(root, slug, sessionID string, receipt ContextReceipt, now time.Time) (DriverSession, error) {
	if err := receipt.Validate(); err != nil {
		return DriverSession{}, err
	}
	path := DriverSessionPath(root, slug)
	return WithSpecLock(root, func() (DriverSession, error) {
		session, err := LoadDriverSession(path)
		if err != nil {
			return DriverSession{}, err
		}
		if session.ID == "" || session.ID != sessionID {
			return DriverSession{}, Refusef("SESSION_UNKNOWN", "no open session %q for spec %s", sessionID, slug).
				WithRecovery(RefusalActorAgent, "specd session open "+slug+" --driver <host>")
		}
		if session.Expired(now) {
			return DriverSession{}, Refusef("SESSION_EXPIRED", "session %s expired at %s", session.ID, session.ExpiresAt.Format(time.RFC3339)).
				WithRecovery(RefusalActorAgent, "specd session open "+slug+" --driver <host>")
		}
		next := session
		next.ContextReceipt = &receipt
		if err := SaveDriverSessionCAS(root, path, session.Revision, next); err != nil {
			return DriverSession{}, err
		}
		next.Revision = session.Revision + 1
		return next, nil
	})
}

// CloseDriverSession removes the session file. A close on a spec with no open
// session is not an error: the desired end state already holds.
func CloseDriverSession(root, slug string) error {
	_, err := WithSpecLock(root, func() (struct{}, error) {
		err := os.Remove(DriverSessionPath(root, slug))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return struct{}{}, fmt.Errorf("close driver session: %w", err)
		}
		return struct{}{}, nil
	})
	return err
}
