package core

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

func delegationConfig() Config {
	cfg := DefaultConfig
	cfg.Delegation.Enabled = true
	return cfg
}

// sampleGrant is a fully bounded grant: one project, one spec, one transition,
// one use, pinned policy, live for an hour.
func sampleGrant(now time.Time) DelegationGrantV1 {
	return DelegationGrantV1{
		ID:              "grant-1",
		Project:         "demo-project",
		SpecIDs:         []string{"demo"},
		Transitions:     []string{"approve.design"},
		MaxUses:         1,
		Issuer:          "ops@example.test",
		IssuerAssurance: AssuranceSandboxed,
		IssuedAt:        now.Format(time.RFC3339),
		ExpiresAt:       now.Add(time.Hour).Format(time.RFC3339),
		ConfigDigest:    Digest([]byte("config")),
		PolicyDigest:    Digest([]byte("policy")),
		ReasonRequired:  true,
	}
}

func sampleRequest(grant DelegationGrantV1, token, requestID string) DelegationRequest {
	return DelegationRequest{
		Project:      grant.Project,
		SpecID:       grant.SpecIDs[0],
		Transition:   grant.Transitions[0],
		RequestID:    requestID,
		Reason:       "unattended nightly run",
		ConfigDigest: grant.ConfigDigest,
		PolicyDigest: grant.PolicyDigest,
		Token:        token,
	}
}

func issueSampleGrant(t *testing.T, root string, now time.Time) (DelegationGrantV1, string) {
	t.Helper()
	grant, token, err := IssueDelegationGrant(root, delegationConfig(), sampleGrant(now))
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	return grant, token
}

func refusalCode(t *testing.T, err error) string {
	t.Helper()
	refusal, ok := AsRefusal(err)
	if !ok {
		t.Fatalf("error %v is not a typed refusal", err)
	}
	return refusal.Code
}

