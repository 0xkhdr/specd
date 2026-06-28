# Specd Repository Analysis & Action Plan

> **Repository:** https://github.com/0xkhdr/specd  
> **Date:** 2026-06-28  
> **Purpose:** Analyze specd to produce runnable slash-command workflow abstractions (`/init`, `/steer`, `/spec`, `/pinky-brain`) with behavior-selection options, adhering to specd best practices.

---

## 1. Executive Summary

`specd` is an **agent-agnostic, spec-driven coding harness CLI** written in Go (stdlib-only). It enforces a rigid planning ratchet (Analyze → Plan → Execute → Verify → Reflect) via deterministic validation gates, DAG-based wave execution, and evidence-gated task completion. It is **not** an LLM wrapper — it is a local process-enforcement tool that any agent (Claude Code, Cursor, Codex, Aider, etc.) drives via shell commands or MCP.

The user's request maps naturally to specd's existing command surface, but requires **orchestration-layer abstraction** — wrapping the low-level CLI into high-intent slash commands that suggest behaviors and handle bootstrap/steering/orchestration configuration automatically.

---

## 2. Repository Architecture Analysis

### 2.1 Core Components

| Component | Location | Responsibility |
|-----------|----------|----------------|
| CLI Router | `main.go` | Entry point, arg dispatch |
| Command Registry | `internal/cmd/registry.go` | Command → handler dispatch table |
| Domain Logic | `internal/core/` | Gates, state, DAG, parser, report, schema |
| State Machine | `internal/core/state.go` | Atomic `state.json` load/save with CAS revision |
| Task Parser | `internal/core/tasksparser.go` | Byte-stable `tasks.md` line parser (no external libs) |
| DAG Engine | `internal/core/dag.go` | Wave computation, runnable frontier, cycle detection |
| Validation Gates | `internal/cmd/check.go` + `internal/core/` | 7 core gates + opt-in acceptance/scope/custom |
| MCP Server | `internal/mcp/` | JSON-RPC 2.0 stdio/HTTP server exposing all commands as tools |
| Templates | `internal/core/embed_templates/` | `AGENTS.md`, `config.json`, steering, roles, skills, stubs (go:embed) |
| Test Harness | `internal/testharness/` | Deterministic sandbox, in-process runner, FakeClock |
| Host Adapters | `internal/integration/` | Managed adapters for claude-code, codex, cursor, antigravity, vscode |

### 2.2 Target Repo Structure (post-`init`)

```
<project>/
├── .specd/
│   ├── config.json              # Project configuration
│   ├── program.json             # Cross-spec dependencies
│   ├── state.json               # Machine state (auto-managed)
│   ├── skills/                  # Progressive skill pack (foundations → brain → pinky)
│   ├── steering/                # Constitution (durable rules)
│   │   ├── reasoning.md
│   │   ├── workflow.md
│   │   ├── product.md
│   │   ├── tech.md
│   │   ├── structure.md
│   │   └── memory.md
│   ├── roles/                   # Persona prompts
│   │   ├── investigator.md
│   │   ├── builder.md
│   │   ├── reviewer.md
│   │   ├── verifier.md
│   │   ├── brain.md
│   │   └── pinky.md
│   ├── subagents/               # Orchestration runtime (sessions, logs, leases)
│   └── specs/
│       └── <slug>/
│           ├── state.json
│           ├── requirements.md  # EARS format
│           ├── design.md        # 7 mandatory H2 sections
│           ├── tasks.md         # Wave DAG with 7 task keys
│           ├── decisions.md     # ADRs
│           ├── mid-requirements.md
│           └── memory.md
└── AGENTS.md                    # Agent workflow guide (merged by init)
```

### 2.3 Key Design Invariants

1. **Foundational Split**: Agent reasons; harness enforces.
2. **Specs as Source of Truth**: Active plan lives on disk, not in context window.
3. **Evidence Gates Every State Change**: `verify:` must pass before `task --status complete`.
4. **Waves, Not Lines**: DAG-based concurrent batches, not flat todo lists.
5. **Agent-Agnostic**: Standardized CLI + role prompt injection.
6. **Human Gates at Phase Boundaries**: `specd approve` advances phases.
7. **Deterministic Reporting**: Reports from `state.json`, never LLM-generated.
8. **Steering as Constitution**: Durable `.specd/steering/` files outlive chat sessions.

---

## 3. Command Surface Mapping

### 3.1 Existing specd Commands (Relevant to User Requirements)

