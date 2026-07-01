# specd workflow wrappers

Optional UX glue for native `specd` commands. Wrappers make common slash-style workflows easier, but enforcement stays in native `specd`: state transitions, evidence gates, Brain/Pinky proof boundaries, and verification records are not reimplemented here.

## Install/use

Shell wrapper:

```sh
. scripts/specd-workflow.sh
specd_workflow init --agent auto
specd_workflow steer show
specd_workflow spec list
specd_workflow pinky-brain status
```

Slash-command hosts can map `/init`, `/steer`, `/spec`, and `/pinky-brain` to `specd_workflow init|steer|spec|pinky-brain`.

Python wrapper:

```sh
python3 scripts/specd-workflow.py init --agent auto
python3 scripts/specd-workflow.py steer show
python3 scripts/specd-workflow.py spec continue my-feature
python3 scripts/specd-workflow.py pinky-brain status
```

## Native command mapping

| Wrapper | Native command / behavior |
|---|---|
| `init` | `specd init` |
| `steer show/status/memory` | read `.specd/steering/*` only |
| `steer edit/bootstrap` | canonical steering files only |
| `spec new` | `specd new` |
| `spec list` | `specd status --json`, fallback `specd status` |
| `spec continue` | `specd context`; executing specs also call `specd next` |
| `spec check/approve/context/next/waves/report` | matching native `specd` command |
| `spec mode` | `specd mode` only when native support is detected; otherwise prints fallback guidance |
| `pinky-brain status` | config read + `specd brain resume --list --json` when available |
| `pinky-brain enable` | `specd init --repair --orchestration ...` |
| `pinky-brain disable` | safe config update only; active sessions untouched |
| `pinky-brain start/run/step/pause/resume/cancel/compact` | `specd brain ...` |
| `pinky-brain workers` | read-only session/worker view |

## `/init` examples

Interactive when stdin is a TTY:

```sh
specd_workflow init
```

CI/non-interactive dry run:

```sh
specd_workflow init --non-interactive --yes --dry-run --agent none --orchestration none
```

Repair or refresh managed assets:

```sh
specd_workflow init --repair --agent auto
specd_workflow init --refresh --agent auto
```

Enable orchestration scaffold during init:

```sh
specd_workflow init --agent auto --orchestration manual --workers 4 --retries 2 --timeout 120 --role-mode inline --sandbox none
```

Native failures propagate unchanged.

## `/steer` examples

Canonical steering files are `reasoning.md`, `workflow.md`, `product.md`, `tech.md`, `structure.md`, and `memory.md`. `show` accepts only these basenames or `all`; paths such as `../config.json` are rejected.

```sh
specd_workflow steer status
specd_workflow steer show product.md
specd_workflow steer show all
specd_workflow steer memory
EDITOR=vim specd_workflow steer edit tech.md
specd_workflow steer bootstrap --dry-run
printf '%s\n' '# Product' | specd_workflow steer bootstrap product.md --stdin
```

`memory` prints `.specd/steering/memory.md` when present. Missing memory is a warning only; wrapper creates no file.

## `/spec` examples

```sh
specd_workflow spec list
specd_workflow spec new auth --title "Auth"
specd_workflow spec continue auth
specd_workflow spec check auth
specd_workflow spec approve auth
specd_workflow spec next auth
specd_workflow spec report auth
specd_workflow spec mode auth
specd_workflow spec mode auth --set orchestrated
```

`spec mode` checks native support through `specd help --json` or help text. If unsupported, wrapper exits with a gate failure and tells you to use the simple workflow or upgrade native `specd`; it does not edit files.

Evidence warning: wrappers never complete tasks for you. Use native verification before completion:

```sh
specd verify auth T1
specd task auth T1 --status complete --evidence "verification ref / test output"
```

## `/pinky-brain` examples

```sh
specd_workflow pinky-brain status
specd_workflow pinky-brain enable --policy manual --workers 4 --retries 2 --timeout 120
specd_workflow pinky-brain disable
specd_workflow pinky-brain start auth --policy manual --workers 4
specd_workflow pinky-brain run auth --session s1 --worker-cmd './worker.sh'
specd_workflow pinky-brain step auth --session s1
specd_workflow pinky-brain pause --session s1
specd_workflow pinky-brain resume --session s1
specd_workflow pinky-brain cancel --session s1
specd_workflow pinky-brain compact --session s1
specd_workflow pinky-brain workers
```

Brain/Pinky session actions are POSIX-only; on native Windows use WSL. `status` remains read-only. Worker telemetry is not proof of correctness; Pinky terminal reports must bind to native verification records.

## Safety model

Wrappers must not edit `state.json`, flip `tasks.md` checkboxes, run `specd task --status complete`, or forge `specd pinky report`. Complete tasks only through native `specd verify` + `specd task --status complete --evidence ...`.

## Tests

Run wrapper tests directly:

```sh
python3 scripts/test-specd-workflow.py
```

`make test` runs wrapper tests before the Go race suite.
