package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	speccontext "github.com/0xkhdr/specd/internal/context"
	"github.com/0xkhdr/specd/internal/core"
)

type sessionGuidance struct {
	core.DriverSession
	NextCommands []string `json:"next_commands"`
}

func sessionAction(root, slug string) (core.OperationBinding, string, error) {
	session, err := core.LoadDriverSession(core.DriverSessionPath(root, slug))
	if err != nil {
		return core.OperationBinding{}, "", err
	}
	if session.ID == "" || session.Expired(time.Now()) {
		return core.OperationBinding{}, "", core.Refusef("SESSION_UNKNOWN", "no live driver session for %s", slug).
			WithRecovery(core.RefusalActorAgent, "specd session open "+slug+" --driver <host>")
	}
	state, err := core.LoadState(core.StatePath(root, slug))
	if err != nil {
		return core.OperationBinding{}, "", err
	}
	nonce, err := core.NewNonce()
	if err != nil {
		return core.OperationBinding{}, "", err
	}
	binding := core.OperationBinding{
		SessionID:        session.ID,
		ExpectedRevision: state.Revision,
		HandshakeDigest:  session.HandshakeDigest,
		AuthorityDigest:  session.AuthorityDigest,
		BaselineRevision: session.BaselineRevision,
		Nonce:            nonce,
	}
	if session.ContextReceipt != nil {
		binding.ContextReceiptDigest = session.ContextReceipt.ReceiptDigest
	}
	return binding, fmt.Sprintf("specd complete-task %s %%s --session %s --nonce %s", slug, session.ID, nonce), nil
}