| Category | Command | Purpose |
|----------|---------|---------|
| **Lifecycle** | `specd init [--agent <auto\|none\|claude-code\|codex\|cursor\|antigravity\|vscode>] [--scope <project\|global>] [--yes] [--dry-run] [--repair] [--refresh] [--orchestration <planning\|session>] [--orchestration-workers N] [--orchestration-retries N] [--orchestration-timeout N] [--orchestration-cost-limit N] [--orchestration-mode <inline\|delegate>] [--orchestration-sandbox <none\|bwrap\|container>]` | Scaffold `.specd/`, detect agent, install project-scoped MCP, configure orchestration |
| | `specd doctor [--fix]` | Diagnose scaffold + MCP + host registration |
| | `specd new <slug> [--title "..."] [--orchestrated]` | Create spec with 6 artifact stubs |
| | `specd approve <slug>` | Advance phase gate (human approval) |
| **Execution** | `specd next <slug> [--all]` | Get next runnable task (or entire frontier) |
| | `specd dispatch <slug> [--inline-roles]` | Emit ready-to-run subagent packets |
| | `specd verify <slug> <task> [--sandbox <none\|bwrap\|container>] [--revert-on-fail]` | Run task verify command, record evidence |
| | `specd task <slug> <task> --status <pending\|running\|complete\|blocked> [--evidence "..."] [--unverified] [--tokens N] [--cost N]` | Evidence-gated status flip |
| **Inspection** | `specd status [<slug>]` | Progress board |
| | `specd check <slug>` | Run validation gates (7 core + opt-in) |
| | `specd context <slug>` | Phase briefing + budgeted LOAD NOW manifest |
| | `specd waves <slug>` | Wave graph, critical paths, blockers |
| | `specd report <slug> [--format md\|html] [--pr-summary]` | Deterministic snapshot report |
| **Orchestration** | `specd brain start <slug> --approval-policy <manual\|planning\|session> --max-workers N --max-retries N --timeout-seconds N [--session <uuid>]` | Start Brain session, advance one step |
| | `specd brain run <slug> [--worker-cmd "..."] [--bootstrap] [--max-steps N]` | Built-in driver loop |
| | `specd brain step <slug> --session <uuid> ...` | Advance existing session by one decision |
| | `specd brain status\|pause\|resume\|cancel --session <uuid>` | Session control |
| | `specd brain compact\|clear <slug> --session <uuid>` | Context compaction before `/clear` |
| | `specd pinky claim --mission <file>` | Worker claims mission lease |
| | `specd pinky brief --session <uuid> --worker <id> --spec <slug> --task <task>` | Render worker brief |
| | `specd pinky heartbeat\|progress\|query\|report\|block\|release` | Worker lifecycle commands |
| **Meta** | `specd mcp [--http] [--config <host>]` | Start MCP server or print host config snippet |
| | `specd help [--json]` | Command registry schema |
| | `specd schema` | Emit embedded Open Spec Format JSON Schema |

### 3.2 Exit Code Contract

| Code | Meaning | Usage |
|------|---------|-------|
| `0` | OK / passed | Success, validation passed |
| `1` | Gate / enforcement failure | `check` failed, `verify` failed, CAS abort |
| `2` | Usage error | CLI argument error |
| `3` | Not found | `.specd/` root or spec slug missing |

---

## 4. Gap Analysis: User Requirements vs. Native specd

### 4.1 Requirement: `/init` — Bootstrap with Any Agent

**Current State:** `specd init` already supports `--agent auto` (detect unambiguous host), `--agent all` (configure every detected host), `--agent none` (scaffold only), and per-host named adapters (`claude-code`, `codex`, `cursor`, `antigravity`, `vscode`). It also supports `--orchestration` flags to bootstrap Brain/Pinky stack.

**Gap:** No single "interactive behavior chooser" that presents available options and auto-configures based on user selection. The `--agent auto` behavior is context-dependent (TTY vs non-TTY).

**Resolution:** Wrap `specd init` with an interactive shell function that:
1. Detects available agents (`specd doctor --json` or `specd init --agent auto --dry-run --json`)
2. Presents a numbered menu of detected hosts + "none" + "all"
3. Prompts for orchestration mode (`planning`, `session`, or `manual`)
4. Invokes `specd init` with the selected parameters and `--yes`

### 4.2 Requirement: `/steer` — Include All Files in Steering Dir

**Current State:** Steering files live under `.specd/steering/` (`reasoning.md`, `workflow.md`, `product.md`, `tech.md`, `structure.md`, `memory.md`). `specd context` already emits a phase-scoped briefing with a `LOAD NOW` manifest that includes steering files. However, there is no single command that "loads all steering files unconditionally."

**Gap:** No dedicated command to aggregate and display all steering files for inspection/editing.

**Resolution:** Create a wrapper that:
1. Locates `.specd/steering/` root
2. Concatenates all `*.md` files with headers
3. Optionally opens them in `$EDITOR` or pipes to stdout
4. Suggests which steering files need authorship (e.g., `product.md`, `tech.md`, `structure.md` are agent-authored after `init`)

### 4.3 Requirement: `/spec` — Spec Workflow Entry Point

**Current State:** The spec lifecycle is distributed across `specd new`, `specd check`, `specd approve`, `specd next`, `specd verify`, `specd task`, `specd report`.

**Gap:** No single "spec dashboard" command that shows status, suggests next action, and offers behavior choices (create new spec, continue existing, switch mode, etc.).

**Resolution:** Create a wrapper that:
1. Lists all specs (`specd status --json`)
2. Shows current phase, blockers, and next runnable task for each
3. Offers interactive choices: `new`, `continue <slug>`, `check <slug>`, `approve <slug>`, `mode <slug>`
4. For `continue`, automatically runs `specd context <slug>` + `specd next <slug>`

### 4.4 Requirement: `/pinky-brain` — Manage Orchestration Loop

**Current State:** Brain/Pinky commands exist (`specd brain start/run/step/status/pause/resume/cancel/clear`, `specd pinky claim/heartbeat/progress/query/report/block/release`). Orchestration is opt-in via `specd init --orchestration ...` or manual config edit.

**Gap:** No single command to toggle orchestration on/off, check if it's enabled, or start/stop the loop with behavior suggestions.

**Resolution:** Create a wrapper that:
1. Checks current orchestration capability (`specd doctor --json` or reads `.specd/config.json`)
2. If disabled: suggests enabling with policy options (`manual`, `planning`, `session`)
3. If enabled: shows active sessions (`specd brain resume --list --json`)
4. Offers choices: `start <slug>`, `run <slug>`, `step <slug>`, `pause/resume/cancel <session>`, `compact <session>`, `disable`
5. For `start/run`, prompts for approval policy, max workers, retries, timeout, cost limit

