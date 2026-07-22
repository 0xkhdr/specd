package core

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

// DelegationGrantSchemaVersion versions the persisted grant contract. An
// unrecognized version fails closed rather than being read on a guess: a grant
// is authority, and authority read wrong is authority invented.
const DelegationGrantSchemaVersion = 1

// DelegationForbiddenTransitions are the operations no grant may ever
// authorize, whatever an operator writes into one (R5.1). They are the
// operations whose whole value is that a human or a passing check stood behind
// them: evidence, security exceptions, and anything that ships.
var DelegationForbiddenTransitions = []string{
	"archive", "complete-task", "deploy", "exception", "release", "submit", "verify",
}

// DelegationGrantV1 is one scoped, bounded, revocable delegation of approval
// authority (R2.1).
//
// Every field narrows. There is no field here that widens what the bearer may
// do, and no field that carries the bearer secret — only its digest, so the
// record on disk cannot be replayed as the token itself (R2.2).
type DelegationGrantV1 struct {
	SchemaVersion int    `json:"schema_version"`
	ID            string `json:"id"`
	// Project is the repository identity the grant was minted for. A grant
	// copied into another project authorizes nothing there.
	Project string `json:"project"`
	// SpecIDs is the closed set of specs the grant covers. Empty is invalid:
	// an unbounded grant is not a scoped grant.
	SpecIDs []string `json:"spec_ids"`
	// Transitions is the closed set of transitions the grant may approve,
	// spelled exactly. No patterns, no wildcards.
	Transitions []string `json:"transitions"`
	MaxUses     int      `json:"max_uses"`
	// Issuer and IssuerAssurance record who minted the grant and how well their
	// identity was attested at the time (see ActorContext, R1.x).
	Issuer          string         `json:"issuer"`
	IssuerAssurance AssuranceLevel `json:"issuer_assurance"`
	IssuedAt        string         `json:"issued_at"`
	ExpiresAt       string         `json:"expires_at"`
	// ConfigDigest and PolicyDigest pin the policy the grant was issued under.
	// A grant is authority over a known ruleset, not over whatever the ruleset
	// later becomes, so drift refuses instead of silently carrying over.
	ConfigDigest string `json:"config_digest"`
	PolicyDigest string `json:"policy_digest"`
	// ProductionAllowed must be set explicitly for a production transition.
	ProductionAllowed bool `json:"production_allowed"`
	// ReasonRequired makes a use refuse without an operator reason.
	ReasonRequired bool `json:"reason_required"`
	// Prohibitions is the explicit denial list carried on the record, so a
	// reviewer reads what the grant cannot do from the grant itself rather than
	// from this file. It always contains DelegationForbiddenTransitions.
	Prohibitions []string `json:"prohibitions"`
	// TokenDigest is SHA-256 over the bearer token's random bytes. The token
	// itself is shown to the operator once and never written here.
	TokenDigest string `json:"token_digest"`
}

// Grant ledger event kinds. A grant is never edited: issue, revoke, and each
// use are separate appended facts, so a projection is a replay and revocation
// can never rewrite a use that already happened.
const (
	GrantIssued      = "grant_issued"
	GrantRevoked     = "grant_revoked"
	GrantUsePrepared = "use_prepared"
	GrantUseConsumed = "use_consumed"
	GrantUseReleased = "use_released"
)

// GrantEventV1 is one appended fact about one grant. RequestID keys a use to
// the approval request that asked for it, which is what makes a use
// non-replayable: the same request can reserve at most one use, forever (R2.4).
type GrantEventV1 struct {
	SchemaVersion int                `json:"schema_version"`
	Kind          string             `json:"kind"`
	GrantID       string             `json:"grant_id"`
	Grant         *DelegationGrantV1 `json:"grant,omitempty"`
	RequestID     string             `json:"request_id,omitempty"`
	Reason        string             `json:"reason,omitempty"`
	Actor         string             `json:"actor"`
	Timestamp     string             `json:"timestamp"`
}

