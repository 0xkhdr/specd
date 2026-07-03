package contextpkg

import (
	"os"
	"path/filepath"
	"sort"
)

// ContextHUD is the deterministic context heads-up display for a spec: the
// steering/skill files a host would load, their on-disk byte and approximate
// token cost, and the active mode/tier. Every field is derived from files on
// disk and recorded state only — no interpretation, no LLM call — so the same
// inputs always render the same HUD (invariant 6/7). It is shared by the
// `specd context --hud` CLI, the SSE `hud` event, and the MCP resource.
type ContextHUD struct {
	Spec         string    `json:"spec"`
	Mode         string    `json:"mode"`
	Tier         string    `json:"tier,omitempty"`
	Files        []HUDFile `json:"files"`
	Skills       []string  `json:"skills"`
	TotalBytes   int       `json:"totalBytes"`
	ApproxTokens int       `json:"approxTokens"`
}

// HUDFile is one load-list entry with its measured cost. Missing files are
// reported with Exists=false and zero cost rather than omitted, so a host can
// tell an absent steering file from an empty one.
type HUDFile struct {
	Path         string `json:"path"`
	Exists       bool   `json:"exists"`
	Bytes        int    `json:"bytes"`
	ApproxTokens int    `json:"approxTokens"`
}

// BuildContextHUD measures each load file from disk and totals the byte/token
// cost. loadFiles are root-relative paths (steering + artifacts + skills); the
// order is preserved so the HUD reads top-to-bottom like the load list. skills
// is the deduplicated set of active skill files (a subset of loadFiles),
// surfaced separately for a host that wants just the skills.
func BuildContextHUD(root, spec, mode, tier string, loadFiles, skills []string) ContextHUD {
	hud := ContextHUD{Spec: spec, Mode: mode, Tier: tier}
	for _, rel := range loadFiles {
		f := HUDFile{Path: rel}
		if b, err := os.ReadFile(filepath.Join(root, rel)); err == nil {
			f.Exists = true
			f.Bytes = len(b)
			f.ApproxTokens = EstimateTokens(b)
		}
		hud.TotalBytes += f.Bytes
		hud.ApproxTokens += f.ApproxTokens
		hud.Files = append(hud.Files, f)
	}
	sk := append([]string{}, skills...)
	sort.Strings(sk)
	hud.Skills = sk
	return hud
}
