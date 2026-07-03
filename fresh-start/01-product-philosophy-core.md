# Domain: Product & Philosophy Core

## 1. Purpose & value mapping
- **Principles served:** all eight, but this domain *owns* P1 (The Foundational
  Split — agent creates; harness enforces) and P8 (Steering as Constitution).
- **Paper concept realized:** the harness itself as the unit of engineering
  (`The_New_SDLC_With_Vibe_Coding.pdf`, Harness Engineering pp.26–34). The paper's
  claim — *"Most agent failures, examined honestly, are configuration failures"*
  (p.30) — is this domain's thesis. specd **is** a harness: a deterministic
  scaffold of tools + sandboxes + orchestration that carries the agent from the
  first planning document to production monitoring.
- **Core use case:** a developer working in *orchestrator* mode (paper pp.31–34)
  delegates well-specified work to an agent and reserves their own attention for
  the "80% problem" (p.34) — the ambiguous 20% (edge cases, integration points,
  correctness). specd is the machine that makes the 80% safely delegable by making
  every state change evidence-gated and every report deterministic.
- **If none → CUT:** N/A. This domain defines the keep/cut line for all others.

## 2. Current-state analysis (from specd)
- **Reference files read:** `README.md`, `docs/concepts.md`,
  `docs/validation-gates.md`, `docs/agent-integration.md`,
  `docs/contributor-guide.md`, `The_New_SDLC_With_Vibe_Coding.pdf` pp.15–18, 26–34.
- **What exists today:**
  - A stated philosophy matching the paper: agent authors specs; the harness (a
    zero-dependency Go binary) enforces them. `docs/concepts.md` lists the on-disk
    contract (`.specd/` root, `state.json` as machine truth, steering constitution,
    embedded templates).
  - The eight principles are already latent in the code: the split (agent-authored
    `tasks.md` vs harness-enforced gates), specs-on-disk, evidence gates, DAG waves,
    role injection, human approval transitions, deterministic reporting, steering.
- **Redundancy / complexity / drift found (evidence, not opinion):**
  - The product surface has grown to **29 registered commands** with a flywheel and
    a program tier that together dwarf the core: orchestration/program/ACP mass in
    `internal/core` is ~350K of source vs ~120K for the entire lifecycle+gates+parser
    core. The *philosophy* is lean; the *implementation* is not.
  - Feature drift examples: `promote` is a top-level command but implemented inside
    `eval.go` (`RunPromote`); `doc.go` is a registered-adjacent file with zero
    functions; Postgres/Redis backends contradict the stated "zero runtime deps,
    git-native" value (`docs/contributor-guide.md` invariant #1) even though they are
    build-tag-gated.

## 3. Fresh-start decision
- **Verdict per capability:**
  - Eight principles as invariants — **KEEP** (they are the product).
  - Zero-dependency, single-static-binary, git-native default — **KEEP** (P1; it is
    what makes the harness auditable and portable).
  - The 29-command surface — **REDESIGN → 16 verbs** (see `00-scope-triage.md`).
  - Postgres/Redis backends — **CUT** (contradict the core value; optional build tag
    only).
  - "specd is a harness, not a framework" positioning — **KEEP**, and make it the
    explicit acceptance test for every retained feature: *does this feature enforce
    the plan, or does it try to author it?* If it authors, it is agent work, not
    harness work (P1), and it is cut.
- **Minimal accurate surface:** the product is (a) a spec lifecycle on disk, (b) a
  deterministic gate engine, (c) an evidence ledger, (d) a context engine, (e) an
  agent-agnostic integration floor, and (f) an *opt-in* orchestration tier. Nothing
  else is core.
- **Architecture & flexibility improvements:**
  - Introduce a written **"harness charter"** (`docs/charter.md` in the new tree)
    that maps every shipped feature to exactly one of the seven harness components
    (instructions / tools / sandboxes / orchestration / guardrails / observability /
    context) and one principle. Unmapped code cannot merge — this operationalizes the
    subtractive bias as a standing gate, not a one-time cleanup.
  - Make the *conductor vs orchestrator* distinction (paper p.31) a first-class,
    documented execution mode (see domain 02: `mode` field), so the same binary serves
    a real-time IDE user and an async multi-agent delegator without two codebases.

## 4. Requirements (EARS-shaped) — seed for requirements.md
1. When a feature is proposed for the core binary, the system shall require it to map
   to exactly one of the seven harness components and at least one of the eight
   principles, or be rejected.
2. The system shall build as a single statically linked binary with zero runtime Go
   module dependencies (`go.mod` has no `require` block).
3. When built with default tags, the system shall use only the git-native state
   backend and shall not link any external database driver.
4. The system shall treat all agent-authored artifacts (`requirements.md`,
   `design.md`, `tasks.md`) as untrusted input and enforce, never author, their
   content.
5. When a user runs the binary with no arguments, the system shall print the 16-verb
   command surface and exit `0`.
6. The system shall keep every human-facing report a pure projection of `state.json`
   with no model invocation in its code path.

## 5. Design notes — seed for design.md
- **Module boundaries:** `internal/core` (domain logic + on-disk contracts),
  `internal/context` (context engine, kept separate to avoid a `core→context→core`
  cycle), `internal/cli` (zero-dep arg parser), `internal/cmd` (one file per verb),
  `internal/integration` (host adapters), plus an opt-in `internal/orchestration`
  tier compiled in always but inert unless enabled.
- **Key types:** none new here; this domain governs *what may exist*, enforced by the
  charter + the registry↔help single-source guard (`TestRegistryMatchesHelp`).
- **Invariants to preserve:** zero-dep; embedded templates via the single `go:embed`
  in `embed.go`; atomic writes; CAS on revision; reentrant per-spec lock; `ParseTasks`
  byte round-trip. (Names corrected from the brief: parser is `ParseTasks`, not
  `ParseTasksMd`; the manifest builder `BuildContextManifest` lives in
  `internal/context`, not core — see `00-decisions.md`.)
- **External interfaces:** the CLI verb surface; the MCP tool surface (domain 07);
  the `.specd/` on-disk layout (domain 02).

## 6. Proposed task DAG — seed for tasks.md

### Wave 0 — charter & guardrails
| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T1.1 | craftsman | `docs/charter.md` | — | `test -f docs/charter.md && grep -q 'harness component' docs/charter.md` | Charter maps all 16 verbs to a component + principle |
| T1.2 | craftsman | `go.mod` | — | `test -z "$(go list -m all | grep -v '^'$(go list -m)'$')"` | No `require` deps |
| T1.3 | craftsman | `main.go`, `internal/cli/args.go` | T1.1 | `go run . 2>&1 | grep -c . ` prints 16 verbs | Bare invocation lists exactly the 16 core verbs |
| T1.4 | validator | `internal/core/commands_test.go` | T1.3 | `go test ./internal/core -run TestRegistryMatchesHelp` | Guard passes; help cannot drift |

## 7. Risks, open questions, cross-domain dependencies
- **Risk:** the charter becomes a rubber stamp. Mitigation: wire it as a lint over
  the registry in CI, not a doc convention.
- **Open question:** is `conductor` mode (real-time) worth any dedicated code, or is
  it purely "the same core with orchestration disabled"? (Leaning: the latter — see
  domain 02.)
- **Cross-domain deps:** this domain constrains all others. Its `mode` field is
  designed in domain 02; its component map is exercised by domains 03/05/08/09.
