#!/usr/bin/env python3
"""Optional UX wrappers for specd slash workflows.

Thin glue only: all project/spec mutations delegate to native `specd`.
"""
from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
from pathlib import Path
from typing import Any, Iterable

EXIT_OK = 0
EXIT_GATE = 1
EXIT_USAGE = 2
EXIT_NOT_FOUND = 3

KNOWN_HOSTS = ["claude-code", "codex", "cursor", "antigravity", "vscode", "none"]
CANONICAL_STEERING = ["reasoning.md", "workflow.md", "product.md", "tech.md", "structure.md", "memory.md"]
ORCH_POLICIES = ["none", "manual", "planning", "session"]


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
    code, out, err = capture_native(["help", "--json"])
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


def add_init_parser(sub: argparse._SubParsersAction[argparse.ArgumentParser]) -> None:
    p = sub.add_parser("init", help="interactive/non-interactive specd init wrapper")
    p.set_defaults(func=cmd_init)
    p.add_argument("--agent", choices=["auto", "all", *KNOWN_HOSTS], default=None)
    p.add_argument("--dry-run", action="store_true")
    p.add_argument("--repair", action="store_true")
    p.add_argument("--refresh", action="store_true")
    p.add_argument("--json", action="store_true")
    p.add_argument("--orchestration", choices=ORCH_POLICIES, default="none")
    p.add_argument("--workers", "--orchestration-workers", dest="workers", type=int, default=4)
    p.add_argument("--retries", "--orchestration-retries", dest="retries", type=int, default=2)
    p.add_argument("--timeout", "--orchestration-timeout", dest="timeout", type=int, default=120)
    p.add_argument("--cost", "--orchestration-cost-limit", dest="cost", default=None)
    p.add_argument("--role-mode", "--orchestration-mode", dest="role_mode", choices=["inline", "delegate"], default="inline")
    p.add_argument("--sandbox", "--orchestration-sandbox", dest="sandbox", choices=["none", "bwrap", "container"], default="none")
    p.add_argument("--yes", action="store_true")
    p.add_argument("--non-interactive", action="store_true")
    p.add_argument("--probe-hosts", action="store_true", help=argparse.SUPPRESS)


def cmd_init(args: argparse.Namespace) -> int:
    if args.repair and args.refresh:
        eprint("--repair and --refresh are mutually exclusive")
        return EXIT_USAGE
    if args.probe_hosts:
        print("\n".join(probe_hosts()))
        return EXIT_OK
    if args.non_interactive or not sys.stdin.isatty():
        agent = args.agent or "auto"
    else:
        hosts = probe_hosts()
        print("Detected/known hosts: " + ", ".join(hosts + ["all"]))
        agent = args.agent or input("Agent [auto]: ").strip() or "auto"
        if agent not in ["auto", "all", *KNOWN_HOSTS]:
            eprint(f"invalid agent: {agent}")
            return EXIT_USAGE
    argv = ["init", "--agent", agent]
    for flag in ("dry_run", "repair", "refresh", "json", "non_interactive", "yes"):
        if getattr(args, flag):
            argv.append("--" + flag.replace("_", "-"))
    if args.orchestration != "none":
        argv += ["--orchestration", args.orchestration,
                 "--orchestration-workers", str(args.workers),
                 "--orchestration-retries", str(args.retries),
                 "--orchestration-timeout", str(args.timeout),
                 "--orchestration-mode", args.role_mode,
                 "--orchestration-sandbox", args.sandbox]
        if args.cost is not None:
            argv += ["--orchestration-cost-limit", str(args.cost)]
    return run_native(argv)


def add_steer_parser(sub: argparse._SubParsersAction[argparse.ArgumentParser]) -> None:
    p = sub.add_parser("steer", help="inspect steering files")
    sp = p.add_subparsers(dest="action")
    p.set_defaults(func=cmd_steer)
    show = sp.add_parser("show")
    show.add_argument("file", nargs="?", default=None)
    sp.add_parser("status")


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
            print(path.read_text(encoding="utf-8", errors="replace"), end="")
            if not path.read_text(encoding="utf-8", errors="replace").endswith("\n"):
                print()
        return EXIT_OK
    eprint("usage: specd-workflow steer [show [file]|status]")
    return EXIT_USAGE


def add_spec_parser(sub: argparse._SubParsersAction[argparse.ArgumentParser]) -> None:
    p = sub.add_parser("spec", help="spec workflow dashboard")
    sp = p.add_subparsers(dest="action")
    p.set_defaults(func=cmd_spec)
    sp.add_parser("list")
    cont = sp.add_parser("continue")
    cont.add_argument("slug", nargs="?")


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
    if action == "continue":
        code, slug = select_slug(args.slug)
        if code != 0:
            return code
        rc = run_native(["context", slug])
        if rc != 0:
            return rc
        print(f"Next: run `specd check {slug}` for planning gates, or `specd next {slug}` during execution.")
        return EXIT_OK
    eprint("usage: specd-workflow spec [list|continue [slug]]")
    return EXIT_USAGE


def add_pinky_parser(sub: argparse._SubParsersAction[argparse.ArgumentParser]) -> None:
    p = sub.add_parser("pinky-brain", help="Brain/Pinky status console")
    sp = p.add_subparsers(dest="action")
    p.set_defaults(func=cmd_pinky)
    sp.add_parser("status")


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


def cmd_pinky(args: argparse.Namespace) -> int:
    action = args.action or "status"
    if action != "status":
        eprint("usage: specd-workflow pinky-brain status")
        return EXIT_USAGE
    root = require_specd_root()
    if root is None:
        return EXIT_NOT_FOUND
    brain = native_has_command("brain")
    pinky = native_has_command("pinky")
    print(f"brain: {'available' if brain else 'unsupported'}")
    print(f"pinky: {'available' if pinky else 'unsupported'}")
    print(f"orchestration: {read_config_status(root)}")
    if brain:
        print_sessions()
    else:
        print("Enable/upgrade native specd with Brain/Pinky support for orchestration actions.")
    return EXIT_OK


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
