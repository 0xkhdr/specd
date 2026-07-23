# Configuration

Specd resolves the nearest project root after resolving symlinks, then inspects
these files in order:

1. `.specd/config.yaml` (canonical)
2. `project.yml` (legacy)
3. `project.yaml` (legacy)

`specd init` creates only `.specd/config.yaml`, validates it with the runtime
parser, and never replaces or relocates an existing configuration file under
any of these spellings. The file is operator-owned and outside managed refresh
regions.

Exactly one effective policy is accepted. If multiple files normalize to equal
key/value policy, the first file above is selected and legacy files produce a
deprecation diagnostic. If values differ, Specd fails closed and reports only
the conflicting keys and source paths. A malformed canonical file also fails;
Specd never falls back to legacy policy.

Resolution reports the canonical root, selected source and kind, SHA-256 source
digest, normalized effective digest, duplicate source paths, and deprecation
messages. Environment overrides are applied after file policy by `LoadConfig`;
diagnostics identify override variable names and never expose secret values.

The parser intentionally supports only top-level scalars and one level of
two-space-indented scalar sections. It rejects tabs, sequences, flow
collections, anchors, aliases, duplicate keys, and multiple YAML documents with
an exact `config line N` error. Outside matching single or double quotes, `#`
starts a comment; quote the whole scalar when a literal hash is data. Whole-line
comments are ignored. Specd uses only the Go standard library.

List and compound values use these separators:

| Key | Separator |
|---|---|
| `verify.trivial` | comma between commands |
| `routing.classes`, `routing.fallback` | comma between class names |
| `routing.recommendations` | comma between `complexity=class` entries |
| `routing.class_capabilities` | semicolon between class entries; `+` between capabilities |
| `environments.<name>` | semicolon between fields; `+` between `criteria` values |

Spaces around entries are trimmed. These separators are part of the flat
configuration grammar; YAML sequences and flow collections remain unsupported.

## Migrating legacy configuration

Preview the exact source, canonical target, backup path, permissions, digests,
effective-value comparison, conflicts, and ordered operations without writing:

```bash
specd config migrate --dry-run
```

Run `specd config migrate` to atomically install and revalidate
`.specd/config.yaml`, then move the legacy file to the non-overwriting
`project.yml.specd-v1.bak` (or `project.yaml.specd-v1.bak`). Replaying the
command after completion reports the completed result and writes nothing.

When both legacy spellings exist they must normalize to equal values, and the
operator must select one explicitly with `--source project.yml` or
`--source project.yaml`. Conflicts, malformed input, unreadable files, and an
existing backup fail before mutation. Legacy reads remain supported with a
stable deprecation diagnostic for at least two minor releases.