// TestDelegationGrant pins R2.1 to R2.4, R5.1, R5.2 and R6.2: a grant is
// scoped, bounded, secret-safe, replay-resistant, and entirely inert until a
// project opts in.
func TestDelegationGrant(t *testing.T) {
	now := time.Now().UTC()

	// R6.2: with no configuration nothing issues and nothing is written.
	t.Run("offbydefault", func(t *testing.T) {
		root := t.TempDir()
		_, _, err := IssueDelegationGrant(root, DefaultConfig, sampleGrant(now))
		if err != ErrDelegationDisabled {
			t.Fatalf("issue with default config = %v, want ErrDelegationDisabled", err)
		}
		if err := ReserveGrantUse(root, DefaultConfig, DelegationRequest{}, "grant-1", now); err != ErrDelegationDisabled {
			t.Fatalf("reserve with default config = %v, want ErrDelegationDisabled", err)
		}
		if _, err := os.Stat(AuthorityDir(root)); !os.IsNotExist(err) {
			t.Fatalf("disabled delegation created %s", AuthorityDir(root))
		}
		if DefaultConfig.Delegation.Enabled {
			t.Fatal("delegation.enabled defaults to true")
		}
	})

	// R2.1: every bound is required at issue time, and the operations no grant
	// may ever authorize are refused whatever the operator wrote (R5.1).
	t.Run("scopeisrequired", func(t *testing.T) {
		cases := map[string]func(*DelegationGrantV1){
			"nospec":            func(g *DelegationGrantV1) { g.SpecIDs = nil },
			"notransition":      func(g *DelegationGrantV1) { g.Transitions = nil },
			"nouses":            func(g *DelegationGrantV1) { g.MaxUses = 0 },
			"noissuer":          func(g *DelegationGrantV1) { g.Issuer = "" },
			"noproject":         func(g *DelegationGrantV1) { g.Project = "" },
			"noconfigdigest":    func(g *DelegationGrantV1) { g.ConfigDigest = "" },
			"nopolicydigest":    func(g *DelegationGrantV1) { g.PolicyDigest = "" },
			"expirybeforeissue": func(g *DelegationGrantV1) { g.ExpiresAt = g.IssuedAt },
			"unknownassurance":  func(g *DelegationGrantV1) { g.IssuerAssurance = "fully-governed" },
			"traversalspec":     func(g *DelegationGrantV1) { g.SpecIDs = []string{"../../etc"} },
		}
		for name, mutate := range cases {
			t.Run(name, func(t *testing.T) {
				root := t.TempDir()
				grant := sampleGrant(now)
				mutate(&grant)
				if _, _, err := IssueDelegationGrant(root, delegationConfig(), grant); err == nil {
					t.Fatal("unbounded grant was issued")
				}
			})
		}
		for _, forbidden := range DelegationForbiddenTransitions {
			root := t.TempDir()
			grant := sampleGrant(now)
			grant.Transitions = []string{forbidden}
			if _, _, err := IssueDelegationGrant(root, delegationConfig(), grant); err == nil {
				t.Fatalf("grant delegating %q was issued", forbidden)
			}
		}
	})

	// R5.1: the prohibitions are stamped onto the record, so a reviewer reads
	// them from the grant and an authorization check enforces the same list.
	t.Run("prohibitionsarerecorded", func(t *testing.T) {
		root := t.TempDir()
		grant, token := issueSampleGrant(t, root, now)
		for _, forbidden := range DelegationForbiddenTransitions {
			if !slices.Contains(grant.Prohibitions, forbidden) {
				t.Fatalf("grant does not record the prohibition on %q: %v", forbidden, grant.Prohibitions)
			}
		}
		projection, err := LoadGrant(root, grant.ID)
		if err != nil {
			t.Fatal(err)
		}
		req := sampleRequest(grant, token, "req-1")
		req.Transition = "release"
		if code := refusalCode(t, AuthorizeDelegatedTransition(projection, req, now)); code != "GRANT_PROHIBITED" {
			t.Fatalf("code = %q, want GRANT_PROHIBITED", code)
		}
	})

	// R2.2/R5.2: the bearer secret never reaches the repository, and a refusal
	// never quotes the token it rejected.
	t.Run("redaction", func(t *testing.T) {
		root := t.TempDir()
		grant, token := issueSampleGrant(t, root, now)
		if token == "" || len(token) != 64 {
			t.Fatalf("token = %q, want 32 random bytes hex", token)
		}
		raw, err := os.ReadFile(GrantLedgerPath(root))
		if err != nil {
			t.Fatal(err)
		}
		ledger := string(raw)
		if strings.Contains(ledger, token) {
			t.Fatal("bearer token was written to the grant ledger")
		}
		// A prefix would be enough to shrink the search space, so assert on one
		// too rather than only the whole secret.
		if strings.Contains(ledger, token[:16]) {
			t.Fatal("bearer token prefix was written to the grant ledger")
		}
		if !strings.Contains(ledger, grant.TokenDigest) || grant.TokenDigest != DelegationTokenDigest(token) {
			t.Fatal("ledger does not carry the token digest")
		}
		// Identity and scope stay reviewable — redaction is not opacity.
		for _, want := range []string{grant.ID, grant.Project, grant.SpecIDs[0], grant.Transitions[0], grant.Issuer} {
			if !strings.Contains(ledger, want) {
				t.Fatalf("ledger omits reviewable field %q", want)
			}
		}
		projection, err := LoadGrant(root, grant.ID)
		if err != nil {
			t.Fatal(err)
		}
		wrong := strings.Repeat("a", 64)
		err = AuthorizeDelegatedTransition(projection, sampleRequest(grant, wrong, "req-1"), now)
		if code := refusalCode(t, err); code != "GRANT_SECRET_INVALID" {
			t.Fatalf("code = %q, want GRANT_SECRET_INVALID", code)
		}
		if strings.Contains(err.Error(), wrong) || strings.Contains(err.Error(), token) {
			t.Fatalf("refusal quoted a bearer token: %v", err)
		}
	})

	// R2.4: comparison is constant-time over the digests, and matching is exact.
	t.Run("constanttimetokencomparison", func(t *testing.T) {
		root := t.TempDir()
		grant, token := issueSampleGrant(t, root, now)
		if !grant.MatchesToken(token) {
			t.Fatal("issued token does not match its own grant")
		}
		for _, wrong := range []string{"", token[:63], token[:63] + "0", strings.ToUpper(token)} {
			if wrong == token {
				continue
			}
			if grant.MatchesToken(wrong) {
				t.Fatalf("token %q matched", wrong)
			}
		}
	})

	// R2.3: scope, expiry, policy drift, and production permission all refuse
	// before anything is reserved, each with its own stable code.
	t.Run("scoperefusals", func(t *testing.T) {
		root := t.TempDir()
		grant, token := issueSampleGrant(t, root, now)
		projection, err := LoadGrant(root, grant.ID)
		if err != nil {
			t.Fatal(err)
		}
		cases := []struct {
			name   string
			mutate func(*DelegationRequest)
			want   string
		}{
			{"wrongproject", func(r *DelegationRequest) { r.Project = "other" }, "GRANT_SCOPE"},
			{"wrongspec", func(r *DelegationRequest) { r.SpecID = "other" }, "GRANT_SCOPE"},
			{"wrongtransition", func(r *DelegationRequest) { r.Transition = "approve.tasks" }, "GRANT_SCOPE"},
			{"production", func(r *DelegationRequest) { r.Production = true }, "GRANT_SCOPE"},
			{"stalepolicy", func(r *DelegationRequest) { r.PolicyDigest = Digest([]byte("other")) }, "GRANT_POLICY_STALE"},
			{"staleconfig", func(r *DelegationRequest) { r.ConfigDigest = Digest([]byte("other")) }, "GRANT_POLICY_STALE"},
			{"noreason", func(r *DelegationRequest) { r.Reason = " " }, "GRANT_REASON_REQUIRED"},
			{"norequestid", func(r *DelegationRequest) { r.RequestID = "" }, "GRANT_SCOPE"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				req := sampleRequest(grant, token, "req-1")
				tc.mutate(&req)
				if code := refusalCode(t, AuthorizeDelegatedTransition(projection, req, now)); code != tc.want {
					t.Fatalf("code = %q, want %q", code, tc.want)
				}
				// A refused request reserves nothing.
				if err := ReserveGrantUse(root, delegationConfig(), req, grant.ID, now); err == nil {
					t.Fatal("refused request reserved a use")
				}
				reloaded, err := LoadGrant(root, grant.ID)
				if err != nil {
					t.Fatal(err)
				}
				if reloaded.Uses() != 0 {
					t.Fatalf("uses = %d after a refusal, want 0", reloaded.Uses())
				}
			})
		}
		// A production-permitting grant accepts the same request.
		production := sampleGrant(now)
		production.ID, production.ProductionAllowed = "grant-prod", true
		productionRoot := t.TempDir()
		issued, prodToken, err := IssueDelegationGrant(productionRoot, delegationConfig(), production)
		if err != nil {
			t.Fatal(err)
		}
		prodProjection, err := LoadGrant(productionRoot, issued.ID)
		if err != nil {
			t.Fatal(err)
		}
		req := sampleRequest(issued, prodToken, "req-1")
		req.Production = true
		if err := AuthorizeDelegatedTransition(prodProjection, req, now); err != nil {
			t.Fatalf("production-permitting grant refused: %v", err)
		}
	})

	t.Run("expired", func(t *testing.T) {
		root := t.TempDir()
		grant, token := issueSampleGrant(t, root, now)
		projection, err := LoadGrant(root, grant.ID)
		if err != nil {
			t.Fatal(err)
		}
		later := now.Add(2 * time.Hour)
		if code := refusalCode(t, AuthorizeDelegatedTransition(projection, sampleRequest(grant, token, "req-1"), later)); code != "GRANT_EXPIRED" {
			t.Fatalf("code = %q, want GRANT_EXPIRED", code)
		}
		if got := projection.Status(later); got != "expired" {
			t.Fatalf("status = %q, want expired", got)
		}
		if err := ReserveGrantUse(root, delegationConfig(), sampleRequest(grant, token, "req-1"), grant.ID, later); err == nil {
			t.Fatal("expired grant reserved a use")
		}
	})

	t.Run("revoked", func(t *testing.T) {
		root := t.TempDir()
		grant, token := issueSampleGrant(t, root, now)
		if err := RevokeDelegationGrant(root, grant.ID, "operator left the project"); err != nil {
			t.Fatal(err)
		}
		projection, err := LoadGrant(root, grant.ID)
		if err != nil {
			t.Fatal(err)
		}
		if code := refusalCode(t, AuthorizeDelegatedTransition(projection, sampleRequest(grant, token, "req-1"), now)); code != "GRANT_REVOKED" {
			t.Fatalf("code = %q, want GRANT_REVOKED", code)
		}
		if got := projection.Status(now); got != "revoked" {
			t.Fatalf("status = %q, want revoked", got)
		}
	})

	// Revocation affects future uses only: a use already consumed stays
	// consumed and the record of it is not rewritten.
	t.Run("revocationdoesnotrewritepastuses", func(t *testing.T) {
		root := t.TempDir()
		grant, token := issueSampleGrant(t, root, now)
		if err := ReserveGrantUse(root, delegationConfig(), sampleRequest(grant, token, "req-1"), grant.ID, now); err != nil {
			t.Fatal(err)
		}
		if err := ConsumeGrantUse(root, grant.ID, "req-1"); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(GrantLedgerPath(root))
		if err != nil {
			t.Fatal(err)
		}
		if err := RevokeDelegationGrant(root, grant.ID, "done"); err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(GrantLedgerPath(root))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(after), string(before)) {
			t.Fatal("revocation rewrote the ledger instead of appending to it")
		}
		projection, err := LoadGrant(root, grant.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !projection.Consumed["req-1"] {
			t.Fatal("revocation erased a consumed use")
		}
	})

	t.Run("exhausted", func(t *testing.T) {
		root := t.TempDir()
		grant, token := issueSampleGrant(t, root, now)
		if err := ReserveGrantUse(root, delegationConfig(), sampleRequest(grant, token, "req-1"), grant.ID, now); err != nil {
			t.Fatal(err)
		}
		if err := ConsumeGrantUse(root, grant.ID, "req-1"); err != nil {
			t.Fatal(err)
		}
		projection, err := LoadGrant(root, grant.ID)
		if err != nil {
			t.Fatal(err)
		}
		if projection.Remaining() != 0 || projection.Status(now) != "exhausted" {
			t.Fatalf("projection = %+v, want an exhausted grant", projection)
		}
		if code := refusalCode(t, AuthorizeDelegatedTransition(projection, sampleRequest(grant, token, "req-2"), now)); code != "GRANT_EXHAUSTED" {
			t.Fatalf("code = %q, want GRANT_EXHAUSTED", code)
		}
		// An outstanding reservation counts against the maximum too, so a
		// second in-flight request cannot overdraw a one-use grant.
		second := t.TempDir()
		grant2, token2 := issueSampleGrant(t, second, now)
		if err := ReserveGrantUse(second, delegationConfig(), sampleRequest(grant2, token2, "req-1"), grant2.ID, now); err != nil {
			t.Fatal(err)
		}
		if err := ReserveGrantUse(second, delegationConfig(), sampleRequest(grant2, token2, "req-2"), grant2.ID, now); err == nil {
			t.Fatal("a reserved-but-uncommitted use did not count against max uses")
		}
		// Releasing an uncommitted reservation returns the use.
		if err := ReleaseGrantUse(second, grant2.ID, "req-1"); err != nil {
			t.Fatal(err)
		}
		if err := ReserveGrantUse(second, delegationConfig(), sampleRequest(grant2, token2, "req-2"), grant2.ID, now); err != nil {
			t.Fatalf("released use was not returned: %v", err)
		}
	})

	// R2.4: one approval request gets at most one use, forever. Replaying the
	// request id after it reserved, consumed, or released is refused.
	t.Run("replay", func(t *testing.T) {
		root := t.TempDir()
		grant := sampleGrant(now)
		grant.MaxUses = 5
		issued, token, err := IssueDelegationGrant(root, delegationConfig(), grant)
		if err != nil {
			t.Fatal(err)
		}
		if err := ReserveGrantUse(root, delegationConfig(), sampleRequest(issued, token, "req-1"), issued.ID, now); err != nil {
			t.Fatal(err)
		}
		if err := ReserveGrantUse(root, delegationConfig(), sampleRequest(issued, token, "req-1"), issued.ID, now); err == nil {
			t.Fatal("an open reservation was replayed")
		}
		if err := ConsumeGrantUse(root, issued.ID, "req-1"); err != nil {
			t.Fatal(err)
		}
		if err := ConsumeGrantUse(root, issued.ID, "req-1"); err == nil {
			t.Fatal("a consumed use was consumed twice")
		}
		projection, err := LoadGrant(root, issued.ID)
		if err != nil {
			t.Fatal(err)
		}
		if code := refusalCode(t, AuthorizeDelegatedTransition(projection, sampleRequest(issued, token, "req-1"), now)); code != "GRANT_REPLAY" {
			t.Fatalf("code = %q, want GRANT_REPLAY", code)
		}
		if projection.Uses() != 1 {
			t.Fatalf("uses = %d after a replayed request, want 1", projection.Uses())
		}
		// Consuming or releasing without a reservation invents no use.
		if err := ConsumeGrantUse(root, issued.ID, "never-reserved"); err == nil {
			t.Fatal("consumed a use that was never reserved")
		}
		if err := ReleaseGrantUse(root, issued.ID, "never-reserved"); err == nil {
			t.Fatal("released a use that was never reserved")
		}
	})

	// A duplicate grant id would make the projection ambiguous — which grant's
	// bounds apply? — so it is refused at issue.
	t.Run("duplicategrantidrefused", func(t *testing.T) {
		root := t.TempDir()
		grant, _ := issueSampleGrant(t, root, now)
		if _, _, err := IssueDelegationGrant(root, delegationConfig(), sampleGrant(now)); err == nil {
			t.Fatalf("grant %s was issued twice", grant.ID)
		}
	})
}