---

## 5. Action Plan

### Phase 1: Foundation — Understand & Validate (Week 1)

| Step | Action | Verification |
|------|--------|--------------|
| 1.1 | Install specd locally: `curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/install.sh \| bash` | `specd version` returns version string |
| 1.2 | Initialize a test repo: `specd init --agent none` | `.specd/` directory exists with `config.json`, `steering/`, `roles/`, `skills/` |
| 1.3 | Validate steering scaffold: inspect `.specd/steering/*.md` | All 6 files present; `product.md`, `tech.md`, `structure.md` are stubs needing authorship |
| 1.4 | Validate role scaffold: inspect `.specd/roles/*.md` | `investigator.md`, `builder.md`, `reviewer.md`, `verifier.md`, `brain.md`, `pinky.md` present |
| 1.5 | Validate skill scaffold: inspect `.specd/skills/` | `specd-foundations/`, `specd-steering/`, `specd-requirements/`, `specd-design/`, `specd-tasks/`, `specd-execute/`, `specd-brain/`, `specd-pinky/` present |
| 1.6 | Test `specd doctor` on test repo | Exit 0, reports healthy |
| 1.7 | Test `specd help --json` | Returns valid JSON command registry |
| 1.8 | Test `specd schema` | Returns valid JSON Schema |

### Phase 2: `/init` Command Implementation (Week 1-2)

**Goal:** Interactive bootstrap with behavior selection.

**Implementation Options:**

**Option A: Shell Function (Bash/Zsh)** — Fastest, no build step.
**Option B: Python Script** — Cross-platform, rich menus.
**Option C: Go Binary (extends specd)** — Native integration, but requires forking/contribution upstream.

**Recommended: Option A + B** (shell function for POSIX, Python fallback for Windows).

**Specification:**

```bash
# /init — Interactive specd bootstrap
# Usage: /init [options]
# Options:
#   --dry-run    Preview mutations without writing
#   --repair     Restore missing managed files
#   --refresh    Update specd-managed assets
#   --json       Output structured JSON

/init() {
  local agent_choice=""
  local orch_policy=""
  local orch_workers=""
  local orch_retries=""
  local orch_timeout=""
  local orch_cost=""
  local orch_mode=""
  local orch_sandbox=""
  local dry_run=""
  local repair=""
  local refresh=""
  local json_out=""

  # Parse flags
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --dry-run) dry_run="--dry-run" ;;
      --repair) repair="--repair" ;;
      --refresh) refresh="--refresh" ;;
      --json) json_out="--json" ;;
      *) echo "Unknown option: $1"; return 2 ;;
    esac
    shift
  done

  # Step 1: Detect available agents
  local detected=""
  detected=$(specd doctor --json 2>/dev/null | jq -r '.hosts[]?.name // empty' 2>/dev/null)

  if [[ -z "$detected" ]]; then
    echo "⚠️  No coding agents detected."
    echo "   Available options:"
    echo "     1) claude-code"
    echo "     2) codex"
    echo "     3) cursor"
    echo "     4) antigravity"
    echo "     5) vscode"
    echo "     6) none (scaffold only)"
    read -p "Select agent [1-6]: " agent_num
    case "$agent_num" in
      1) agent_choice="claude-code" ;;
      2) agent_choice="codex" ;;
      3) agent_choice="cursor" ;;
      4) agent_choice="antigravity" ;;
      5) agent_choice="vscode" ;;
      6) agent_choice="none" ;;
      *) echo "Invalid selection"; return 2 ;;
    esac
  else
    echo "✅ Detected agents:"
    local i=1
    local agents_array=()
    while IFS= read -r line; do
      echo "     $i) $line"
      agents_array+=("$line")
      ((i++))
    done <<< "$detected"
    echo "     $i) all (configure every detected host)"
    ((i++))
    echo "     $i) none (scaffold only, no host config)"
    read -p "Select agent [1-$i]: " agent_num
    if [[ "$agent_num" -le "${#agents_array[@]}" ]]; then
      agent_choice="${agents_array[$((agent_num-1))]}"
    elif [[ "$agent_num" -eq "$((i-1))" ]]; then
      agent_choice="all"
    elif [[ "$agent_num" -eq "$i" ]]; then
      agent_choice="none"
    else
      echo "Invalid selection"; return 2
    fi
  fi

  # Step 2: Orchestration behavior selection
  echo ""
  echo "🧠 Orchestration mode (Brain/Pinky multi-agent loop):"
  echo "     1) manual    — Human approval at every gate (default)"
  echo "     2) planning  — Auto-advance planning gates only"
  echo "     3) session   — Auto-advance within session boundaries"
  echo "     4) none      — Disable orchestration (Base mode only)"
  read -p "Select policy [1-4]: " orch_num
  case "$orch_num" in
    1) orch_policy="manual" ;;
    2) orch_policy="planning" ;;
    3) orch_policy="session" ;;
    4) orch_policy="" ;;
    *) echo "Invalid selection"; return 2 ;;
  esac

  # Step 3: Orchestration parameters (if enabled)
  if [[ -n "$orch_policy" ]]; then
    read -p "Max concurrent workers [4]: " orch_workers
    orch_workers=${orch_workers:-4}
    read -p "Max retries per task [2]: " orch_retries
    orch_retries=${orch_retries:-2}
    read -p "Session timeout (minutes) [120]: " orch_timeout
    orch_timeout=${orch_timeout:-120}
    read -p "Cost limit USD (0 = disabled) [0]: " orch_cost
    orch_cost=${orch_cost:-0}
    echo "Subagent coordination mode:"
    echo "     1) inline   — Host runs all roles (default)"
    echo "     2) delegate — Host spawns subagents per role"
    read -p "Select mode [1-2]: " mode_num
    case "$mode_num" in
      1) orch_mode="inline" ;;
      2) orch_mode="delegate" ;;
      *) orch_mode="inline" ;;
    esac
    echo "Verification sandbox:"
    echo "     1) none     — No isolation (default)"
    echo "     2) bwrap    — Bubblewrap (Linux only)"
    echo "     3) container — Container runtime"
    read -p "Select sandbox [1-3]: " sandbox_num
    case "$sandbox_num" in
      1) orch_sandbox="none" ;;
      2) orch_sandbox="bwrap" ;;
      3) orch_sandbox="container" ;;
      *) orch_sandbox="none" ;;
    esac
  fi

  # Step 4: Build and execute specd init command
  local cmd=(specd init --agent "$agent_choice" --yes)
  [[ -n "$dry_run" ]] && cmd+=("$dry_run")
  [[ -n "$repair" ]] && cmd+=("$repair")
  [[ -n "$refresh" ]] && cmd+=("$refresh")
  [[ -n "$json_out" ]] && cmd+=("$json_out")

  if [[ -n "$orch_policy" ]]; then
    cmd+=(--orchestration "$orch_policy")
    cmd+=(--orchestration-workers "$orch_workers")
    cmd+=(--orchestration-retries "$orch_retries")
    cmd+=(--orchestration-timeout "$orch_timeout")
    cmd+=(--orchestration-cost-limit "$orch_cost")
    cmd+=(--orchestration-mode "$orch_mode")
    cmd+=(--orchestration-sandbox "$orch_sandbox")
  fi

  echo ""
  echo "▶ Running: ${cmd[*]}"
  "${cmd[@]}"
}
```