// GrantProjection is the replayed state of one grant.
type GrantProjection struct {
	Grant DelegationGrantV1
	// Revoked blocks future preparation only. Uses already consumed stay
	// consumed and the approvals they authorized stay valid.
	Revoked bool
	// Reserved holds request ids with an open reservation; Consumed holds the
	// ones that committed. A use moves between them, never back.
	Reserved map[string]bool
	Consumed map[string]bool
}

// DelegationRequest is one attempt to use a grant.
type DelegationRequest struct {
	Project      string
	SpecID       string
	Transition   string
	RequestID    string
	Reason       string
	ConfigDigest string
	PolicyDigest string
	Production   bool
	// Token is the bearer secret supplied by the caller. It is compared in
	// constant time and never stored, logged, or formatted into an error.
	Token string
}

// GrantLedgerPath is the append-only grant ledger for a project.
func GrantLedgerPath(root string) string {
	return filepath.Join(AuthorityDir(root), "grants.jsonl")
}

// NewDelegationToken mints a bearer token and its digest. The token is returned
// to the operator once; only the digest is ever persisted (R2.2).
func NewDelegationToken() (token, digest string, err error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", "", fmt.Errorf("mint delegation token: %w", err)
	}
	token = hex.EncodeToString(raw[:])
	return token, DelegationTokenDigest(token), nil
}

// DelegationTokenDigest is SHA-256 over the token bytes, hex encoded.
func DelegationTokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// MatchesToken compares a supplied token against the grant's digest in constant
// time, so a wrong token cannot be narrowed byte by byte from timing (R2.4).
func (g DelegationGrantV1) MatchesToken(token string) bool {
	return subtle.ConstantTimeCompare([]byte(DelegationTokenDigest(token)), []byte(g.TokenDigest)) == 1
}

// Validate refuses a grant that is not fully bounded. Everything a grant needs
// to be checkable later must be present when it is written, because a field
// that is optional at issue time is a field an authorization check has to guess
// about at use time.
func (g DelegationGrantV1) Validate() error {
	if g.SchemaVersion != DelegationGrantSchemaVersion {
		return fmt.Errorf("unsupported delegation grant schema %d", g.SchemaVersion)
	}
	if !boundedIdentifier.MatchString(g.ID) {
		return fmt.Errorf("delegation grant id %q is not a bounded identifier", g.ID)
	}
	for name, value := range map[string]string{
		"project": g.Project, "issuer": g.Issuer, "issued_at": g.IssuedAt, "expires_at": g.ExpiresAt,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("delegation grant %s: %s is required", g.ID, name)
		}
	}
	if len(g.SpecIDs) == 0 {
		return fmt.Errorf("delegation grant %s must name at least one spec", g.ID)
	}
	for _, slug := range g.SpecIDs {
		if err := ValidateSlug(slug); err != nil {
			return fmt.Errorf("delegation grant %s: %w", g.ID, err)
		}
	}
	if len(g.Transitions) == 0 {
		return fmt.Errorf("delegation grant %s must name at least one transition", g.ID)
	}
	for _, transition := range g.Transitions {
		if slices.Contains(DelegationForbiddenTransitions, transition) {
			return fmt.Errorf("delegation grant %s cannot authorize %q: no grant may delegate it", g.ID, transition)
		}
	}
	for _, forbidden := range DelegationForbiddenTransitions {
		if !slices.Contains(g.Prohibitions, forbidden) {
			return fmt.Errorf("delegation grant %s must record the prohibition on %q", g.ID, forbidden)
		}
	}
	if g.MaxUses < 1 {
		return fmt.Errorf("delegation grant %s must permit at least one use", g.ID)
	}
	issued, err := time.Parse(time.RFC3339, g.IssuedAt)
	if err != nil {
		return fmt.Errorf("delegation grant %s issued_at must be RFC3339: %w", g.ID, err)
	}
	expires, err := time.Parse(time.RFC3339, g.ExpiresAt)
	if err != nil {
		return fmt.Errorf("delegation grant %s expires_at must be RFC3339: %w", g.ID, err)
	}
	if !expires.After(issued) {
		return fmt.Errorf("delegation grant %s expires before it is issued", g.ID)
	}
	switch g.IssuerAssurance {
	case AssuranceAdvisory, AssuranceGated, AssuranceSandboxed:
	default:
		return fmt.Errorf("delegation grant %s issuer assurance %q is not a known level", g.ID, g.IssuerAssurance)
	}
	for name, digest := range map[string]string{
		"config_digest": g.ConfigDigest, "policy_digest": g.PolicyDigest, "token_digest": g.TokenDigest,
	} {
		if !validDigest(digest) {
			return fmt.Errorf("delegation grant %s %s is invalid", g.ID, name)
		}
	}
	return nil
}

