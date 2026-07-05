# 0003 Integration Polish Scope

Status: accepted

Context:
Spec 11 ports v1's adoption/maintenance ergonomics: `mcp --config <host>` config
snippets, `init --repair/--refresh/--dry-run` managed-asset maintenance, and
handshake palette/config digests. v1 also shipped multi-host detection heuristics
(codex/cursor/…) and embedded per-host snippet templates.

Decision:
- **Hosts targeted now: `claude-code` only.** `mcp --config` is designed to grow
  (a `switch` on host + a `MCPHosts()` list), but we add a host only when it is
  actually targeted, not speculatively. Multi-host **detection** (sniffing which
  host is present) is deferred entirely — the operator names the host.
- **Snippet generation is typed, not templated.** The snippet is built from a Go
  structure and `json.MarshalIndent`, guaranteeing valid JSON with stable key
  order. This is a deliberate deviation from the spec's "embedded template" note:
  a typed builder cannot emit malformed JSON and is still golden-testable. Revisit
  if a host needs a non-JSON or comment-bearing config.
- **`--repair` and `--refresh` share one code path.** With a single
  `TemplateVersion` they are operationally identical (both re-sync managed regions
  from the current templates). The version stamp rides in every marker so a future
  version bump gives `--refresh` distinct restamping behavior; until then the
  distinction is semantic (reported label) only.

Consequences:
Adding a host = one `switch` case + one `MCPHosts()` entry + a golden test.
Bumping a template = raise `TemplateVersion`, which makes `init --refresh`
restamp older regions. Revisiting multi-host detection means a new spec, not an
edit to this record.