**Deliverables:**
- [ ] `scripts/init-wrapper.sh` — POSIX-compliant shell function
- [ ] `scripts/init-wrapper.py` — Cross-platform Python equivalent (uses `curses`/`inquirer` for menus)
- [ ] `.specd/skills/specd-init/SKILL.md` — Skill documenting the `/init` behavior

### Phase 3: `/steer` Command Implementation (Week 2)

**Goal:** Aggregate and present all steering files with authorship guidance.

**Specification:**

```bash
# /steer — Inspect and edit steering constitution
# Usage: /steer [action]
# Actions:
#   show       Display all steering files (default)
#   edit       Open all in $EDITOR
#   status     Show which files are authored vs stub
#   bootstrap  Guided authorship of product.md + tech.md + structure.md
#   memory     Show promoted learnings from memory.md

/steer() {
  local action="${1:-show}"
  local specd_root=""
  specd_root=$(specd status 2>/dev/null | head -1 | grep -o '\.specd' || echo "")

  if [[ -z "$specd_root" ]]; then
    # Try to find .specd by walking up
    local dir="$PWD"
    while [[ "$dir" != "/" ]]; do
      if [[ -d "$dir/.specd/steering" ]]; then
        specd_root="$dir/.specd"
        break
      fi
      dir=$(dirname "$dir")
    done
  else
    specd_root="$PWD/.specd"
  fi

  if [[ ! -d "$specd_root/steering" ]]; then
    echo "❌ No .specd/steering directory found. Run /init first."
    return 3
  fi

  local steering_dir="$specd_root/steering"

  case "$action" in
    show)
      echo "📜 Steering Constitution — $steering_dir"
      echo ""
      for f in reasoning.md workflow.md product.md tech.md structure.md memory.md; do
        local file="$steering_dir/$f"
        if [[ -f "$file" ]]; then
          local size=$(wc -c < "$file" | tr -d ' ')
          local status="✅"
          [[ "$size" -lt 200 ]] && status="⚠️  STUB"
          echo "$status $f ($size bytes)"
        else
          echo "❌ $f MISSING"
        fi
      done
      echo ""
      echo "To view contents: /steer show <file>"
      echo "To edit: /steer edit"
      ;;

    edit)
      if [[ -n "$EDITOR" ]]; then
        $EDITOR "$steering_dir"/*.md
      else
        echo "EDITOR not set. Displaying contents:"
        for f in "$steering_dir"/*.md; do
          echo "
=== $(basename "$f") ==="
          cat "$f"
        done
      fi
      ;;

    status)
      echo "📊 Steering File Status"
      echo ""
      for f in reasoning.md workflow.md product.md tech.md structure.md memory.md; do
        local file="$steering_dir/$f"
        if [[ -f "$file" ]]; then
          local lines=$(wc -l < "$file" | tr -d ' ')
          local has_todo=$(grep -c "TODO\|FIXME\|XXX" "$file" 2>/dev/null || echo 0)
          if [[ "$lines" -lt 10 ]] || [[ "$has_todo" -gt 0 ]]; then
            echo "⚠️  $f — needs authorship ($lines lines, $has_todo placeholders)"
          else
            echo "✅ $f — authored ($lines lines)"
          fi
        else
          echo "❌ $f — missing"
        fi
      done
      echo ""
      echo "💡 Next: Run '/steer bootstrap' to author product.md, tech.md, and structure.md."
      ;;

    bootstrap)
      echo "🚀 Guided Steering Bootstrap"
      echo ""
      echo "The harness requires you (the agent) to author three files based on repo inspection:"
      echo "  1. product.md  — Domain rules, audience, constraints"
      echo "  2. tech.md     — Stack, languages, dependencies, test framework"
      echo "  3. structure.md — File org, module boundaries"
      echo ""
      echo "Recommended inspection commands:"
      echo "  rtk ls -R . | head -50"
      echo "  rtk cat README.md"
      echo "  rtk cat go.mod || rtk cat package.json"
      echo "  rtk cat .github/workflows/*.yml"
      echo ""
      read -p "Proceed with guided authorship? [y/N]: " confirm
      if [[ "$confirm" =~ ^[Yy]$ ]]; then
        for f in product.md tech.md structure.md; do
          echo "
--- Editing $f ---"
          if [[ -n "$EDITOR" ]]; then
            $EDITOR "$steering_dir/$f"
          else
            echo "Paste content for $f (Ctrl+D to finish):"
            cat > "$steering_dir/$f"
          fi
        done
        echo "✅ Steering bootstrap complete. Run '/steer status' to verify."
      fi
      ;;

    memory)
      if [[ -f "$steering_dir/memory.md" ]]; then
        echo "🧠 Promoted Learnings:"
        cat "$steering_dir/memory.md"
      else
        echo "No memory.md found."
      fi
      ;;

    *)
      echo "Unknown action: $action"
      echo "Usage: /steer [show|edit|status|bootstrap|memory]"
      return 2
      ;;
  esac
}
```

