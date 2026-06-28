# specd workflow wrappers

Optional UX glue for native `specd` commands. Enforcement remains in native `specd`: state transitions, evidence gates, Brain/Pinky proof boundaries, and verification records are not reimplemented here.

## Install/use

Shell wrapper:

```sh
. scripts/specd-workflow.sh
specd_workflow init --agent auto
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

| Wrapper | Native command |
|---|---|
| `init` | `specd init` |
| `steer show/status` | read `.specd/steering/*` only |
| `steer edit/bootstrap` | canonical steering files only |
| `spec new` | `specd new` |
| `spec list` | `specd status --json`, fallback `specd status` |
| `spec continue` | `specd context`; executing specs also call `specd next` |
| `spec check/approve/context/next/waves/report` | matching native `specd` command |
| `pinky-brain status` | config read + `specd brain resume --list --json` when available |
| `pinky-brain enable` | `specd init --repair --orchestration ...` |
| `pinky-brain disable` | safe config update only; active sessions untouched |
| `pinky-brain start/run/step/pause/resume/cancel/compact` | `specd brain ...` |
| `pinky-brain workers` | read-only session/worker view |

## Safety model

Wrappers must not edit `state.json`, flip `tasks.md` checkboxes, run `specd task --status complete`, or forge `specd pinky report`. Complete tasks only after verification:

```sh
specd verify my-feature T1
specd task my-feature T1 --status complete --evidence "verification ref / test output"
```

Brain/Pinky session actions are POSIX-only; on native Windows use WSL. Worker telemetry is not proof of correctness; Pinky reports must bind to native verification records.

## Tests

Run wrapper tests directly:

```sh
python3 scripts/test-specd-workflow.py
```

`make test` runs wrapper tests before the Go race suite.
