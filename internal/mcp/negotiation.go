package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// hostPrefs holds the per-session, non-standard tool-shaping hints a host may
// send under initialize's capabilities.specd (host-negotiation spec C2). Zero
// value means "no hints" — the feature is a strict, silent no-op (R3/R6).
type hostPrefs struct {
	maxTools            int
	preferredNamespaces []string
}

// active reports whether the host sent any usable hint. When false, tools/list
// skips negotiation entirely so output is identical to the feature-off path (AC4).
func (h hostPrefs) active() bool {
	return h.maxTools > 0 || len(h.preferredNamespaces) > 0
}

type initializeParams struct {
	ProtocolVersion string `json:"protocolVersion"`
	// Capabilities.specd is a specd-only extension (not in the MCP spec): hosts
	// that omit it are unaffected, and spec-strict hosts ignore the namespace.
	Capabilities struct {
		Specd struct {
			MaxTools            int      `json:"maxTools"`
			PreferredNamespaces []string `json:"preferredNamespaces"`
		} `json:"specd"`
	} `json:"capabilities"`
}

// parseHostPrefs reads the optional capabilities.specd hints from initialize.
// Parsing is lenient (R6): a malformed params blob or garbage values degrade to
// an inert hostPrefs rather than tearing down the handshake. A negative maxTools
// clamps to 0 (no cap); namespace validation happens later, at apply time.
func parseHostPrefs(rawParams json.RawMessage) hostPrefs {
	var p initializeParams
	_ = json.Unmarshal(rawParams, &p)
	hp := hostPrefs{
		maxTools:            p.Capabilities.Specd.MaxTools,
		preferredNamespaces: p.Capabilities.Specd.PreferredNamespaces,
	}
	if hp.maxTools < 0 {
		hp.maxTools = 0
	}
	return hp
}

// knownNamespaces are the canonical tool groups host negotiation can prioritise.
// They emerge from composite naming (host-negotiation spec §5.1): read =
// inspect/read/query, orchestration = orchestrate/worker + brain_* intents, meta
// = install-maintenance mirrors, core = the remaining command mirrors.
var knownNamespaces = map[string]bool{
	"read": true, "orchestration": true, "meta": true, "core": true,
}

// toolNamespace maps a tool name to its namespace (host-negotiation §5.1).
func toolNamespace(name string) string {
	switch name {
	case "specd_inspect", "specd_read", "specd_query":
		return "read"
	case "specd_orchestrate", "specd_worker":
		return "orchestration"
	}
	if strings.HasPrefix(name, "brain_") {
		return "orchestration"
	}
	if cmd, ok := strings.CutPrefix(name, toolPrefix); ok && metaRiskCommands[cmd] {
		return "meta"
	}
	return "core"
}

// normalizeNamespace resolves a host-supplied preferredNamespaces token to a
// canonical namespace. A token may be a namespace name ("read") or a tool name
// ("specd_read") whose namespace is taken (spec AC2). Unknown tokens are
// reported absent (R6) so they are safely ignored.
func normalizeNamespace(token string, known map[string]bool) (string, bool) {
	if knownNamespaces[token] {
		return token, true
	}
	if known[token] {
		return toolNamespace(token), true
	}
	return "", false
}

// applyHostPrefs reorders and caps a candidate tool list per the session's host
// hints (host-negotiation §5.2). It is a pure view transform applied at
// tools/list time — including dynamic re-fetches under expose:"phase" — so the
// registry stays hint-agnostic and the same prefs apply on every list (R5).
// Ordering is namespace-priority first (R2); truncation keeps config/manifest
// required tools regardless of maxTools (R4). required may be nil.
func applyHostPrefs(tools []toolDef, prefs hostPrefs, required map[string]bool) []toolDef {
	if !prefs.active() {
		return tools // R3/R6: no hints ⇒ no reorder, no cap.
	}
	// Copy so reordering never mutates a shared registry slice.
	out := make([]toolDef, len(tools))
	copy(out, tools)

	if len(prefs.preferredNamespaces) > 0 {
		out = orderByNamespace(out, prefs.preferredNamespaces)
	}
	if prefs.maxTools > 0 && len(out) > prefs.maxTools {
		out = truncateKeepingRequired(out, prefs.maxTools, required)
	}
	return out
}

// orderByNamespace stable-sorts tools so preferred namespaces lead, in the order
// the host listed them, while the original order is preserved within each bucket
// and for unpreferred tools (R2). Unknown/garbage tokens are dropped (R6).
func orderByNamespace(tools []toolDef, preferred []string) []toolDef {
	known := allToolNames()
	rank := make(map[string]int)
	for i, token := range preferred {
		if ns, ok := normalizeNamespace(token, known); ok {
			if _, seen := rank[ns]; !seen {
				rank[ns] = i
			}
		}
	}
	if len(rank) == 0 {
		return tools
	}
	sink := len(preferred) // unpreferred namespaces sort after all preferred ones.
	prio := func(t toolDef) int {
		if r, ok := rank[toolNamespace(t.Name)]; ok {
			return r
		}
		return sink
	}
	sort.SliceStable(tools, func(a, b int) bool {
		return prio(tools[a]) < prio(tools[b])
	})
	return tools
}

// truncateKeepingRequired caps the list to max while force-keeping every
// required tool, even when required alone exceeds max — in which case all
// required tools are emitted plus a stderr diagnostic (R4). Non-required tools
// fill remaining slots in the (already namespace-ordered) order, and the final
// slice preserves that order so output stays deterministic.
func truncateKeepingRequired(tools []toolDef, max int, required map[string]bool) []toolDef {
	keep := make(map[string]bool, max)
	reqCount := 0
	for _, t := range tools {
		if required[t.Name] {
			keep[t.Name] = true
			reqCount++
		}
	}
	if reqCount > max {
		fmt.Fprintf(os.Stderr, "specd mcp: %d required tools exceed maxTools=%d; emitting all required\n", reqCount, max)
	}
	for _, t := range tools {
		if len(keep) >= max {
			break
		}
		keep[t.Name] = true
	}
	out := tools[:0:0]
	for _, t := range tools {
		if keep[t.Name] {
			out = append(out, t)
		}
	}
	return out
}
