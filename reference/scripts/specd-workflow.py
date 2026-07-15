#!/usr/bin/env python3
"""Optional UX wrappers for specd slash workflows.

Thin glue only: project/spec mutations delegate to native `specd` unless a task
explicitly needs safe canonical steering/config file writes.
"""
from __future__ import annotations

import argparse
import json
import os
import platform
import re
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Any, Iterable

EXIT_OK = 0
EXIT_GATE = 1
EXIT_USAGE = 2
EXIT_NOT_FOUND = 3

KNOWN_HOSTS = ["claude-code", "codex", "cursor", "antigravity", "vscode", "none"]
CANONICAL_STEERING = ["reasoning.md", "workflow.md", "product.md", "tech.md", "structure.md", "memory.md"]
BOOTSTRAP_STEERING = ["product.md", "tech.md", "structure.md"]
ORCH_POLICIES = ["none", "manual", "planning", "session"]
ROLE_MODES = ["inline", "delegate"]
SANDBOXES = ["none", "bwrap", "container"]
DIRECT_SPEC_ACTIONS = {"check", "approve", "context", "next", "waves", "report"}
SESSION_ACTIONS = {"start", "run", "step", "pause", "resume", "cancel", "compact"}


def eprint(msg: str) -> None:
    print(msg, file=sys.stderr)


def run_native(argv: list[str]) -> int:
    try:
        return subprocess.run(["specd", *argv]).returncode
    except FileNotFoundError:
        eprint("specd not found on PATH")
        return EXIT_NOT_FOUND


