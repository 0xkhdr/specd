package core

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"
)

// senseHostReportedCost sums host-reported cost across a session's evidence
// events. Cost is hostReported and untrusted (see roles/pinky.md): it never
// gates completion, but it drives the advisory cost-limit escalation (GAP-4).
// Each evidence MessageID is counted once so a replayed/duplicated report does
// not double-charge. Unparseable cost strings contribute 0 rather than failing
// the sense — malformed untrusted input must not crash the deterministic core.
func senseHostReportedCost(root, sessionID string) (float64, error) {
	store, err := NewACPStore(root)
	if err != nil {
		return 0, err
	}
	events, err := store.readAllEvents(sessionID)
	if err != nil {
		return 0, err
	}
	var total float64
	seen := make(map[string]struct{}, len(events))
	for _, event := range events {
		if event.Type != ACPMessageEvidence {
			continue
		}
		if _, dup := seen[event.MessageID]; dup {
			continue
		}
		seen[event.MessageID] = struct{}{}
		var payload ACPEvidencePayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			continue
		}
		total += parseHostCostUSD(payload.HostCost)
	}
	return total, nil
}

// parseHostCostUSD parses a host-reported cost string into USD. It accepts an
// optional leading "$" and thousands separators. Anything it cannot parse to a
// finite, non-negative number contributes 0 (untrusted input, fail-soft).
func parseHostCostUSD(raw string) float64 {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0
	}
	s = strings.TrimSpace(strings.ReplaceAll(strings.TrimPrefix(s, "$"), ",", ""))
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return 0
	}
	return v
}

// senseSessionExpired reports whether the session's fixed wall-clock deadline
// (session.ExpiresAt) has passed. Absent a session file there is no deadline to
// enforce (plain controller mode), so it returns false.
func senseSessionExpired(root, sessionID string) (bool, error) {
	session, ok, err := loadOrchestrationSessionIfExists(root, sessionID)
	if err != nil || !ok {
		return false, err
	}
	expires, err := time.Parse(time.RFC3339Nano, session.ExpiresAt)
	if err != nil {
		return false, nil
	}
	return !Clock().UTC().Before(expires.UTC()), nil
}
