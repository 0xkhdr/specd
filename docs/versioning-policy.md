# Versioning & Release Policy

## Semantic Versioning

`specd` versions are [SemVer](https://semver.org/) `vMAJOR.MINOR.PATCH`. The public contract is
the **CLI surface** (verbs, flags, exit codes, allowed phases — the command palette in
`internal/core/commands.go`) and the on-disk `.specd/` format:

- **MAJOR** — a breaking change to the CLI surface or the on-disk format.
- **MINOR** — a backward-compatible verb/flag addition, or a raised Go floor (below).
- **PATCH** — bug fixes and doc changes with no surface change.

The non-negotiable invariants (determinism, evidence integrity, no bypass flag, zero runtime
dependencies) are contract-level: breaking one is a MAJOR change and, in practice, out of scope.

## Go version floor

The **single authoritative source** for the minimum supported Go is the `go` directive in
`go.mod` (currently **1.26**). Everything else is derived from it:

- CI's test matrix pins `1.26.x` as its minimum leg.
- Docs state `Go 1.26+`; `scripts/docs-lint.sh` fails if any doc body disagrees with `go.mod`.

Raising the floor is a **MINOR** release, called out in the changelog. Never lower it silently.

## Cutting a release

1. Update `CHANGELOG.md`: move `Unreleased` items under the new `vX.Y.Z` heading and refresh the
   compare links.
2. Confirm the full local gate set is green (see [../TESTING.md](../TESTING.md)) and run the
   regression harnesses (`scripts/regress-*.sh`).
3. Tag: `git tag vX.Y.Z && git push origin vX.Y.Z`.
4. The release workflow runs `goreleaser` off the tag (`.goreleaser.yml`) to produce the static,
   reproducible binaries, checksums, and SBOM.

`internal/version.Version` is injected at build time from the tag
(`-ldflags -X …/internal/version.Version=vX.Y.Z`); untagged builds report a development version.

## Compatibility window and removal

Every deprecated surface (legacy config source, state schema, status projection, machine-output
route, unknown actor provenance, and task grammar) is tracked in the in-code compatibility registry
(`internal/core/compatibility.go`) with a stable diagnostic code, the version and date at which its
removal window opens, the replacement command, and an owner. The registry is a pure function of the
binary — it reaches no network and keeps no mutable metrics store.

`specd agents doctor --compat` projects the registry against local metadata and reports which
surfaces are still in active use, their replacement, and whether their removal window is met.
Removal is never automatic: a surface stays supported until **both** its minimum version and its
minimum date are reached **and** no active use remains. When any of those is unmet the surface is
retained and the unmet gate is named (`unmet-window-version`, `unmet-window-date`, or `active-use`).
Time alone never deletes support. A migrated surface stops being reported as active but stays in the
inventory as migration history.

---

**See also:** [../CHANGELOG.md](../CHANGELOG.md) · [../TESTING.md](../TESTING.md) ·
[command-reference.md](command-reference.md)
