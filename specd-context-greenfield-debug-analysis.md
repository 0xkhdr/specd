# specd context failure for greenfield task outputs

## Executive summary

`specd context` rejects executable tasks when a file listed in the task's `files` column does not exist yet, even though that column is the task's authorized create/modify scope and the separate `context` column contains the required pre-existing inputs.

This blocks normal greenfield tasks before an agent can receive task authority. The failure reproduces for multiple independent tasks, so it is not specific to the provider implementation or its requirements.

The likely defect is in context source assembly or validation: task output paths are being added to a collection whose members are all validated as required existing sources. The fix should preserve output paths in the authority/scope manifest while only requiring declared reference inputs to be readable.

## Environment

- Workspace: `/var/www/html/rai/up/aido`
- Spec: `aido-documentary-agent`
- Spec phase/status: `execute` / `executing`
- Spec revision observed during bootstrap: `5`
- specd version: `1.0.0`
- specd commit: `0505f816a852d09c6a7316927c261fc9dd837bcc`
- Host: Linux amd64
- Date observed: 2026-07-21

## Primary reproduction

From the workspace root:

```bash
specd handshake bootstrap aido-documentary-agent --json
specd status aido-documentary-agent --guide --json
specd context aido-documentary-agent T3 --json
```

The first two commands succeed. The third exits unsuccessfully with:

```text
required source: context source "internal/aido/provider/provider.go": missing or unreadable
```

At this point neither `internal/aido/provider/provider.go` nor `internal/aido/provider/provider_test.go` exists. That is expected: creating them is T3's work.

## Task definition evidence

T3 in `.specd/specs/aido-documentary-agent/tasks.md` declares:

| Field | Value |
|---|---|
| `id` | `T3` |
| `role` | `craftsman` |
| `files` | `internal/aido/provider/provider.go`, `internal/aido/provider/provider_test.go` |
| `depends-on` | `T1` |
| `verify` | `go test ./internal/aido/provider` |
| `capabilities` | `read,write` |
| `context` | `aido-architecture.md`, `requirements.md`, `design.md`, `internal/aido/workspace/workspace.go` |

The rejected path is present in `files`, not `context`.

All declared T3 context inputs exist. T1 is marked complete, and its output `internal/aido/workspace/workspace.go` exists. Thus the failure is not a missing dependency or missing declared reference input.

## Independent reproduction

T5 is another greenfield task whose dependencies are satisfied independently of T3. Run:

```bash
specd context aido-documentary-agent T5 --json
```

Observed result:

```text
required source: context source "internal/aido/agent/agent.go": missing or unreadable
```

T5 declares `internal/aido/agent/agent.go` in `files`; it is not listed in T5's `context`. This second reproduction shows the behavior is systematic for missing task outputs rather than caused by T3 content.

## Contract evidence from managed guidance

`.specd/steering/workflow.md` says execution must "Touch only a task's declared `files:`." Therefore `files` is the writable task boundary. For a greenfield task, some or all paths in that boundary necessarily do not exist before execution.

`.specd/skills/execute/SKILL.md` requires the agent to load task context, stay within declared files, implement the task, verify it, and complete it. The context command is consequently the gate that must grant authority before those files are created.

`.specd/skills/tasks/SKILL.md` requires both `files` and `context` fields in task rows. The separate columns strongly indicate separate concerns:

- `files`: authorized task outputs/change scope.
- `context`: additional existing reference inputs.

`.specd/steering/reasoning.md` also says to read the task's declared files. That is compatible with reading declared files *when they already exist*, but it cannot imply that every output must pre-exist: doing so makes creation tasks impossible.

## Expected behavior

For each task path:

1. Preserve every `files` entry in the returned authority manifest as an authorized writable path.
2. If a `files` entry exists, it may be included as readable current implementation context.
3. If a `files` entry does not exist, represent it as an authorized prospective output and do not fail context construction.
4. Require every declared `context` input to exist and be readable, unless the format explicitly supports optional context.
5. Continue enforcing dependency, role, capability, digest, traversal, and workspace-boundary checks.

For T3, `specd context ... T3 --json` should succeed and grant read/write authority limited to the two provider paths while loading the four existing context inputs.

## Actual behavior

Context construction fails before emitting task authority because the first missing `files` path is reported as a required `context source`.

This creates a deadlock:

```text
context authority required before edit
        -> output file must exist before context authority
        -> agent cannot legally create output file
```

Creating placeholder files manually would bypass the authority gate and hide the defect. It would also weaken the guarantee that all task changes occur under an issued task context.

## Likely fault boundary

The exact implementation is not present in this repository, so the following is an inference from CLI behavior.

The context builder likely combines paths resembling:

```text
task.files + task.context + managed spec/role/steering sources
```

and then sends the combined collection through a validator equivalent to:

```text
for each source: require readable regular file
```

Either of these designs would produce the observed error:

- task output paths are mislabeled as required context sources; or
- the source model lacks an `optional_if_missing` / `prospective_output` distinction.

The correction belongs where task rows are converted into bounded context/authority sources, not in this project's task plan.

## Suggested minimal fix

Keep two path sets through context construction:

```text
requiredInputs = task.context + required managed/spec/role sources
authorizedFiles = task.files
```

Validate all paths for normalization, traversal, and workspace containment. Require readability only for `requiredInputs`. For each `authorizedFiles` path, read it if present; otherwise retain the normalized path in authority without creating or reading it.

Do not solve this by creating empty output files during `specd context`; a read command should remain non-mutating, and empty placeholders are not meaningful source evidence.

## Regression tests

Add the smallest table-driven coverage around context construction or the CLI route:

1. **Missing authorized output succeeds**
   - Task has `files: new/module.go` and valid existing context.
   - `new/module.go` does not exist.
   - Context succeeds and authority includes `new/module.go`.

2. **Existing authorized output is loaded**
   - Task output exists.
   - Context succeeds and includes its current contents/digest as applicable.

3. **Missing declared context fails**
   - Task has `context: missing-design.md`.
   - Context fails with a precise required-input error.

4. **Unreadable declared context fails**
   - Existing declared context cannot be read.
   - Context fails closed.

5. **Output traversal still fails**
   - Task has `files: ../outside.go` or an escaping symlink target.
   - Context refuses authority.

6. **Mixed existing and new outputs succeeds**
   - One task output exists and another is new.
   - Existing content is loaded; both paths remain authorized.

7. **Read-only task behavior remains explicit**
   - A task with no writable/new output follows the current read-only authority rules.

## Acceptance criteria for the fix

- The T3 and T5 commands above return successful JSON context without placeholder files.
- Missing entries from the `context` column still fail closed.
- Missing entries from the `files` column appear in task authority but are not treated as required readable sources.
- Existing output files remain available as bounded context.
- No context command mutates the filesystem.
- Traversal, symlink escape, capability, dependency, digest, and declared-file gates remain intact.
- Existing context-related test suites pass.

## Project-side conclusion

The `aido-documentary-agent` task rows correctly place new implementation paths under `files` and existing dependencies under `context`. Changing the plan to pre-create placeholders would be a workaround, not a correct repair, and would make every greenfield task require an out-of-band mutation before specd can authorize it.