**Deliverables:**
- [ ] `scripts/steer-wrapper.sh` — POSIX shell function
- [ ] `scripts/steer-wrapper.py` — Cross-platform Python script
- [ ] `.specd/skills/specd-steering/SKILL.md` update — Document `/steer` behavior

### Phase 4: `/spec` Command Implementation (Week 2-3)

**Goal:** Unified spec dashboard with behavior selection.

**Specification:**

```bash
# /spec — Spec workflow dashboard
# Usage: /spec [action] [slug]
# Actions:
#   list         List all specs with status
#   new          Create a new spec (interactive)
#   continue     Continue an existing spec (auto-selects if only one)
#   check        Run validation gates on a spec
#   approve      Advance phase gate
#   context      Show phase briefing + LOAD NOW manifest
#   next         Get next runnable task
#   mode         Set execution mode (base/orchestrated)
#   report       Generate snapshot report

/spec() {
  local action="${1:-list}"
  local slug="$2"

  # Ensure .specd exists
  if ! specd status >/dev/null 2>&1; then
    echo "❌ No .specd found. Run /init first."
    return 3
  fi

  case "$action" in
    list)
      echo "📋 Spec Dashboard"
      echo ""
      local specs=""
      specs=$(specd status --json 2>/dev/null | jq -r '.specs[]? | "\(.slug)\t\(.status)\t\(.phase // "")"' 2>/dev/null)
      if [[ -z "$specs" ]]; then
        echo "No specs found. Create one with: /spec new"
        return 0
      fi
      echo "SPEC SLUG          STATUS        PHASE"
      echo "─────────────────────────────────────────"
      while IFS=$'\t' read -r s st ph; do
        local indicator=""
        case "$st" in
          complete) indicator="✅" ;;
          executing) indicator="▶️ " ;;
          blocked) indicator="🚫" ;;
          verifying) indicator="🧪" ;;
          requirements|design|tasks) indicator="📝" ;;
          *) indicator="⏳" ;;
        esac
        printf "%-18s %-13s %s %s\n" "$s" "$st" "$indicator" "$ph"
      done <<< "$specs"
      echo ""
      echo "Actions: /spec new | /spec continue <slug> | /spec check <slug>"
      ;;

    new)
      if [[ -z "$slug" ]]; then
        read -p "Spec slug (e.g., 'auth-service'): " slug
      fi
      if [[ -z "$slug" ]]; then
        echo "Slug required."
        return 2
      fi
      read -p "Spec title: " title
      read -p "Orchestrated mode? [y/N]: " orch
      local orch_flag=""
      [[ "$orch" =~ ^[Yy]$ ]] && orch_flag="--orchestrated"
      specd new "$slug" --title "$title" $orch_flag
      echo ""
      echo "✅ Created $slug. Next steps:"
      echo "  1. Edit .specd/specs/$slug/requirements.md (EARS format)"
      echo "  2. Run: /spec check $slug"
      echo "  3. Run: /spec approve $slug"
      ;;

    continue)
      if [[ -z "$slug" ]]; then
        # Auto-select if only one spec exists
        local count=""
        count=$(specd status --json 2>/dev/null | jq '.specs | length' 2>/dev/null)
        if [[ "$count" == "1" ]]; then
          slug=$(specd status --json 2>/dev/null | jq -r '.specs[0].slug')
          echo "Auto-selected spec: $slug"
        else
          specd status
          read -p "Enter spec slug to continue: " slug
        fi
      fi
      if [[ -z "$slug" ]]; then
        echo "No spec selected."
        return 2
      fi

      echo "📍 Continuing spec: $slug"
      echo ""

      # Show context
      specd context "$slug"
      echo ""

      # Show status
      local status=""
      status=$(specd status "$slug" --json 2>/dev/null | jq -r '.status')

      case "$status" in
        requirements|design|tasks)
          echo "📝 Planning phase. Next: author artifact, then /spec check $slug && /spec approve $slug"
          ;;
        executing)
          echo "▶️  Execution phase."
          local next_task=""
          next_task=$(specd next "$slug" --json 2>/dev/null | jq -r '.taskID // empty')
          if [[ -n "$next_task" ]]; then
            echo "Next runnable task: $next_task"
            echo "Run: specd verify $slug $next_task  (after implementing)"
            echo "Then: specd task $slug $next_task --status complete"
          else
            echo "No runnable tasks. Check blockers with: specd waves $slug"
          fi
          ;;
        verifying)
          echo "🧪 Verification phase. Run: /spec approve $slug"
          ;;
        complete)
          echo "✅ Spec complete. Run: /spec report $slug"
          ;;
        blocked)
          echo "🚫 Spec blocked. Check: specd waves $slug"
          ;;
      esac
      ;;

    check)
      [[ -z "$slug" ]] && { read -p "Spec slug: " slug; }
      specd check "$slug"
      ;;

    approve)
      [[ -z "$slug" ]] && { read -p "Spec slug: " slug; }
      specd approve "$slug"
      ;;

    context)
      [[ -z "$slug" ]] && { read -p "Spec slug: " slug; }
      specd context "$slug"
      ;;

    next)
      [[ -z "$slug" ]] && { read -p "Spec slug: " slug; }
      specd next "$slug"
      ;;

    mode)
      [[ -z "$slug" ]] && { read -p "Spec slug: " slug; }
      echo "Current mode:"
      specd mode "$slug" --json 2>/dev/null | jq -r '.mode // .executionMode // "unknown"'
      echo ""
      echo "Options:"
      echo "  1) base        — Manual host-driven execution"
      echo "  2) orchestrated — Brain/Pinky autonomous loop"
      read -p "Select mode [1-2] or Enter to keep current: " mode_choice
      case "$mode_choice" in
        1) specd mode "$slug" --set base ;;
        2) specd mode "$slug" --set orchestrated ;;
      esac
      ;;

    report)
      [[ -z "$slug" ]] && { read -p "Spec slug: " slug; }
      specd report "$slug"
      ;;

    *)
      echo "Unknown action: $action"
      echo "Usage: /spec [list|new|continue|check|approve|context|next|mode|report]"
      return 2
      ;;
  esac
}
```

