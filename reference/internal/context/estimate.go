// Package contextpkg is the deterministic context engine: the single source of
// truth for "what to load" across every delivery surface. It is pure — all IO is
// injected via ContextRequest.ReadArtifact — and depends only on internal/spec
// (shared value types) and the standard library. It must never import
// internal/core (the engine is a leaf), and it must never pull in an LLM or a
// real tokenizer (the No-LLM-in-context invariant).
package contextpkg

// Token estimation for context manifests. The context engine must attach a
// measured token hint to every item it asks a host to load, but it may not
// depend on an LLM or a tokenizer (see the no-LLM-in-context invariant). These
// heuristics give a deterministic, IO-free approximation that is good enough to
// drive a soft budget and never panics.

// tokenBytesPerToken is the prose baseline: English text tokenizes at roughly
// four bytes per token, so the baseline estimate is ceil(len/4).
const tokenBytesPerToken = 4

// EstimateTokens returns a deterministic heuristic estimate of how many model
// tokens b would occupy.
//
// Heuristic:
//   - Baseline ceil(len(b)/4) — ~4 bytes per token for ordinary prose.
//   - Markdown surcharge: code fences and table rows are dense in structural
//     symbols (backticks and pipes) that each tend to tokenize on their own and
//     are undercounted by the prose average. Every such symbol adds half a
//     token (ceil), nudging code/table-heavy briefs upward.
//
// Properties (enforced by tests):
//   - Pure and total: identical input always yields identical output; never panics.
//   - Empty input yields 0.
//   - Monotonic in length: appending bytes never decreases the estimate (the
//     baseline grows with length and the surcharge counts only ever increase).
//
// It is intentionally a heuristic, not a tokenizer: estimates must be
// reproducible and dependency-free, not exact.
func EstimateTokens(b []byte) int {
	if len(b) == 0 {
		return 0
	}
	base := (len(b) + tokenBytesPerToken - 1) / tokenBytesPerToken
	var dense int
	for _, c := range b {
		if c == '`' || c == '|' {
			dense++
		}
	}
	surcharge := (dense + 1) / 2
	return base + surcharge
}

// EstimateTokensString is the string-typed convenience wrapper over
// EstimateTokens. It shares the same heuristic and properties.
func EstimateTokensString(s string) int {
	return EstimateTokens([]byte(s))
}
