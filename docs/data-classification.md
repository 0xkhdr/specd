# Data classification and boundary redaction (Domain 10, R4)

`specd` classifies every reference an adapter carries so the boundary layer can
decide what may cross a process, network, A2A, CI, or telemetry boundary and what
must be redacted. Domain 10 owns this taxonomy and the default-deny export;
Domain 06 owns enforcement.

The taxonomy is fixed (`internal/adapter/classify.go`, `AllClasses()`):

| Class | Meaning | Restricted by default |
|---|---|---|
| `public-metadata` | ids, digests, counts, status — safe to export | no |
| `spec-text` | requirement/design/task prose | no |
| `source-path` | file paths without content | no |
| `source-content` | raw source bytes | **yes** |
| `prompt` | model prompt / instruction text | **yes** |
| `tool-output` | captured tool/command output | no |
| `secret` | credentials, tokens, keys | **yes** |
| `telemetry` | measurements, traces, cost | no |
| `production-feedback` | runtime signals treated as untrusted data | no |

## Default export policy

The zero-value `ExportPolicy` exports **references and digests only**, plus inline
content of `public-metadata`. Every other class is reference+digest with inline
bytes stripped. Restricted classes (`secret`, `source-content`, `prompt`) never
cross a boundary in the clear unless a project policy explicitly opts them in via
`ExportPolicy.AllowInline`, and inline content is always size-bounded
(`MaxInlineBytes`, R4.3).

`RedactForExport` returns the sanitized references and a `Redaction` record for
every removal, so an audit sees that a transfer policy applied rather than a
silent drop. Export tests (`TestRedactForExport`) prove restricted inline content
is absent after export.