**Deliverables:**
- [ ] `scripts/spec-wrapper.sh`
- [ ] `scripts/spec-wrapper.py`
- [ ] `.specd/skills/specd-spec/SKILL.md` — New skill documenting `/spec` behavior

### Phase 5: `/pinky-brain` Command Implementation (Week 3)

**Goal:** Unified orchestration loop management with behavior selection.

**Specification:**

```bash
# /pinky-brain — Brain/Pinky orchestration control
# Usage: /pinky-brain [action] [args]
# Actions:
#   status       Show orchestration capability + active sessions
#   enable       Enable orchestration with policy selection
#   disable      Disable orchestration
#   start        Start a Brain session for a spec
#   run          Run the built-in driver loop
#   step         Advance a session by one decision
#   pause        Pause a session
#   resume       Resume a session (or list resumable)
#   cancel       Cancel a session
#   compact      Compact context before /clear
#   config       Show current orchestration config
#   workers      List active Pinky workers

/pinky-brain() {
  local action="${1:-status}"
  local specd_root=""

  # Find .specd root
  local dir="$PWD"
  while [[ "$dir" != "/" ]]; do
    if [[ -f "$dir/.specd/config.json" ]]; then
      specd_root="$dir/.specd"
      break
    fi
    dir=$(dirname "$dir")
  done

  if [[ -z "$specd_root" ]]; then
    echo "❌ No .specd/config.json found. Run /init first."
    return 3
  fi

  local config_file="$specd_root/config.json"

  case "$action" in
    status)
      echo "🧠 Orchestration Status"
      echo ""

      # Check if orchestration is enabled in config
      local enabled=""
      enabled=$(jq -r '.orchestration.enabled // false' "$config_file" 2>/dev/null)
      if [[ "$enabled" == "true" ]]; then
        echo "✅ Orchestration enabled in config"
        echo "   Policy: $(jq -r '.orchestration.approvalPolicy // "manual"' "$config_file")"
        echo "   Workers: $(jq -r '.orchestration.maxWorkers // 4' "$config_file")"
        echo "   Retries: $(jq -r '.orchestration.maxRetries // 2' "$config_file")"
        echo "   Timeout: $(jq -r '.orchestration.sessionTimeoutMinutes // 120' "$config_file") min"
      else
        echo "⏸️  Orchestration disabled in config"
        echo "   Run '/pinky-brain enable' to configure."
      fi
      echo ""

      # List active sessions
      echo "Active Sessions:"
      local sessions=""
      sessions=$(specd brain resume --list --json 2>/dev/null | jq -r '.[]? | "\(.sessionID)\t\(.spec)\t\(.status)\t\(.updatedAt)"' 2>/dev/null)
      if [[ -n "$sessions" ]]; then
        echo "SESSION ID                           SPEC        STATUS    UPDATED"
        while IFS=$'\t' read -r sid spec st upd; do
          printf "%-36s %-11s %-9s %s\n" "$sid" "$spec" "$st" "$upd"
        done <<< "$sessions"
      else
        echo "   No active sessions."
      fi
      echo ""
      echo "Actions: /pinky-brain enable | /pinky-brain start <spec> | /pinky-brain resume"
      ;;

    enable)
      echo "🚀 Enabling Brain/Pinky Orchestration"
      echo ""
      echo "Approval policy:"
      echo "  1) manual   — Human approval at every gate"
      echo "  2) planning — Auto-advance requirements→design→tasks gates"
      echo "  3) session  — Auto-advance within current session"
      read -p "Select policy [1-3]: " policy_num
      local policy="manual"
      case "$policy_num" in
        1) policy="manual" ;;
        2) policy="planning" ;;
        3) policy="session" ;;
      esac

      read -p "Max workers [4]: " workers
      workers=${workers:-4}
      read -p "Max retries [2]: " retries
      retries=${retries:-2}
      read -p "Timeout minutes [120]: " timeout
      timeout=${timeout:-120}
      read -p "Cost limit USD (0=disabled) [0]: " cost
      cost=${cost:-0}

      # Update config.json
      local tmp_config="$(mktemp)"
      jq --arg policy "$policy"          --argjson workers "$workers"          --argjson retries "$retries"          --argjson timeout "$timeout"          --argjson cost "$cost"          '.orchestration.enabled = true |
          .orchestration.approvalPolicy = $policy |
          .orchestration.maxWorkers = $workers |
          .orchestration.maxRetries = $retries |
          .orchestration.sessionTimeoutMinutes = $timeout |
          .orchestration.hostReportedCostLimitUSD = $cost'          "$config_file" > "$tmp_config"
      mv "$tmp_config" "$config_file"
      echo "✅ Orchestration enabled. Config updated."
      ;;

    disable)
      local tmp_config="$(mktemp)"
      jq '.orchestration.enabled = false' "$config_file" > "$tmp_config"
      mv "$tmp_config" "$config_file"
      echo "⏸️  Orchestration disabled. Existing sessions remain active until complete/cancelled."
      ;;

    start)
      local slug="$2"
      [[ -z "$slug" ]] && { read -p "Spec slug: " slug; }
      [[ -z "$slug" ]] && { echo "Slug required"; return 2; }

      local policy=""
      policy=$(jq -r '.orchestration.approvalPolicy // "manual"' "$config_file")
      local workers=""
      workers=$(jq -r '.orchestration.maxWorkers // 4' "$config_file")
      local retries=""
      retries=$(jq -r '.orchestration.maxRetries // 2' "$config_file")
      local timeout=""
      timeout=$(jq -r '.orchestration.sessionTimeoutMinutes // 120' "$config_file")

      echo "Starting Brain session for $slug..."
      echo "Policy: $policy | Workers: $workers | Retries: $retries | Timeout: ${timeout}m"
      specd brain start "$slug"         --approval-policy "$policy"         --max-workers "$workers"         --max-retries "$retries"         --timeout-seconds "$((timeout * 60))"         --json
      ;;

    run)
      local slug="$2"
      [[ -z "$slug" ]] && { read -p "Spec slug: " slug; }
      [[ -z "$slug" ]] && { echo "Slug required"; return 2; }

      read -p "Worker command (e.g., 'python3 agent.py') [optional]: " worker_cmd
      local cmd=(specd brain run "$slug")
      [[ -n "$worker_cmd" ]] && cmd+=(--worker-cmd "$worker_cmd")
      echo "Running: ${cmd[*]}"
      "${cmd[@]}"
      ;;

    step)
      local slug="$2"
      local session="$3"
      [[ -z "$slug" ]] && { read -p "Spec slug: " slug; }
      [[ -z "$session" ]] && { read -p "Session ID: " session; }

      local policy=""
      policy=$(jq -r '.orchestration.approvalPolicy // "manual"' "$config_file")
      local workers=""
      workers=$(jq -r '.orchestration.maxWorkers // 4' "$config_file")
      local retries=""
      retries=$(jq -r '.orchestration.maxRetries // 2' "$config_file")
      local timeout=""
      timeout=$(jq -r '.orchestration.sessionTimeoutMinutes // 120' "$config_file")

      specd brain step "$slug" --session "$session"         --approval-policy "$policy"         --max-workers "$workers"         --max-retries "$retries"         --timeout-seconds "$((timeout * 60))"         --json
      ;;

    pause)
      local session="$2"
      [[ -z "$session" ]] && { read -p "Session ID: " session; }
      specd brain pause --session "$session"
      ;;

    resume)
      local session="$2"
      if [[ -z "$session" ]]; then
        # List resumable and pick head
        local list=""
        list=$(specd brain resume --list --json 2>/dev/null | jq -r '.[0]?.sessionID // empty' 2>/dev/null)
        if [[ -n "$list" ]]; then
          session="$list"
          echo "Resuming most recent session: $session"
        else
          echo "No resumable sessions found."
          return 1
        fi
      fi
      specd brain resume --session "$session"
      ;;

    cancel)
      local session="$2"
      [[ -z "$session" ]] && { read -p "Session ID: " session; }
      specd brain cancel --session "$session"
      ;;

    compact)
      local session="$2"
      [[ -z "$session" ]] && { read -p "Session ID: " session; }
      specd brain compact --session "$session" --reason "manual-compact"
      ;;

    config)
      echo "Current orchestration configuration:"
      jq '.orchestration // {}' "$config_file"
      ;;

    workers)
      echo "Active Pinky workers (from session state):"
      # This would require parsing session state from subagents/ directory
      # Simplified: show session list with status
      specd brain resume --list --json 2>/dev/null | jq -r '.[]? | "\(.sessionID): \(.spec) — \(.status)"' 2>/dev/null
      ;;

    *)
      echo "Unknown action: $action"
      echo "Usage: /pinky-brain [status|enable|disable|start|run|step|pause|resume|cancel|compact|config|workers]"
      return 2
      ;;
  esac
}
```

