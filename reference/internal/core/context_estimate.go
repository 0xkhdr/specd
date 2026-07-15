package core

import contextpkg "github.com/0xkhdr/specd/internal/context"

// Token estimation lives in internal/context (contextpkg) — the pure context
// engine owns the heuristic. This thin alias keeps the one remaining core caller
// (mode_recommend.go) compiling without importing contextpkg directly. The math
// is unchanged: ceil(len/4) plus a markdown surcharge, with no LLM or tokenizer
// dependency (No-LLM-in-context invariant).

// EstimateTokens forwards to contextpkg.EstimateTokens.
func EstimateTokens(b []byte) int { return contextpkg.EstimateTokens(b) }

// EstimateTokensString forwards to contextpkg.EstimateTokensString — used by the
// compaction path to size a phase summary without an external tokenizer.
func EstimateTokensString(s string) int { return contextpkg.EstimateTokensString(s) }
