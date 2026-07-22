package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// runDelegate is the operator surface for scoped delegation: issue a grant,
// revoke one, or use one to approve. Nothing here can approve anything the
// interactive path would refuse — `delegate approve` runs approveSpec, the same
// function `specd approve` runs (R3.1).
func runDelegate(root string, args []string, flags map[string]string) error {
	if len(args) == 0 {
		return usageError("delegate")
	}
	switch args[0] {
	case "issue":
		return runDelegateIssue(root, args[1:], flags)
	case "revoke":
		return runDelegateRevoke(root, args[1:], flags)
	case "approve":
		return runDelegateApprove(root, args[1:], flags)
	default:
		return core.Refusef("OPERATION_UNKNOWN", "unknown delegate operation %q", args[0]).
			WithRecovery(core.RefusalActorOperator, "specd help delegate").Wrapping(ErrUsage)
	}
}

// projectIdentity binds a grant to the project it was issued in, so a grant
// file copied into another checkout authorizes nothing there.
//
// ponytail: the directory name is the identity; a project that renames its
// directory invalidates its outstanding grants. Swap in a config-declared
// project id if that ever costs anyone a real run.
func projectIdentity(root string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		return root
	}
	return filepath.Base(abs)
}

func delegationConfig(root string) (core.Config, error) {
	cfg, diagnostics := core.LoadConfig(configPaths(root), getenv())
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" {
			return core.Config{}, fmt.Errorf("load config: %s", diagnostic.Message)
		}
	}
	return cfg, nil
}

func runDelegateIssue(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return usageError("delegate")
	}
	slug := args[0]
	if err := core.ValidateSlug(slug); err != nil {
		return err
	}
	cfg, err := delegationConfig(root)
	if err != nil {
		return err
	}
	transitions := grantTransitions(flags["transitions"])
	if flags["grant"] == "" || len(transitions) == 0 {
		return usageError("delegate")
	}
	uses := 1
	if raw := flags["uses"]; raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("%w: --uses must be an integer", ErrUsage)
		}
		uses = parsed
	}
	lifetime := 24 * time.Hour
	if raw := flags["expires-in"]; raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return fmt.Errorf("%w: --expires-in must be a Go duration", ErrUsage)
		}
		lifetime = parsed
	}
	now := core.Clock()
	actor := cliActorContext()
	grant, token, err := core.IssueDelegationGrant(root, cfg, core.DelegationGrantV1{
		ID:              flags["grant"],
		Project:         projectIdentity(root),
		SpecIDs:         []string{slug},
		Transitions:     transitions,
		MaxUses:         uses,
		Issuer:          issuerIdentity(actor),
		IssuerAssurance: actor.Assurance,
		IssuedAt:        now.Format(time.RFC3339),
		ExpiresAt:       now.Add(lifetime).Format(time.RFC3339),
		ConfigDigest:    core.ConfigDigest(cfg),
		PolicyDigest:    core.PolicyDigest(cfg),
		// A production transition needs the permission spelled out; absent the
		// flag the grant cannot reach one however long it lives.
		ProductionAllowed: boolFlag(flags, "production"),
		ReasonRequired:    boolFlag(flags, "reason-required"),
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "issued grant %s for %s: transitions %s uses %d expires %s\n",
		grant.ID, slug, strings.Join(grant.Transitions, ","), grant.MaxUses, grant.ExpiresAt)
	// The one and only time the bearer value exists outside host secret
	// storage. Nothing in the repository can reproduce it (R2.2).
	fmt.Fprintf(os.Stdout, "token %s\n", token)
	fmt.Fprintln(os.Stdout, "store the token in host secret storage now; it is not recoverable from .specd/")
	return nil
}

func runDelegateRevoke(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return usageError("delegate")
	}
	if err := core.RevokeDelegationGrant(root, args[0], flags["reason"]); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "revoked grant %s\n", args[0])
	return nil
}

