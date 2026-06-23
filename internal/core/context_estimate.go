package core

import contextpkg "github.com/0xkhdr/specd/internal/context"

// Token estimation moved to internal/context (contextpkg) — the pure context
// engine owns the heuristic. These thin aliases keep core call sites (the
// in-package engine and its tests) compiling unchanged until the engine itself
// moves into contextpkg. The math is unchanged: ceil(len/4) plus a markdown
// surcharge, with no LLM or tokenizer dependency (No-LLM-in-context invariant).

// EstimateTokens forwards to contextpkg.EstimateTokens.
func EstimateTokens(b []byte) int { return contextpkg.EstimateTokens(b) }

// EstimateTokensString forwards to contextpkg.EstimateTokensString.
func EstimateTokensString(s string) int { return contextpkg.EstimateTokensString(s) }
