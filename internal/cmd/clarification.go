package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

const clarificationUsage = "usage: specd clarification <open|answer|withdraw|expire> <spec> [id] [flags]"

// clarificationTransitions maps the subcommand an operator types to the state
// the appended record records.
var clarificationTransitions = map[string]core.ClarificationTransition{
	"open":     core.ClarificationOpen,
	"answer":   core.ClarificationAnswered,
	"withdraw": core.ClarificationWithdrawn,
	"expire":   core.ClarificationExpired,
}

// runClarification appends one immutable clarification transition. Opening a
// question is agent-legal; answering, withdrawing, and expiring are human
// resolutions, mirroring the request-decision/decision split. Nothing here edits
// a prior record: a resolution is a new key, and a changed question is a new id
// (spec 03 R4.1, R4.2, R4.3).
func runClarification(root string, args []string, flags map[string]string) error {
	if len(args) < 2 {
		return errors.New(clarificationUsage)
	}
	slug := args[1]
	spec, err := loadSpec(root, slug)
	if err != nil {
		return err
	}
	transition, ok := clarificationTransitions[args[0]]
	if !ok {
		return errors.New(clarificationUsage)
	}
	rec := core.ClarificationRecord{Transition: transition}
	switch rec.Transition {
	case core.ClarificationOpen:
		if len(args) != 2 {
			return errors.New(clarificationUsage)
		}
		if rec.EntityKind, rec.EntityID, err = clarificationEntity(flags["entity"], slug); err != nil {
			return err
		}
		rec.Question = strings.TrimSpace(flags["question"])
		rec.Blocking = flagEnabled(flags, "blocking")
	case core.ClarificationAnswered, core.ClarificationWithdrawn, core.ClarificationExpired:
		if len(args) != 3 {
			return errors.New(clarificationUsage)
		}
		rec.ID = args[2]
		rec.Answer = strings.TrimSpace(flags["answer"])
		rec.Reason = strings.TrimSpace(flags["reason"])
	}
	appended, err := core.WithSpecLock(root, func() (core.ClarificationRecord, error) {
		statePath := core.StatePath(root, slug)
		state, err := core.LoadState(statePath)
		if err != nil {
			return rec, err
		}
		existing, err := core.ReadClarifications(state.Records)
		if err != nil {
			return rec, err
		}
		if rec.Transition == core.ClarificationOpen {
			rec.ID = core.NextClarificationID(existing)
		}
		key, planned, err := core.PlanClarification(existing, rec)
		if err != nil {
			return rec, err
		}
		// The version is pinned at the moment the question is asked and again
		// when it is answered, so a later task revision reads as a stale answer
		// instead of silently inheriting one (R4.3).
		planned.EntityVersion = clarificationEntityVersion(spec.Tasks, planned)
		planned = core.StampClarification(planned, gitHead(root))
		raw, err := json.Marshal(planned)
		if err != nil {
			return rec, err
		}
		if state.Records == nil {
			state.Records = map[string]json.RawMessage{}
		}
		state.Records[key] = raw
		return planned, core.SaveStateCAS(statePath, state.Revision, state)
	})
	if err != nil {
		return err
	}
	if flagEnabled(flags, "json") {
		return writeJSON(appended)
	}
	fmt.Fprintf(os.Stdout, "clarification %s %s for %s:%s\n", appended.ID, appended.Transition, appended.EntityKind, appended.EntityID)
	return nil
}

// clarificationEntity parses <kind>:<id>, defaulting to the spec itself.
func clarificationEntity(entity, slug string) (string, string, error) {
	if strings.TrimSpace(entity) == "" {
		return core.ClarificationEntitySpec, slug, nil
	}
	kind, id, ok := strings.Cut(entity, ":")
	if !ok || strings.TrimSpace(id) == "" {
		return "", "", fmt.Errorf("%w: --entity must be <spec|task|artifact>:<id>", ErrUsage)
	}
	return kind, id, nil
}

func clarificationEntityVersion(tasks []core.TaskRow, rec core.ClarificationRecord) string {
	if rec.EntityKind != core.ClarificationEntityTask {
		return ""
	}
	return core.TaskEntityVersions(tasks)[rec.EntityID]
}
