package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// enableDelegation opts the project into scoped delegation. Without it every
// path below is inert, which is the default the T28 tests pin.
func enableDelegation(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, "project.yml")
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	body := string(raw)
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	if err := os.WriteFile(path, []byte(body+"delegation.enabled: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// issueGrant issues a grant through the CLI and returns its bearer token, which
// the verb prints exactly once.
func issueGrant(t *testing.T, root, slug, id string, flags map[string]string) string {
	t.Helper()
	all := map[string]string{"grant": id, "transitions": "approve.requirements,approve.design"}
	for key, value := range flags {
		all[key] = value
	}
	out, err := captureStdout(t, func() error { return Run(root, "delegate", []string{"issue", slug}, all) })
	if err != nil {
		t.Fatalf("delegate issue: %v", err)
	}
	for _, line := range strings.Split(out, "\n") {
		if token, ok := strings.CutPrefix(line, "token "); ok {
			return strings.TrimSpace(token)
		}
	}
	t.Fatalf("delegate issue printed no token: %q", out)
	return ""
}

func grantUses(t *testing.T, root, id string) core.GrantProjection {
	t.Helper()
	projection, err := core.LoadGrant(root, id)
	if err != nil {
		t.Fatal(err)
	}
	return projection
}

func specState(t *testing.T, root, slug string) core.State {
	t.Helper()
	state, err := core.LoadState(core.StatePath(root, slug))
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func recordAt(t *testing.T, state core.State, key string) (core.Record, bool) {
	t.Helper()
	raw, ok := state.Records[key]
	if !ok {
		return core.Record{}, false
	}
	var rec core.Record
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	return rec, true
}

// TestDelegatedApprovalTransaction pins R3.1 to R3.4 and R6.3: delegated
// approval runs the interactive gate path, consumes a use only after that path
// succeeds, survives a crash without burning or duplicating a use, and records
// an approval nobody can mistake for a human one.
func TestDelegatedApprovalTransaction(t *testing.T) {
	// R3.1: the two paths are one function, not two implementations that have
	// to be kept in agreement by hand.
	t.Run("gatefailure", func(t *testing.T) {
		root := newDemoSpec(t)
		enableDelegation(t, root)
		if err := Run(root, "new", []string{"gated"}, nil); err != nil {
			t.Fatal(err)
		}
		// Deliberately unauthored: the readiness gates refuse the stub.
		token := issueGrant(t, root, "gated", "gated-grant", nil)
		if err := Run(root, "approve", []string{"gated"}, nil); err == nil {
			t.Fatal("interactive approval passed the gates on an unauthored spec")
		}
		before := specState(t, root, "gated")

		err := Run(root, "delegate", []string{"approve", "gated"},
			map[string]string{"grant": "gated-grant", "token": token, "reason": "nightly"})
		if err == nil {
			t.Fatal("delegated approval advanced a spec the gates refuse")
		}
		refusal, ok := core.AsRefusal(err)
		if !ok || refusal.Code != "GATE_FAILED" {
			t.Fatalf("delegated refusal = %v, want the interactive GATE_FAILED refusal", err)
		}
		// R3.2: a failed gate consumes nothing.
		projection := grantUses(t, root, "gated-grant")
		if projection.Uses() != 0 || projection.Remaining() != 1 {
			t.Fatalf("failed gates consumed a use: %+v", projection)
		}
		after := specState(t, root, "gated")
		if after.Revision != before.Revision || after.Status != before.Status {
			t.Fatal("refused delegated approval mutated state")
		}
	})

	// R3.4/R6.3: the audit distinction, asserted against the stored records.
	t.Run("auditdistinction", func(t *testing.T) {
		root := newDemoSpec(t)
		enableDelegation(t, root)
		token := issueGrant(t, root, "demo", "audit-grant", map[string]string{"uses": "2", "reason-required": "1"})

		out, err := captureStdout(t, func() error {
			return Run(root, "delegate", []string{"approve", "demo"},
				map[string]string{"grant": "audit-grant", "token": token, "reason": "unattended nightly run"})
		})
		if err != nil {
			t.Fatalf("delegated approve: %v", err)
		}
		if !strings.Contains(out, "delegated") || !strings.Contains(out, "audit-grant") {
			t.Fatalf("delegated approval output is indistinguishable from interactive: %q", out)
		}
		state := specState(t, root, "demo")
		approval, ok := recordAt(t, state, "approval:requirements")
		if !ok {
			t.Fatal("delegated approval wrote no approval record")
		}
		if approval.Scope != "delegated" {
			t.Fatalf("approval scope = %q, want the delegated marker", approval.Scope)
		}
		delegation, ok := recordAt(t, state, "delegation:requirements")
		if !ok {
			t.Fatal("delegated approval wrote no delegation record")
		}
		if delegation.Scope != "audit-grant" {
			t.Fatalf("delegation record grant = %q", delegation.Scope)
		}
		for _, want := range []string{"grant=audit-grant", "use=demo:requirements:", "actor=", "assurance=", "reason=unattended nightly run"} {
			if !strings.Contains(delegation.Text, want) {
				t.Fatalf("delegation audit %q missing %q", delegation.Text, want)
			}
		}
		// A grant use that is not attested by a governed host must not read as
		// a human approval (R1.3 carried into the audit).
		if !strings.Contains(delegation.Text, "actor="+string(core.ActorClassUnknown)) ||
			!strings.Contains(delegation.Text, "assurance="+string(core.AssuranceAdvisory)) {
			t.Fatalf("unattested delegated approval claims better provenance: %q", delegation.Text)
		}
		if projection := grantUses(t, root, "audit-grant"); projection.Uses() != 1 {
			t.Fatalf("uses = %d after one delegated approval, want 1", projection.Uses())
		}

		// The interactive path over the next gate records neither marker, so
		// the two are distinguishable in the same spec's history.
		if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
			t.Fatalf("interactive approve: %v", err)
		}
		state = specState(t, root, "demo")
		interactive, ok := recordAt(t, state, "approval:design")
		if !ok {
			t.Fatal("interactive approval wrote no record")
		}
		if interactive.Scope != "" {
			t.Fatalf("interactive approval carries scope %q", interactive.Scope)
		}
		if _, ok := recordAt(t, state, "delegation:design"); ok {
			t.Fatal("interactive approval wrote a delegation record")
		}
	})

	// R3.3: a process that dies between reserving and committing must not burn
	// the use. The next run adjudicates the open reservation from the ledger.
	t.Run("crashrecovery", func(t *testing.T) {
		t.Run("uncommittedreservationisreclaimed", func(t *testing.T) {
			root := newDemoSpec(t)
			enableDelegation(t, root)
			token := issueGrant(t, root, "demo", "crash-grant", nil)
			cfg, err := delegationConfig(root)
			if err != nil {
				t.Fatal(err)
			}
			state := specState(t, root, "demo")
			// The crash: the reservation exists, the approval never committed.
			stale := grantRequestID("demo", "requirements", state.Revision)
			if err := core.ReserveGrantUse(root, cfg, core.DelegationRequest{
				Project: projectIdentity(root), SpecID: "demo", Transition: "approve.requirements",
				RequestID: stale, ConfigDigest: core.ConfigDigest(cfg), PolicyDigest: core.PolicyDigest(cfg), Token: token,
			}, "crash-grant", core.Clock()); err != nil {
				t.Fatal(err)
			}
			if projection := grantUses(t, root, "crash-grant"); projection.Remaining() != 0 {
				t.Fatal("fixture did not reserve the only use")
			}

			if err := Run(root, "delegate", []string{"approve", "demo"},
				map[string]string{"grant": "crash-grant", "token": token, "reason": "retry after crash"}); err != nil {
				t.Fatalf("retry after a crash did not reclaim the reservation: %v", err)
			}
			projection := grantUses(t, root, "crash-grant")
			if projection.Uses() != 1 || len(projection.Reserved) != 0 {
				t.Fatalf("projection after recovery = %+v, want exactly one consumed use", projection)
			}
			if specState(t, root, "demo").Status != core.StatusDesign {
				t.Fatal("recovered run did not approve")
			}
		})

		t.Run("committedreservationisfinalized", func(t *testing.T) {
			root := newDemoSpec(t)
			enableDelegation(t, root)
			token := issueGrant(t, root, "demo", "commit-grant", map[string]string{"uses": "2"})
			cfg, err := delegationConfig(root)
			if err != nil {
				t.Fatal(err)
			}
			state := specState(t, root, "demo")
			reserved := grantRequestID("demo", "requirements", state.Revision)
			if err := core.ReserveGrantUse(root, cfg, core.DelegationRequest{
				Project: projectIdentity(root), SpecID: "demo", Transition: "approve.requirements",
				RequestID: reserved, ConfigDigest: core.ConfigDigest(cfg), PolicyDigest: core.PolicyDigest(cfg), Token: token,
			}, "commit-grant", core.Clock()); err != nil {
				t.Fatal(err)
			}
			// The crash landed after the approval committed: the use was spent,
			// and recovery must finalize it rather than hand it back.
			if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
				t.Fatal(err)
			}
			if err := reconcileGrantUses(root, "commit-grant"); err != nil {
				t.Fatal(err)
			}
			projection := grantUses(t, root, "commit-grant")
			if !projection.Consumed[reserved] || len(projection.Reserved) != 0 {
				t.Fatalf("projection = %+v, want the committed reservation consumed", projection)
			}
			if projection.Remaining() != 1 {
				t.Fatalf("remaining = %d, want the second use still available", projection.Remaining())
			}
		})
	})

	// R3.3: concurrent consumers of the final use.
	t.Run("concurrentfinaluse", func(t *testing.T) {
		root := newDemoSpec(t)
		enableDelegation(t, root)
		token := issueGrant(t, root, "demo", "race-grant", nil)

		var wg sync.WaitGroup
		results := make([]error, 4)
		for i := range results {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results[i] = Run(root, "delegate", []string{"approve", "demo"},
					map[string]string{"grant": "race-grant", "token": token, "reason": "race"})
			}()
		}
		wg.Wait()

		winners := 0
		for _, err := range results {
			if err == nil {
				winners++
			}
		}
		if winners != 1 {
			t.Fatalf("%d of 4 concurrent consumers approved on a single-use grant", winners)
		}
		projection := grantUses(t, root, "race-grant")
		if projection.Uses() != 1 || len(projection.Reserved) != 0 {
			t.Fatalf("projection after the race = %+v, want one consumed use", projection)
		}
		if state := specState(t, root, "demo"); state.Status != core.StatusDesign {
			t.Fatalf("status = %q, want exactly one advance", state.Status)
		}
	})

	// R6.2 at the command surface: with delegation unconfigured the verb
	// refuses and writes nothing, while interactive approval is unaffected.
	t.Run("inertwhenunconfigured", func(t *testing.T) {
		root := newDemoSpec(t)
		if err := Run(root, "delegate", []string{"issue", "demo"},
			map[string]string{"grant": "off", "transitions": "approve.requirements"}); err != core.ErrDelegationDisabled {
			t.Fatalf("issue with delegation off = %v, want ErrDelegationDisabled", err)
		}
		if _, err := os.Stat(core.AuthorityDir(root)); !os.IsNotExist(err) {
			t.Fatal("disabled delegation created the authority directory")
		}
		if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
			t.Fatalf("interactive approval broken by the delegation surface: %v", err)
		}
	})

	// The bearer token never reaches the repository, even on the used path.
	t.Run("tokenneverpersisted", func(t *testing.T) {
		root := newDemoSpec(t)
		enableDelegation(t, root)
		token := issueGrant(t, root, "demo", "secret-grant", nil)
		if err := Run(root, "delegate", []string{"approve", "demo"},
			map[string]string{"grant": "secret-grant", "token": token, "reason": "check"}); err != nil {
			t.Fatal(err)
		}
		found := false
		err := filepath.WalkDir(core.SpecdDir(root), func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			raw, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			if strings.Contains(string(raw), token) {
				found = true
				t.Errorf("bearer token found in %s", path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if found {
			t.Fatal("delegated approval persisted the bearer token")
		}
	})
}