// ErrDelegationDisabled is the inert default (R6.2): with no configuration,
// nothing issues, nothing authorizes, and nothing is written to disk.
var ErrDelegationDisabled = errors.New("delegation is disabled: set delegation.enabled in project.yml")

// IssueDelegationGrant mints a token, seals the grant, and appends the issue
// event under the authority lock. It returns the bearer token exactly once —
// the caller hands it to host secret storage, and nothing in the repository can
// reproduce it afterwards (R2.2).
func IssueDelegationGrant(root string, cfg Config, grant DelegationGrantV1) (DelegationGrantV1, string, error) {
	if !cfg.Delegation.Enabled {
		return DelegationGrantV1{}, "", ErrDelegationDisabled
	}
	token, digest, err := NewDelegationToken()
	if err != nil {
		return DelegationGrantV1{}, "", err
	}
	grant.SchemaVersion, grant.TokenDigest = DelegationGrantSchemaVersion, digest
	// The forbidden set is stamped onto the record rather than trusted from the
	// caller, so an operator cannot mint a grant whose written prohibitions
	// disagree with the ones actually enforced.
	for _, forbidden := range DelegationForbiddenTransitions {
		if !slices.Contains(grant.Prohibitions, forbidden) {
			grant.Prohibitions = append(grant.Prohibitions, forbidden)
		}
	}
	sort.Strings(grant.Prohibitions)
	if err := grant.Validate(); err != nil {
		return DelegationGrantV1{}, "", err
	}
	_, err = WithAuthorityLock(root, func() (struct{}, error) {
		events, err := ReadGrantEvents(root)
		if err != nil {
			return struct{}{}, err
		}
		if _, found := ProjectGrant(events, grant.ID); found {
			return struct{}{}, fmt.Errorf("delegation grant %s already exists", grant.ID)
		}
		return struct{}{}, appendGrantEvent(root, GrantEventV1{Kind: GrantIssued, GrantID: grant.ID, Grant: &grant})
	})
	if err != nil {
		return DelegationGrantV1{}, "", err
	}
	return grant, token, nil
}

// RevokeDelegationGrant blocks every future use. It never touches a use already
// consumed, and never rewrites an approval it authorized.
func RevokeDelegationGrant(root, grantID, reason string) error {
	_, err := WithAuthorityLock(root, func() (struct{}, error) {
		events, err := ReadGrantEvents(root)
		if err != nil {
			return struct{}{}, err
		}
		if _, found := ProjectGrant(events, grantID); !found {
			return struct{}{}, fmt.Errorf("unknown delegation grant %q", grantID)
		}
		return struct{}{}, appendGrantEvent(root, GrantEventV1{Kind: GrantRevoked, GrantID: grantID, Reason: reason})
	})
	return err
}