def capture_native(argv: list[str]) -> tuple[int, str, str]:
    try:
        p = subprocess.run(["specd", *argv], text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        return p.returncode, p.stdout, p.stderr
    except FileNotFoundError:
        return EXIT_NOT_FOUND, "", "specd not found on PATH\n"


def find_specd_root(start: Path | None = None) -> Path | None:
    cur = (start or Path.cwd()).resolve()
    for p in [cur, *cur.parents]:
        if (p / ".specd").is_dir():
            return p
    return None


def require_specd_root() -> Path | None:
    root = find_specd_root()
    if root is None:
        eprint(".specd not found. Run /init or `specd init` first.")
    return root


def steering_root() -> Path | None:
    root = require_specd_root()
    if root is None:
        return None
    d = (root / ".specd" / "steering").resolve()
    if not d.is_dir():
        eprint(".specd/steering not found. Run /init or `specd init --repair` first.")
        return None
    return d


def json_loads_maybe(s: str) -> Any | None:
    try:
        return json.loads(s)
    except Exception:
        return None


def iter_values(obj: Any) -> Iterable[Any]:
    if isinstance(obj, dict):
        yield obj
        for v in obj.values():
            yield from iter_values(v)
    elif isinstance(obj, list):
        for v in obj:
            yield from iter_values(v)
    else:
        yield obj


def probe_hosts() -> list[str]:
    for argv in (["doctor", "--json"], ["init", "--dry-run", "--json"]):
        code, out, _ = capture_native(argv)
        if code in (0, 1):
            data = json_loads_maybe(out)
            found: list[str] = []
            if data is not None:
                for item in iter_values(data):
                    if isinstance(item, dict):
                        name = item.get("name") or item.get("host") or item.get("agent")
                        detected = item.get("detected", True)
                        if isinstance(name, str) and name in KNOWN_HOSTS and detected is not False:
                            found.append(name)
                if found:
                    return sorted(set(found), key=KNOWN_HOSTS.index)
    return KNOWN_HOSTS.copy()


def native_has_command(command: str) -> bool:
    code, out, _ = capture_native(["help", "--json"])
    if code == 0:
        data = json_loads_maybe(out)
        for item in iter_values(data):
            if isinstance(item, dict) and item.get("command") == command:
                return True
            if isinstance(item, str) and item == command:
                return True
    code, out, err = capture_native(["help"])
    text = f"{out}\n{err}"
    return re.search(rf"(^|\s){re.escape(command)}(\s|$)", text) is not None


def prompt_choice(label: str, choices: list[str], default: str) -> tuple[int, str]:
    while True:
        val = input(f"{label} [{default}] ({'/'.join(choices)}): ").strip() or default
        if val in choices:
            return EXIT_OK, val
        print(f"invalid choice: {val}")


def prompt_int(label: str, default: int, min_value: int = 0) -> tuple[int, int]:
    while True:
        raw = input(f"{label} [{default}]: ").strip()
        if raw == "":
            return EXIT_OK, default
        try:
            val = int(raw)
        except ValueError:
            print(f"invalid integer: {raw}")
            continue
        if val >= min_value:
            return EXIT_OK, val
        print(f"must be >= {min_value}")


def atomic_write(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, tmp = tempfile.mkstemp(prefix=f".{path.name}.", suffix=".tmp", dir=str(path.parent))
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as f:
            f.write(text)
            f.flush()
            os.fsync(f.fileno())
        os.replace(tmp, path)
    finally:
        try:
            os.unlink(tmp)
        except FileNotFoundError:
            pass


def add_init_parser(sub: argparse._SubParsersAction[argparse.ArgumentParser]) -> None:
    p = sub.add_parser("init", help="interactive/non-interactive specd init wrapper")
    p.set_defaults(func=cmd_init)
    p.add_argument("--agent", choices=["auto", "all", *KNOWN_HOSTS], default=None)
    p.add_argument("--dry-run", action="store_true")
    p.add_argument("--repair", action="store_true")
    p.add_argument("--refresh", action="store_true")
    p.add_argument("--json", action="store_true")
    p.add_argument("--orchestration", choices=ORCH_POLICIES, default=None)
    p.add_argument("--workers", "--orchestration-workers", dest="workers", type=int, default=4)
    p.add_argument("--retries", "--orchestration-retries", dest="retries", type=int, default=2)
    p.add_argument("--timeout", "--orchestration-timeout", dest="timeout", type=int, default=120)
    p.add_argument("--cost", "--orchestration-cost-limit", dest="cost", default=None)
    p.add_argument("--role-mode", "--orchestration-mode", dest="role_mode", choices=ROLE_MODES, default="inline")
    p.add_argument("--sandbox", "--orchestration-sandbox", dest="sandbox", choices=SANDBOXES, default="none")
    p.add_argument("--yes", action="store_true")
    p.add_argument("--non-interactive", action="store_true")
    p.add_argument("--probe-hosts", action="store_true", help=argparse.SUPPRESS)


def build_init_argv(args: argparse.Namespace, agent: str, policy: str) -> list[str]:
    argv = ["init", "--agent", agent]
    for flag in ("dry_run", "repair", "refresh", "json", "non_interactive", "yes"):
        if getattr(args, flag):
            argv.append("--" + flag.replace("_", "-"))
    if policy != "none":
        argv += ["--orchestration", policy,
                 "--orchestration-workers", str(args.workers),
                 "--orchestration-retries", str(args.retries),
                 "--orchestration-timeout", str(args.timeout),
                 "--orchestration-mode", args.role_mode,
                 "--orchestration-sandbox", args.sandbox]
        if args.cost is not None:
            argv += ["--orchestration-cost-limit", str(args.cost)]
    return argv


def cmd_init(args: argparse.Namespace) -> int:
    if args.repair and args.refresh:
        eprint("--repair and --refresh are mutually exclusive")
        return EXIT_USAGE
    if args.probe_hosts:
        print("\n".join(probe_hosts()))
        return EXIT_OK
    interactive = (not args.non_interactive) and sys.stdin.isatty()
    if interactive:
        hosts = probe_hosts()
        print("Detected/known hosts: " + ", ".join(hosts + ["all", "auto"]))
        agent = args.agent
        if agent is None:
            code, agent = prompt_choice("Agent", ["auto", "all", *KNOWN_HOSTS], "auto")
            if code != 0:
                return code
        policy = args.orchestration
        if policy is None:
            code, policy = prompt_choice("Orchestration", ORCH_POLICIES, "none")
            if code != 0:
                return code
        if policy != "none":
            _, args.workers = prompt_int("Workers", args.workers, 1)
            _, args.retries = prompt_int("Retries", args.retries, 0)
            _, args.timeout = prompt_int("Timeout minutes", args.timeout, 1)
            code, args.role_mode = prompt_choice("Role mode", ROLE_MODES, args.role_mode)
            if code != 0:
                return code
            code, args.sandbox = prompt_choice("Sandbox", SANDBOXES, args.sandbox)
            if code != 0:
                return code
            raw_cost = input("Cost limit USD [disabled]: ").strip()
            if raw_cost:
                args.cost = raw_cost
    else:
        agent = args.agent or "auto"
        policy = args.orchestration or "none"
    return run_native(build_init_argv(args, agent, policy))


def add_steer_parser(sub: argparse._SubParsersAction[argparse.ArgumentParser]) -> None:
    p = sub.add_parser("steer", help="inspect/edit steering files")
    sp = p.add_subparsers(dest="action")
    p.set_defaults(func=cmd_steer)
    show = sp.add_parser("show")
    show.add_argument("file", nargs="?", default=None)
    sp.add_parser("status")
    sp.add_parser("memory")
    edit = sp.add_parser("edit")
    edit.add_argument("file", nargs="?", default="all")
    boot = sp.add_parser("bootstrap")
    boot.add_argument("file", nargs="?", choices=BOOTSTRAP_STEERING + ["all"], default=None)
    boot.add_argument("--dry-run", action="store_true")
    boot.add_argument("--stdin", action="store_true", help="write stdin to selected bootstrap file")


def valid_steering_name(name: str) -> bool:
    return name in CANONICAL_STEERING or name == "all"


def classify_steering(path: Path) -> str:
    if not path.exists():
        return "missing"
    text = path.read_text(encoding="utf-8", errors="replace")
    low = text.lower()
    if len(text.strip()) < 20:
        return "stub"
    if re.search(r"\b(todo|tbd|placeholder|fill this|replace me)\b", low):
        return "placeholder"
    if "<!-- specd" in low or "speccd init" in low:
        return "stub"
    return "authored"


def edit_steering(root: Path, name: str) -> int:
    if not valid_steering_name(name):
        eprint("invalid steering file; use basename from canonical set")
        return EXIT_USAGE
    editor = os.environ.get("EDITOR")
    if not editor:
        eprint("EDITOR not set. Set EDITOR or use `steer bootstrap <file> --stdin`.")
        return EXIT_USAGE
    names = CANONICAL_STEERING if name == "all" else [name]
    paths = [str(root / n) for n in names]
    try:
        return subprocess.run([editor, *paths]).returncode
    except FileNotFoundError:
        eprint(f"editor not found: {editor}")
        return EXIT_NOT_FOUND


def bootstrap_guidance() -> None:
    print("Bootstrap steering checklist:")
    print("  product.md: users, problem, core workflows, non-goals")
    print("  tech.md: language/runtime, build/test/lint commands, dependencies, constraints")
    print("  structure.md: repo layout, ownership, generated files, naming rules")
    print("Inspect manifests, README, CI, tests, and source tree before writing.")


def bootstrap_steering(root: Path, args: argparse.Namespace) -> int:
    bootstrap_guidance()
    target = args.file or "all"
    names = BOOTSTRAP_STEERING if target == "all" else [target]
    if args.dry_run:
        print("dry-run: would update " + ", ".join(names))
        return EXIT_OK
    if args.stdin:
        if target == "all":
            eprint("--stdin requires one of product.md, tech.md, structure.md")
            return EXIT_USAGE
        atomic_write(root / target, sys.stdin.read())
        print(f"wrote {target}")
        return EXIT_OK
    return edit_steering(root, target)


def cmd_steer(args: argparse.Namespace) -> int:
    action = args.action or "show"
    root = steering_root()
    if root is None:
        return EXIT_NOT_FOUND
    if action == "status":
        for name in CANONICAL_STEERING:
            path = root / name
            size = path.stat().st_size if path.exists() else 0
            print(f"{name}\t{classify_steering(path)}\t{size} bytes")
        return EXIT_OK
    if action == "edit":
        return edit_steering(root, args.file)
    if action == "memory":
        path = root / "memory.md"
        print("--- memory.md ---")
        if not path.exists():
            print("warning: memory.md missing; no file created")
            return EXIT_OK
        text = path.read_text(encoding="utf-8", errors="replace")
        print(text, end="")
        if not text.endswith("\n"):
            print()
        return EXIT_OK
    if action == "bootstrap":
        return bootstrap_steering(root, args)
    if action == "show":
        name = args.file
        if name is None:
            for n in CANONICAL_STEERING:
                path = root / n
                state = classify_steering(path)
                size = path.stat().st_size if path.exists() else 0
                print(f"{n}\t{state}\t{size} bytes")
            return EXIT_OK
        if not valid_steering_name(name):
            eprint("invalid steering file; use basename from canonical set")
            return EXIT_USAGE
        names = CANONICAL_STEERING if name == "all" else [name]
        for n in names:
            path = root / n
            print(f"--- {n} ---")
            if not path.exists():
                print("(missing)")
                continue
            text = path.read_text(encoding="utf-8", errors="replace")
            print(text, end="")
            if not text.endswith("\n"):
                print()
        return EXIT_OK
    eprint("usage: specd-workflow steer [show [file]|status|memory|edit [file]|bootstrap [file] [--dry-run|--stdin]]")
    return EXIT_USAGE


def add_spec_parser(sub: argparse._SubParsersAction[argparse.ArgumentParser]) -> None:
    p = sub.add_parser("spec", help="spec workflow dashboard")
    sp = p.add_subparsers(dest="action")
    p.set_defaults(func=cmd_spec)
    sp.add_parser("list")
    new = sp.add_parser("new")
    new.add_argument("slug")
    new.add_argument("--title")
    new.add_argument("--orchestrated", action="store_true")
    cont = sp.add_parser("continue")
    cont.add_argument("slug", nargs="?")
    mode = sp.add_parser("mode")
    mode.add_argument("slug", nargs="?")
    mode.add_argument("extra", nargs=argparse.REMAINDER)
    for action in sorted(DIRECT_SPEC_ACTIONS):
        q = sp.add_parser(action)
        q.add_argument("slug", nargs="?")
        q.add_argument("extra", nargs=argparse.REMAINDER)


def parse_specs_from_status_json(out: str) -> list[dict[str, Any]] | None:
    data = json_loads_maybe(out)
    if data is None:
        return None
    rows = data if isinstance(data, list) else data.get("specs") if isinstance(data, dict) else None
    if not isinstance(rows, list):
        return None
    return [r for r in rows if isinstance(r, dict) and isinstance(r.get("spec"), str)]


def list_specs() -> tuple[int, list[dict[str, Any]], str]:
    code, out, err = capture_native(["status", "--json"])
    if code != 0:
        return code, [], err or out
    rows = parse_specs_from_status_json(out)
    if rows is not None:
        return EXIT_OK, rows, ""
    code, out, err = capture_native(["status"])
    if code != 0:
        return code, [], err or out
    rows = []
    for line in out.splitlines():
        m = re.match(r"^([a-z0-9][a-z0-9-]*)\s+\[([^]]+)\]", line)
        if m:
            rows.append({"spec": m.group(1), "status": m.group(2)})
    return EXIT_OK, rows, ""


def select_slug(provided: str | None) -> tuple[int, str | None]:
    if provided:
        return EXIT_OK, provided
    code, rows, msg = list_specs()
    if code != 0:
        if msg:
            eprint(msg.strip())
        return code, None
    if len(rows) == 0:
        print("No specs yet. Run `/spec new <slug>` or `specd new <slug>`.")
        return EXIT_NOT_FOUND, None
    if len(rows) == 1:
        return EXIT_OK, rows[0]["spec"]
    print("Multiple specs found; choose one explicitly:")
    for r in rows:
        print(f"  {r['spec']}")
    return EXIT_USAGE, None


def status_for_slug(slug: str) -> dict[str, Any]:
    code, rows, _ = list_specs()
    if code != 0:
        return {}
    for r in rows:
        if r.get("spec") == slug:
            return r
    return {}


def cmd_spec(args: argparse.Namespace) -> int:
    action = args.action or "list"
    if action == "list":
        code, rows, msg = list_specs()
        if code != 0:
            if msg:
                eprint(msg.strip())
            return code
        if not rows:
            print("No specs yet. Run `/spec new <slug>` or `specd new <slug>`.")
            return EXIT_OK
        for r in rows:
            spec = r.get("spec")
            status = r.get("status", "unknown")
            phase = r.get("phase", "")
            done = r.get("complete", "?")
            total = r.get("total", "?")
            print(f"{spec}\t{status}\t{phase}\t{done}/{total} done")
        return EXIT_OK
    if action == "new":
        argv = ["new", args.slug]
        if args.title:
            argv += ["--title", args.title]
        if args.orchestrated:
            argv.append("--orchestrated")
        rc = run_native(argv)
        if rc == 0:
            print(f"Next: write `.specd/specs/{args.slug}/requirements.md`, then run `specd check {args.slug}`.")
        return rc
    if action == "continue":
        code, slug = select_slug(args.slug)
        if code != 0:
            return code
        rc = run_native(["context", slug])
        if rc != 0:
            return rc
        row = status_for_slug(slug)
        status = str(row.get("status") or row.get("phase") or "").lower()
        if "execut" in status:
            return run_native(["next", slug])
        if "verify" in status:
            print(f"Next: run `specd approve {slug}` after spec-level verification evidence is reviewed.")
        elif "complete" in status:
            print(f"Spec `{slug}` complete. Run `specd report {slug}` for snapshot.")
        else:
            print(f"Next: run `specd check {slug}`; if gate passes, ask human to run `specd approve {slug}`.")
        return EXIT_OK
    if action == "mode":
        if not native_has_command("mode"):
            eprint("native specd mode command unsupported; fallback: use base workflow or upgrade specd")
            return EXIT_GATE
        code, slug = select_slug(args.slug)
        if code != 0:
            return code
        extra = getattr(args, "extra", []) or []
        return run_native(["mode", slug, *extra])
    if action in DIRECT_SPEC_ACTIONS:
        code, slug = select_slug(args.slug)
        if code != 0:
            return code
        extra = getattr(args, "extra", []) or []
        return run_native([action, slug, *extra])
    eprint("usage: specd-workflow spec [list|new|continue|check|approve|context|next|waves|report|mode]")
    return EXIT_USAGE


def add_pinky_parser(sub: argparse._SubParsersAction[argparse.ArgumentParser]) -> None:
    p = sub.add_parser("pinky-brain", help="Brain/Pinky orchestration console")
    sp = p.add_subparsers(dest="action")
    p.set_defaults(func=cmd_pinky)
    sp.add_parser("status")
    en = sp.add_parser("enable")
    add_orch_flags(en)
    sp.add_parser("disable")
    for action in sorted(SESSION_ACTIONS):
        q = sp.add_parser(action)
        q.add_argument("spec", nargs="?")
        q.add_argument("--session")
        q.add_argument("--worker-cmd")
        q.add_argument("--policy", "--approval-policy", dest="policy", choices=ORCH_POLICIES[1:], default="manual")
        q.add_argument("--workers", type=int, default=4)
        q.add_argument("--retries", type=int, default=2)
        q.add_argument("--timeout", type=int, default=120)
        q.add_argument("--cost")
        q.add_argument("--json", action="store_true")
    sp.add_parser("workers")


def add_orch_flags(p: argparse.ArgumentParser) -> None:
    p.add_argument("--policy", "--orchestration", dest="policy", choices=ORCH_POLICIES[1:], default="manual")
    p.add_argument("--workers", type=int, default=4)
    p.add_argument("--retries", type=int, default=2)
    p.add_argument("--timeout", type=int, default=120)
    p.add_argument("--cost")
    p.add_argument("--role-mode", choices=ROLE_MODES, default="inline")
    p.add_argument("--sandbox", choices=SANDBOXES, default="none")
    p.add_argument("--yes", action="store_true")
    p.add_argument("--non-interactive", action="store_true")
    p.add_argument("--dry-run", action="store_true")


def read_config_status(root: Path) -> str:
    cfg_json = root / ".specd" / "config.json"
    cfg_yml = root / ".specd" / "config.yml"
    if cfg_json.exists():
        data = json_loads_maybe(cfg_json.read_text(encoding="utf-8", errors="replace"))
        if isinstance(data, dict):
            orch = data.get("orchestration")
            if isinstance(orch, dict):
                return "enabled" if orch.get("enabled") is True else "disabled"
            return "disabled"
        return "unknown (invalid config.json)"
    if cfg_yml.exists():
        text = cfg_yml.read_text(encoding="utf-8", errors="replace")
        m = re.search(r"(?ms)^orchestration:\s*(?:\n\s+[^\n]*)*?\n\s+enabled:\s*(true|false)\b", text + "\n")
        if m:
            return "enabled" if m.group(1) == "true" else "disabled"
        if re.search(r"(?m)^orchestration:\s*$", text):
            return "unknown"
        return "disabled"
    return "unknown (missing config)"


def print_sessions() -> None:
    code, out, err = capture_native(["brain", "resume", "--list", "--json"])
    if code != 0:
        print("sessions: unknown (native list failed)")
        if err.strip():
            print("warning: " + err.strip())
        return
    data = json_loads_maybe(out)
    rows = data if isinstance(data, list) else data.get("sessions") if isinstance(data, dict) else None
    if not isinstance(rows, list) or not rows:
        print("sessions: none")
        return
    print("sessions:")
    for s in rows:
        if isinstance(s, dict):
            sid = s.get("id") or s.get("session") or "?"
            spec = s.get("spec") or s.get("slug") or "?"
            status = s.get("status") or "?"
            updated = s.get("updated") or s.get("updatedAt") or "?"
            print(f"  {sid}\t{spec}\t{status}\t{updated}")


def disable_yaml_orchestration(text: str) -> str:
    lines = text.splitlines(keepends=True)
    out: list[str] = []
    i = 0
    changed = False
    found = False
    while i < len(lines):
        line = lines[i]
        if re.match(r"^orchestration:\s*(?:#.*)?\n?$", line):
            found = True
            out.append(line)
            i += 1
            block: list[str] = []
            while i < len(lines) and (lines[i].startswith(" ") or lines[i].startswith("\t") or lines[i].strip() == ""):
                block.append(lines[i])
                i += 1
            for j, b in enumerate(block):
                if re.match(r"^\s+enabled:\s*(true|false)\b", b):
                    indent = re.match(r"^(\s*)", b).group(1)  # type: ignore[union-attr]
                    nl = "\n" if b.endswith("\n") else ""
                    block[j] = f"{indent}enabled: false{nl}"
                    changed = True
                    break
            if not changed:
                block.insert(0, "  enabled: false\n")
                changed = True
            out.extend(block)
            continue
        out.append(line)
        i += 1
    if not found:
        sep = "" if text.endswith("\n") or text == "" else "\n"
        return text + sep + "\norchestration:\n  enabled: false\n"
    return "".join(out)


def disable_orchestration(root: Path) -> int:
    cfg_json = root / ".specd" / "config.json"
    cfg_yml = root / ".specd" / "config.yml"
    if cfg_json.exists():
        data = json_loads_maybe(cfg_json.read_text(encoding="utf-8", errors="replace"))
        if not isinstance(data, dict):
            eprint("invalid config.json; refusing to edit")
            return EXIT_GATE
        orch = data.get("orchestration")
        if not isinstance(orch, dict):
            orch = {}
            data["orchestration"] = orch
        orch["enabled"] = False
        atomic_write(cfg_json, json.dumps(data, indent=2, sort_keys=True) + "\n")
        print("orchestration disabled for future sessions; active session files untouched")
        return EXIT_OK
    if cfg_yml.exists():
        text = cfg_yml.read_text(encoding="utf-8", errors="replace")
        atomic_write(cfg_yml, disable_yaml_orchestration(text))
        print("orchestration disabled for future sessions; active session files untouched")
        return EXIT_OK
    eprint("no config file found; run `specd init` first")
    return EXIT_NOT_FOUND


def print_worker_view(root: Path) -> int:
    sessions_dir = root / ".specd" / "runtime" / "sessions"
    if not sessions_dir.is_dir():
        print("workers: none")
        return EXIT_OK
    found = False
    for session_dir in sorted(p for p in sessions_dir.iterdir() if p.is_dir()):
        workers_dir = session_dir / "workers"
        if not workers_dir.is_dir():
            continue
        for worker_dir in sorted(p for p in workers_dir.iterdir() if p.is_dir()):
            found = True
            lease = worker_dir / "lease.json"
            cursor = worker_dir / "cursor.json"
            status = "unknown"
            task = "?"
            if lease.exists():
                data = json_loads_maybe(lease.read_text(encoding="utf-8", errors="replace"))
                if isinstance(data, dict):
                    status = str(data.get("status") or data.get("state") or status)
                    task = str(data.get("task") or data.get("taskID") or task)
            if cursor.exists():
                data = json_loads_maybe(cursor.read_text(encoding="utf-8", errors="replace"))
                if isinstance(data, dict):
                    status = str(data.get("status") or data.get("state") or status)
            print(f"{session_dir.name}\t{worker_dir.name}\t{status}\t{task}")
    if not found:
        print("workers: none")
    return EXIT_OK


def native_windows_without_wsl() -> bool:
    system = platform.system()
    if os.environ.get("SPECD_WORKFLOW_TESTING") == "1":
        system = os.environ.get("SPECD_WORKFLOW_PLATFORM", system)
    return system.lower().startswith("win") and "WSL_DISTRO_NAME" not in os.environ


def cmd_pinky(args: argparse.Namespace) -> int:
    action = args.action or "status"
    root = require_specd_root()
    if root is None:
        return EXIT_NOT_FOUND
    brain = native_has_command("brain")
    pinky = native_has_command("pinky")
    if action == "status":
        print(f"brain: {'available' if brain else 'unsupported'}")
        print(f"pinky: {'available' if pinky else 'unsupported'}")
        print(f"orchestration: {read_config_status(root)}")
        if brain:
            print_sessions()
        else:
            print("Enable/upgrade native specd with Brain/Pinky support for orchestration actions.")
        return EXIT_OK
    if action == "enable":
        argv = ["init", "--repair", "--orchestration", args.policy,
                "--orchestration-workers", str(args.workers),
                "--orchestration-retries", str(args.retries),
                "--orchestration-timeout", str(args.timeout),
                "--orchestration-mode", args.role_mode,
                "--orchestration-sandbox", args.sandbox]
        if args.cost:
            argv += ["--orchestration-cost-limit", args.cost]
        for flag in ("yes", "non_interactive", "dry_run"):
            if getattr(args, flag):
                argv.append("--" + flag.replace("_", "-"))
        return run_native(argv)
    if action == "disable":
        return disable_orchestration(root)
    if action == "workers":
        if not pinky:
            eprint("native pinky command unsupported")
            return EXIT_GATE
        return print_worker_view(root)
    if action in SESSION_ACTIONS:
        if native_windows_without_wsl():
            eprint("Brain/Pinky orchestration is POSIX-only on native Windows. Use WSL for session actions.")
            return EXIT_GATE
        if not brain:
            eprint("native brain command unsupported")
            return EXIT_GATE
        if action in {"start", "run", "step"} and not args.spec:
            eprint(f"{action} requires spec slug")
            return EXIT_USAGE
        native_action = "resume" if action == "compact" else action
        argv = ["brain", native_action]
        if args.spec:
            argv.append(args.spec)
        if args.session:
            argv += ["--session", args.session]
        if action in {"start", "run", "step"}:
            argv += ["--approval-policy", args.policy,
                     "--max-workers", str(args.workers),
                     "--max-retries", str(args.retries),
                     "--timeout-seconds", str(args.timeout * 60)]
            if args.cost:
                argv += ["--cost-limit", args.cost]
            if action == "run" and args.worker_cmd:
                argv += ["--worker-cmd", args.worker_cmd]
        elif action in {"pause", "resume", "cancel", "compact"} and not args.session:
            eprint(f"{action} requires --session")
            return EXIT_USAGE
        if args.json:
            argv.append("--json")
        return run_native(argv)
    eprint("usage: specd-workflow pinky-brain [status|enable|disable|start|run|step|pause|resume|cancel|compact|workers]")
    return EXIT_USAGE


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(prog="specd-workflow", description="Optional slash workflow wrappers for specd")
    sub = p.add_subparsers(dest="command", required=True)
    add_init_parser(sub)
    add_steer_parser(sub)
    add_spec_parser(sub)
    add_pinky_parser(sub)
    return p


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    return args.func(args)


if __name__ == "__main__":
    raise SystemExit(main())