func runSession(root string, args []string, flags map[string]string) error {
	if len(args) < 2 || len(args) > 3 {
		return usageError("session")
	}
	action, slug := args[0], args[1]
	if _, err := loadSpec(root, slug); err != nil {
		return err
	}
	asJSON := flagEnabled(flags, "json")

	switch action {
	case "open":
		driver := flags["driver"]
		if driver == "" {
			return core.Refuse("FLAG_VALUE_INVALID", "session open requires --driver <host> to name the opening host")
		}
		state, err := core.LoadState(core.StatePath(root, slug))
		if err != nil {
			return err
		}
		config, _ := core.LoadConfig(configPaths(root), getenv())
		handshake := core.BootstrapHandshake(config)
		session, err := core.OpenDriverSession(root, slug, driver, handshake.GuidanceDigest, gitHead(root), state.Revision, time.Now())
		if err != nil {
			return err
		}
		if asJSON {
			return writeJSON(sessionGuidance{DriverSession: session, NextCommands: []string{
				"specd session ack " + slug + " <task> --tokens <n>",
				"specd verify " + slug + " <task>",
				"specd session action " + slug,
				"specd complete-task " + slug + " <task> --session <session> --nonce <nonce>",
			}})
		}
		fmt.Fprintf(os.Stdout, "opened driver session %s for %s (driver %s, baseline revision %d, expires %s)\n",
			session.ID, slug, session.Driver, session.BaselineRevision, session.ExpiresAt.Format(time.RFC3339))
		fmt.Fprintf(os.Stdout, "next: specd session ack %s <task> --tokens <n>\nthen: specd verify %s <task>\nthen: specd session action %s\nthen: specd complete-task %s <task> --session <session> --nonce <nonce>\n",
			slug, slug, slug, slug)
		return nil

	case "show":
		session, err := core.LoadDriverSession(core.DriverSessionPath(root, slug))
		if err != nil {
			return err
		}
		if session.ID == "" {
			return core.Refusef("SESSION_UNKNOWN", "no driver session is open for %s", slug).
				WithRecovery(core.RefusalActorAgent, "specd session open "+slug+" --driver <host>")
		}
		if asJSON {
			return writeJSON(session)
		}
		expiry := "expired"
		if !session.Expired(time.Now()) {
			expiry = "expires " + session.ExpiresAt.Format(time.RFC3339)
		}
		authority := "none (mutable operations refused)"
		if session.AuthorityDigest != "" {
			authority = session.AuthorityDigest
		}
		fmt.Fprintf(os.Stdout, "session %s\ndriver %s\nbaseline revision %d\nauthority %s\nnonces spent %d\n%s\n",
			session.ID, session.Driver, session.BaselineRevision, authority, len(session.SpentNonces), expiry)
		return nil

	case "action":
		// R2.2: mint the single-use nonce and hand back the bindings a mutable
		// operation must carry. The nonce is minted here rather than by the host
		// so that "single use" is the harness's fact, not the host's promise.
		binding, command, err := sessionAction(root, slug)
		if err != nil {
			return err
		}
		if asJSON {
			return writeJSON(binding)
		}
		fmt.Fprintf(os.Stdout, "session %s\nnonce %s\nexpected revision %d\nrun: specd complete-task %s <task> --session %s --nonce %s\n",
			binding.SessionID, binding.Nonce, binding.ExpectedRevision, slug, binding.SessionID, binding.Nonce)
		_ = command
		return nil

	case "ack":
		// R3.1: the host acknowledges the context it loaded. specd derives the
		// missing set from the manifest rather than trusting the host's own
		// account of what it skipped.
		if len(args) != 3 {
			return core.Refuse("FLAG_VALUE_INVALID", "session ack requires a task id: specd session ack <spec> <task> --tokens <n>")
		}
		taskID := args[2]
		session, err := core.LoadDriverSession(core.DriverSessionPath(root, slug))
		if err != nil {
			return err
		}
		if session.ID == "" || session.Expired(time.Now()) {
			return core.Refusef("SESSION_UNKNOWN", "no live driver session for %s", slug).
				WithRecovery(core.RefusalActorAgent, "specd session open "+slug+" --driver <host>")
		}
		spec, err := loadSpec(root, slug)
		if err != nil {
			return err
		}
		config, _ := core.LoadConfig(configPaths(root), getenv())
		manifest, err := speccontext.BuildMachineManifest(root, slug, spec.Tasks, taskID, "context", "execute",
			contextBudget(root), core.BootstrapHandshake(config))
		if err != nil {
			return err
		}
		required := speccontext.RequiredDigests(manifest)
		// A CLI ack asserts the host loaded what the manifest required. Over a
		// real transport the host sends the digests it actually holds; here the
		// operator is asserting it, which is why the session stays advisory.
		supplied := required
		if flagEnabled(flags, "partial") {
			supplied = nil
		}
		tokens := 0
		if raw := flags["tokens"]; raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 0 {
				return core.Refusef("FLAG_VALUE_INVALID", "--tokens must be a non-negative integer, got %q", raw)
			}
			tokens = parsed
		}
		receipt, err := core.BuildContextReceipt(manifest.ManifestDigest, session.Driver, session.Driver, required, supplied, tokens)
		if err != nil {
			return err
		}
		state, err := core.LoadState(core.StatePath(root, slug))
		if err != nil {
			return err
		}
		updated, err := core.RotateDriverSession(root, slug, session.ID, gitHead(root), state.Revision, time.Now())
		if err != nil {
			return err
		}
		updated, err = core.RecordContextReceipt(root, slug, session.ID, receipt, time.Now())
		if err != nil {
			return err
		}
		// R3.2: authority activates only once the required context is
		// acknowledged. An incomplete receipt binds nothing, so the session keeps
		// no authority digest and every mutable operation refuses.
		if receipt.Complete() {
			task, ok := findTaskRow(spec.Tasks, taskID)
			if !ok {
				return core.Refusef("SPEC_INVALID", "task %s is not in %s", taskID, slug)
			}
			now := time.Now()
			authority, err := core.BuildAuthority(task, session.Driver, session.Driver, slug, string(state.Phase),
				fmt.Sprintf("%d", state.Revision), core.BootstrapHandshake(config).ConfigDigest, "none",
				now, now.Add(core.DriverSessionTTL))
			if err != nil {
				return err
			}
			// R3.2: run the packet through the receipt gate even here, where the
			// receipt is known complete. It is the one function that decides
			// whether authority activates, and routing around it because the
			// caller already checked would leave two places that answer the
			// question — the shape of drift, and of a future bypass.
			guarded, err := core.AuthorizeWithReceipt(authority, receipt, manifest.ManifestDigest)
			if err != nil {
				return err
			}
			// The packet's own canonical digest, not a hand-rolled one: it is
			// what FinalizeAuthority computed over the whole packet, so a
			// binding pins the exact grant rather than a few fields of it.
			if updated, err = core.BindAuthority(root, slug, session.ID, guarded.Digest, now); err != nil {
				return err
			}
		}
		if asJSON {
			raw, _ := json.Marshal(updated.ContextReceipt)
			var result map[string]any
			_ = json.Unmarshal(raw, &result)
			result["baseline_head"] = updated.BaselineHead
			result["preexisting_untracked"] = updated.PreexistingUntracked
			return writeJSON(result)
		}
		status := "complete; mutable authority active"
		if !receipt.Complete() {
			status = fmt.Sprintf("incomplete; %d required lane(s) unacknowledged, mutable authority withheld", len(receipt.MissingDigests))
		}
		fmt.Fprintf(os.Stdout, "acknowledged context for %s %s (%d required lanes, %d tokens reported)\n%s\n",
			slug, taskID, len(required), tokens, status)
		return nil

	case "close":
		if err := core.CloseDriverSession(root, slug); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "closed driver session for %s\n", slug)
		return nil
	}
	return usageError("session")
}