// ReserveGrantUse authorizes the request and reserves one use for it, both
// under the authority lock so the check and the reservation cannot interleave
// with another consumer's (R2.4, R3.3). A request id that already holds or
// spent a reservation is refused: one request, at most one use, ever.
//
// The reservation is the prepared half of the transaction T28A completes; a
// caller that fails afterwards releases it, and a caller that commits consumes
// it. Neither can produce a second use.
func ReserveGrantUse(root string, cfg Config, req DelegationRequest, grantID string, now time.Time) error {
	if !cfg.Delegation.Enabled {
		return ErrDelegationDisabled
	}
	_, err := WithAuthorityLock(root, func() (struct{}, error) {
		projection, err := LoadGrant(root, grantID)
		if err != nil {
			return struct{}{}, err
		}
		if err := AuthorizeDelegatedTransition(projection, req, now); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, appendGrantEvent(root, GrantEventV1{Kind: GrantUsePrepared, GrantID: grantID, RequestID: req.RequestID, Reason: req.Reason})
	})
	return err
}

// ConsumeGrantUse commits a reservation. ReleaseGrantUse returns one that never
// committed. Both refuse without an open reservation, so neither can invent a
// use the authorization check never approved.
func ConsumeGrantUse(root, grantID, requestID string) error {
	return closeGrantUse(root, grantID, requestID, GrantUseConsumed)
}

func ReleaseGrantUse(root, grantID, requestID string) error {
	return closeGrantUse(root, grantID, requestID, GrantUseReleased)
}

func closeGrantUse(root, grantID, requestID, kind string) error {
	_, err := WithAuthorityLock(root, func() (struct{}, error) {
		projection, err := LoadGrant(root, grantID)
		if err != nil {
			return struct{}{}, err
		}
		if !projection.Reserved[requestID] {
			return struct{}{}, Refusef("GRANT_USE_UNRESERVED", "delegation grant %s has no open reservation for request %s", grantID, requestID)
		}
		return struct{}{}, appendGrantEvent(root, GrantEventV1{Kind: kind, GrantID: grantID, RequestID: requestID})
	})
	return err
}

// AuthorizeDelegatedTransition refuses before any gate runs or any state is
// touched (R2.3). Each refusal carries a stable code so an operator route can
// be derived from the code rather than parsed out of a sentence.
//
// The supplied token is never echoed: a refusal that quotes the secret it
// rejected has leaked it into every log that captured the refusal (R5.2).
func AuthorizeDelegatedTransition(p GrantProjection, req DelegationRequest, now time.Time) error {
	grant := p.Grant
	if !grant.MatchesToken(req.Token) {
		return Refusef("GRANT_SECRET_INVALID", "delegation grant %s bearer token does not match", grant.ID)
	}
	if p.Revoked {
		return Refusef("GRANT_REVOKED", "delegation grant %s is revoked", grant.ID)
	}
	if expires, err := time.Parse(time.RFC3339, grant.ExpiresAt); err != nil || !now.Before(expires) {
		return Refusef("GRANT_EXPIRED", "delegation grant %s expired at %s", grant.ID, grant.ExpiresAt)
	}
	if req.Project != grant.Project {
		return Refusef("GRANT_SCOPE", "delegation grant %s is bound to project %s", grant.ID, grant.Project)
	}
	if !slices.Contains(grant.SpecIDs, req.SpecID) {
		return Refusef("GRANT_SCOPE", "delegation grant %s does not cover spec %s", grant.ID, req.SpecID)
	}
	if slices.Contains(grant.Prohibitions, req.Transition) {
		return Refusef("GRANT_PROHIBITED", "delegation grant %s prohibits transition %s", grant.ID, req.Transition)
	}
	if !slices.Contains(grant.Transitions, req.Transition) {
		return Refusef("GRANT_SCOPE", "delegation grant %s does not cover transition %s", grant.ID, req.Transition)
	}
	if req.Production && !grant.ProductionAllowed {
		return Refusef("GRANT_SCOPE", "delegation grant %s does not permit production transitions", grant.ID)
	}
	if grant.ConfigDigest != req.ConfigDigest || grant.PolicyDigest != req.PolicyDigest {
		return Refusef("GRANT_POLICY_STALE", "delegation grant %s was issued under a different policy", grant.ID)
	}
	if grant.ReasonRequired && strings.TrimSpace(req.Reason) == "" {
		return Refusef("GRANT_REASON_REQUIRED", "delegation grant %s requires a reason", grant.ID)
	}
	if strings.TrimSpace(req.RequestID) == "" {
		return Refusef("GRANT_SCOPE", "delegation grant %s requires an approval request id", grant.ID)
	}
	if p.Reserved[req.RequestID] || p.Consumed[req.RequestID] {
		return Refusef("GRANT_REPLAY", "delegation grant %s already has a use for request %s", grant.ID, req.RequestID)
	}
	if p.Remaining() < 1 {
		return Refusef("GRANT_EXHAUSTED", "delegation grant %s has used all %d uses", grant.ID, grant.MaxUses)
	}
	return nil
}