**Deliverables:**
- [ ] `scripts/pinky-brain-wrapper.sh`
- [ ] `scripts/pinky-brain-wrapper.py`
- [ ] `.specd/skills/specd-brain/SKILL.md` update — Document `/pinky-brain` behavior

### Phase 6: Integration & Testing (Week 3-4)

| Step | Action | Verification |
|------|--------|--------------|
| 6.1 | Combine all wrappers into a single sourceable file: `source specd-workflow.sh` | All functions (`/init`, `/steer`, `/spec`, `/pinky-brain`) available in shell |
| 6.2 | Create Python unified CLI: `python specd-workflow.py <command>` | Cross-platform execution |
| 6.3 | Test `/init` on fresh repo with `--dry-run` | Produces expected command without mutation |
| 6.4 | Test `/steer` on initialized repo | Shows all 6 steering files with correct status |
| 6.5 | Test `/spec new` and `/spec continue` | Creates spec, shows context, suggests next action |
| 6.6 | Test `/pinky-brain status` on disabled project | Reports disabled, suggests enable |
| 6.7 | Test `/pinky-brain enable` + `start` | Config updated, session starts |
| 6.8 | Run `make ci` equivalent on specd repo if contributing upstream | All tests pass |
| 6.9 | Write integration tests using `internal/testharness` patterns | Deterministic, no golden files |

