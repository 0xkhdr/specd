---
name: specd-steering
description: Bootstrap and enrich the specd steering files. Teaches the agent to inspect the repo itself (manifests, dir tree, README, CI), detect the stack/layout, author product.md / structure.md / tech.md grounded in cited evidence, and set config.defaultVerify to the detected test command. Run after `specd init`, before authoring any spec, and whenever steering drifts.
---

# specd steering

After `specd init` the steering files `product.md`, `structure.md`, and `tech.md`
are stubs. **You** fill them — perceiving the repo is agent work, not harness work
(Foundational Split). No CLI command detects the stack or authors steering for you;
you do the inspection and authoring directly, then commit the files.

## Preconditions

- A `.specd/` exists (`specd init`). `reasoning.md` and `workflow.md` are frozen —
  do not edit them.

## Procedure

1. **Inspect the repo — read, do not guess.** Gather concrete evidence before
   writing a word:
   - **Manifests** for stack + dependencies: `go.mod`, `package.json`,
     `pyproject.toml` / `requirements.txt`, `Cargo.toml`, `pom.xml`, `Gemfile`, etc.
   - **Directory tree** (top level, dirs only) for layout and module boundaries.
   - **`README*` / `CONTRIBUTING*` / `docs/`** for product intent, scope, conventions.
   - **CI files** (`.github/workflows/*`, `Makefile`, `Taskfile`) for the real test/
     build/lint commands.
   Open every source before citing it. Do not invent users, scope, frameworks, or
   conventions — anything you cannot ground in a file does not belong in steering.

2. **Author `product.md`** — WHAT this product is, WHO uses it, the problem it
   solves, and explicit out-of-scope. Ground each claim in README/docs/manifest.

3. **Author `structure.md`** — the real top-level layout, which modules may depend
   on what, and the naming conventions a builder must follow. Derive from the dir
   tree and imports, not aspiration.

4. **Author `tech.md`** — the detected stack (languages, frameworks, versions) plus
   conventions (style, naming, error handling, testing). Cite the manifest/CI files
   that prove each fact.

5. **Set `config.defaultVerify`** to the test command you found in step 1 (e.g.
   `go test ./...`, `npm test`, `pytest`). This is what `specd verify` runs for
   spec-level verification, so it must be the real, working command. Edit
   `.specd/config.json`'s `defaultVerify` field directly.

## Notes

- Keep each file tight and repo-grounded; prefer precise prose over aspiration.
- Re-run this procedure whenever the stack, layout, or test command changes — stale
  steering misleads every later phase.
- Steering is your durable constitution (Principle 8): it outlives the chat session
  and grounds requirements, design, and tasks.
