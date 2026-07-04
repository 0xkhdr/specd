# Context

`specd context <slug> <task> --json` emits a deterministic, read-targeted manifest.
The manifest uses four modes (`craftsman`, `validator`, `scout`, `scribe`) and four
items: spec, tasks, task, and role. Token estimates use a stdlib heuristic only:
`ceil(bytes/4)`, never an LLM.