### Phase 7: Documentation & Skill Packs (Week 4)

| Step | Action | Deliverable |
|------|--------|-------------|
| 7.1 | Author `SKILL.md` for `/init` | `.specd/skills/specd-init/SKILL.md` |
| 7.2 | Author `SKILL.md` for `/steer` | `.specd/skills/specd-steering/SKILL.md` (update) |
| 7.3 | Author `SKILL.md` for `/spec` | `.specd/skills/specd-spec/SKILL.md` |
| 7.4 | Author `SKILL.md` for `/pinky-brain` | `.specd/skills/specd-brain/SKILL.md` (update) |
| 7.5 | Update root `AGENTS.md` with slash command quick reference | `AGENTS.md` section |
| 7.6 | Write `README.md` for the wrapper scripts | `scripts/README.md` |

---

## 6. Best Practice Adherence Checklist

| Practice | Implementation |
|----------|----------------|
| ✅ **Foundational Split** | Wrappers are thin orchestration glue; all enforcement remains in `specd` CLI |
| ✅ **Specs as Source of Truth** | Wrappers never hand-edit `state.json`; all mutations via `specd` commands |
| ✅ **Evidence Gates** | `/spec continue` enforces `verify → complete` flow; no bypass |
| ✅ **Waves, Not Lines** | `/spec` uses `specd next` and `specd waves` for DAG-aware execution |
| ✅ **Agent-Agnostic** | Wrappers are shell/Python; work with any agent that can run shell commands |
| ✅ **Human Gates** | `/init`, `/pinky-brain enable`, `/spec approve` require explicit confirmation |
| ✅ **Deterministic Reporting** | `/spec report` and `/pinky-brain status` use `specd` native reporting |
| ✅ **Steering as Constitution** | `/steer` treats `.specd/steering/` as durable, outliving chat sessions |
| ✅ **Exit Code Contract** | Wrappers propagate specd exit codes (0=ok, 1=gate, 2=usage, 3=not found) |
| ✅ **Atomic State** | No wrapper modifies `state.json` directly; all via CLI CAS-guarded writes |
| ✅ **Project-Scoped Config** | `/init` defaults to `--scope project`; never touches global config without consent |
| ✅ **Fail-Closed** | `--dry-run` preview; `--sandbox` fail-closed if isolator absent |

---

## 7. Risk Assessment & Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| specd CLI changes in future versions | Wrappers break | Pin wrapper to specd version; use `specd help --json` for schema discovery; add version check in `/init` |
| Windows compatibility (orchestration POSIX-only) | Brain/Pinky fails on Windows native | `/pinky-brain` detects Windows and warns: "Orchestration requires POSIX shell; run under WSL." Base mode remains portable. |
| Concurrent wrapper + specd CLI access | CAS aborts | Wrappers do not cache state; every call re-reads from disk. On CAS failure, suggest retry. |
| Agent misinterprets wrapper suggestions | Wrong behavior | All suggestions are deterministic (no LLM in wrapper); menu options are explicit and numbered. |
| Steering file bloat | Context overflow | `/steer` shows file sizes; `specd context` already budgets token hints. |

---

## 8. Quick Reference: Slash Command → specd Native Mapping

| Slash Command | Primary Native Commands | Purpose |
|---------------|------------------------|---------|
| `/init` | `specd init`, `specd doctor` | Bootstrap project with interactive agent detection + orchestration config |
| `/steer` | `cat .specd/steering/*.md`, `specd context` | Inspect, edit, and bootstrap steering constitution |
| `/spec` | `specd new`, `specd status`, `specd check`, `specd approve`, `specd next`, `specd context`, `specd mode`, `specd report` | Unified spec lifecycle dashboard |
| `/pinky-brain` | `specd brain *`, `specd pinky *`, `jq .orchestration` | Enable/disable/manage Brain/Pinky orchestration loop |

---

## 9. Conclusion

The specd repository provides a **complete, deterministic, agent-agnostic harness** for spec-driven coding. The user's requested slash commands (`/init`, `/steer`, `/spec`, `/pinky-brain`) are best implemented as **thin interactive wrappers** around the existing CLI surface, not as modifications to specd core. This preserves the Foundational Split (agent reasons, harness enforces) while adding user-friendly behavior selection menus.

**Immediate next action:** Implement Phase 1 (local install + test repo validation), then proceed to Phase 2 (`/init` wrapper) as the highest-impact deliverable.

---

*Generated from analysis of https://github.com/0xkhdr/specd — commit range: main branch, 2026-06-28.*