// runDelegateApprove is the delegated approval transaction (R3.1 to R3.4).
//
// The whole transaction runs under the authority lock, taken before the spec
// lock the approval itself needs. That order is what makes the bookkeeping
// sound: at most one delegated approval is in flight per project, so an open
// reservation observed by anyone else belongs to a process that died, and
// reconcileGrantUses can adjudicate it without guessing.
func runDelegateApprove(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return usageError("delegate")
	}
	slug := args[0]
	if err := core.ValidateSlug(slug); err != nil {
		return err
	}
	grantID, token := flags["grant"], flags["token"]
	if grantID == "" || token == "" {
		return usageError("delegate")
	}
	cfg, err := delegationConfig(root)
	if err != nil {
		return err
	}
	_, err = core.WithAuthorityLock(root, func() (struct{}, error) {
		if err := reconcileGrantUses(root, grantID); err != nil {
			return struct{}{}, err
		}
		state, err := core.LoadState(core.StatePath(root, slug))
		if err != nil {
			return struct{}{}, err
		}
		gate := approvalGateFor(state.Status)
		delegated := &delegatedApproval{
			GrantID:          grantID,
			RequestID:        grantRequestID(slug, gate, state.Revision),
			Reason:           flags["reason"],
			Actor:            cliActorContext(),
			ExpectedRevision: state.Revision,
		}
		request := core.DelegationRequest{
			Project:      projectIdentity(root),
			SpecID:       slug,
			Transition:   "approve." + gate,
			RequestID:    delegated.RequestID,
			Reason:       delegated.Reason,
			ConfigDigest: core.ConfigDigest(cfg),
			PolicyDigest: core.PolicyDigest(cfg),
			Production:   cfg.ProductionProfile(),
			Token:        token,
		}
		if err := core.ReserveGrantUse(root, cfg, request, grantID, core.Clock()); err != nil {
			return struct{}{}, err
		}
		if err := approveSpec(root, slug, delegated); err != nil {
			// The gates refused, or the state moved: the use was reserved but
			// never spent, so it goes back to the grant (R3.2).
			return struct{}{}, errors.Join(err, core.ReleaseGrantUse(root, grantID, delegated.RequestID))
		}
		return struct{}{}, core.ConsumeGrantUse(root, grantID, delegated.RequestID)
	})
	return err
}

// grantRequestID keys one grant use to one transition of one spec at one
// revision. It is the replay key (a second attempt at the same transition finds
// the use already spent) and the recovery key (its parts say which approval
// record would prove the use committed).
func grantRequestID(slug, gate string, revision int64) string {
	return fmt.Sprintf("%s:%s:%d", slug, gate, revision)
}

// reconcileGrantUses adjudicates reservations left open by a process that died
// mid-transaction (R3.3). A crash must not burn a use, and must not hand out a
// second one for an approval that did commit — so the ledger is not guessed at
// from a timeout: each open reservation is resolved against the approval record
// it names. Committed ⇒ consume, absent ⇒ release. Running it twice changes
// nothing.
func reconcileGrantUses(root, grantID string) error {
	projection, err := core.LoadGrant(root, grantID)
	if err != nil {
		return err
	}
	for requestID := range projection.Reserved {
		slug, gate, revision, ok := parseGrantRequestID(requestID)
		if !ok {
			// Not a request id this binary minted: releasing it would hand out
			// a use nobody can account for. Leave it reserved.
			continue
		}
		if approvalCommitted(root, slug, gate, revision) {
			err = errors.Join(err, core.ConsumeGrantUse(root, grantID, requestID))
			continue
		}
		err = errors.Join(err, core.ReleaseGrantUse(root, grantID, requestID))
	}
	return err
}

func parseGrantRequestID(requestID string) (slug, gate string, revision int64, ok bool) {
	parts := strings.Split(requestID, ":")
	if len(parts) != 3 {
		return "", "", 0, false
	}
	parsed, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return "", "", 0, false
	}
	return parts[0], parts[1], parsed, true
}

// approvalCommitted reports whether the approval a reservation was taken for is
// already in state. The approval record pins the revision it advanced from,
// which is the same revision the request id carries.
func approvalCommitted(root, slug, gate string, revision int64) bool {
	state, err := core.LoadState(core.StatePath(root, slug))
	if err != nil {
		return false
	}
	raw, ok := state.Records["approval:"+gate]
	if !ok {
		return false
	}
	var rec core.Record
	if err := json.Unmarshal(raw, &rec); err != nil {
		return false
	}
	return rec.ApprovedRevision == revision
}

// audit is the delegation line written beside the approval. Actor class and
// assurance come from the resolved actor context, so an unattested run records
// "unknown/advisory" rather than borrowing the operator's name (R1.3, R3.4).
func (d delegatedApproval) audit() string {
	reason := d.Reason
	if strings.TrimSpace(reason) == "" {
		reason = "-"
	}
	return fmt.Sprintf("delegated approval grant=%s use=%s actor=%s assurance=%s reason=%s",
		d.GrantID, d.RequestID, d.Actor.Class, d.Actor.Assurance, reason)
}

func issuerIdentity(actor core.ActorContext) string {
	if strings.TrimSpace(actor.Subject) != "" {
		return actor.Subject
	}
	return "unknown"
}

func boolFlag(flags map[string]string, name string) bool {
	_, ok := flags[name]
	return ok
}

// grantTransitions is splitList with the surrounding whitespace removed: a
// transition that does not match exactly authorizes nothing, so " approve.design"
// silently scoping a grant to nothing would be the worst kind of no-op.
func grantTransitions(value string) []string {
	var out []string
	for _, part := range splitList(value) {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