// Uses counts every use the grant has spent or has outstanding. A reservation
// counts: an in-flight approval must not be able to overdraw the grant just
// because it has not committed yet.
func (p GrantProjection) Uses() int { return len(p.Reserved) + len(p.Consumed) }

// Remaining is the uses still available.
func (p GrantProjection) Remaining() int { return p.Grant.MaxUses - p.Uses() }

// Status is the single word a report shows for a grant (R6.3), resolved in
// precedence order: an expired grant that was revoked reads as revoked, because
// revocation is the operator's decision and expiry is only the clock.
func (p GrantProjection) Status(now time.Time) string {
	switch {
	case p.Revoked:
		return "revoked"
	case p.Remaining() < 1:
		return "exhausted"
	default:
		if expires, err := time.Parse(time.RFC3339, p.Grant.ExpiresAt); err != nil || !now.Before(expires) {
			return "expired"
		}
		return "active"
	}
}

// LoadGrant replays one grant from the ledger.
func LoadGrant(root, grantID string) (GrantProjection, error) {
	events, err := ReadGrantEvents(root)
	if err != nil {
		return GrantProjection{}, err
	}
	projection, found := ProjectGrant(events, grantID)
	if !found {
		return GrantProjection{}, fmt.Errorf("unknown delegation grant %q", grantID)
	}
	return projection, nil
}

// ProjectGrant replays the ledger for one grant id.
func ProjectGrant(events []GrantEventV1, grantID string) (GrantProjection, bool) {
	projection := GrantProjection{Reserved: map[string]bool{}, Consumed: map[string]bool{}}
	found := false
	for _, event := range events {
		if event.GrantID != grantID {
			continue
		}
		switch event.Kind {
		case GrantIssued:
			if event.Grant != nil {
				projection.Grant, found = *event.Grant, true
			}
		case GrantRevoked:
			projection.Revoked = true
		case GrantUsePrepared:
			projection.Reserved[event.RequestID] = true
		case GrantUseConsumed:
			delete(projection.Reserved, event.RequestID)
			projection.Consumed[event.RequestID] = true
		case GrantUseReleased:
			delete(projection.Reserved, event.RequestID)
		}
	}
	return projection, found
}

// ReadGrantEvents reads the ledger. A malformed line fails closed: a projection
// built from records it could not read would under-count uses, which is exactly
// the direction that hands out an extra approval.
func ReadGrantEvents(root string) ([]GrantEventV1, error) {
	raw, err := os.ReadFile(GrantLedgerPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var events []GrantEventV1
	for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event GrantEventV1
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("decode grant event: %w", err)
		}
		if event.SchemaVersion != DelegationGrantSchemaVersion {
			return nil, fmt.Errorf("unsupported grant event schema %d", event.SchemaVersion)
		}
		events = append(events, event)
	}
	return events, nil
}

// appendGrantEvent stamps and appends one ledger line. Callers hold the
// authority lock; it is unexported so no path can append a use without it.
func appendGrantEvent(root string, event GrantEventV1) error {
	event.SchemaVersion = DelegationGrantSchemaVersion
	event.Timestamp = Clock().Format(time.RFC3339)
	event.Actor = recordActor()
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(AuthorityDir(root), 0o755); err != nil {
		return err
	}
	return AppendFile(GrantLedgerPath(root), string(raw)+"\n")
}
