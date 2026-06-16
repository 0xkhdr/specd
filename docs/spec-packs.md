# Spec Packs

A **spec pack** is a declarative scaffold bundle: a set of files to write into a
project, plus template variables. Packs let teams share a steering/role baseline
without copy-paste. They are intentionally **not executable** — a pack can only
ever write files.

```sh
specd init --list-packs                 # list embedded built-in packs
specd init --pack minimal               # apply a built-in by name
specd init --pack go-service --force    # overwrite existing files
specd init --pack https://example.com/pack.json --sha256 <hex>   # remote, pinned
```

## Manifest contract (`pack.json`)

```json
{
  "name": "go-service",
  "version": "1.0.0",
  "description": "Steering for a stdlib-first Go service.",
  "files": [
    { "path": ".specd/steering/tech.md", "content": "# Tech — {{TITLE}}\n..." }
  ],
  "vars": { "TITLE": "Project" }
}
```

| Field | Required | Meaning |
|-------|----------|---------|
| `name` | yes | Pack identifier (matches `--pack <name>` for built-ins). |
| `version` | yes | Semantic version of the pack. |
| `description` | no | One-line summary shown by `--list-packs`. |
| `files` | yes (≥1) | Files to write. Each has `path` and inline `content`. |
| `vars` | no | `{{KEY}}` placeholders substituted into every file's content. |

### Hard rules (fail-closed)

- **Declarative only.** Any executable-intent key (`hooks`, `exec`, `run`,
  `command(s)`, `script(s)`, `pre/post-install`, `pre/post-apply`) is rejected,
  as is any unknown field (`DisallowUnknownFields`). Applying a pack carries no
  code-execution surface.
- **Path safety.** Every `path` must be relative, canonical, and stay within the
  project root. Absolute paths, `..`, non-canonical forms, and duplicate paths
  are rejected. A malformed manifest yields **no** partial pack.

## Resolution

- A **bare name** resolves against the embedded built-in packs.
- An **http(s) URL** is a remote pack and **must** carry a pinned `--sha256`
  digest. The bytes are downloaded (bounded to 1 MiB), hashed, and compared
  before parsing — on any mismatch, missing pin, or non-200 response, **nothing
  is written**. This mirrors `specd update`'s SHA256SUMS contract.

## Apply semantics (transactional)

`specd init --pack` is all-or-nothing:

1. **Plan** — every path is re-validated and checked for collisions with no side
   effects. Without `--force`, a pre-existing target is a hard error and nothing
   is written.
2. **Apply** — files are written atomically. If any write fails, every file
   created in this call is removed, so a failed apply never leaves a partial
   scaffold. (Files you already had are never destroyed on rollback.)

Omitting `--pack` leaves `specd init` byte-unchanged — pack support is purely
additive.
