#!/usr/bin/env python3
"""Deterministic fake specd for wrapper tests."""
from __future__ import annotations

import json
import os
import sys

FORBIDDEN = [
    ["task", "--status", "complete"],
    ["pinky", "report"],
]


def log(argv: list[str]) -> None:
    path = os.environ.get("SPECD_FAKE_LOG")
    if path:
        with open(path, "a", encoding="utf-8") as f:
            f.write(json.dumps(argv) + "\n")


def contains_subsequence(argv: list[str], seq: list[str]) -> bool:
    if len(seq) > len(argv):
        return False
    return any(argv[i : i + len(seq)] == seq for i in range(len(argv) - len(seq) + 1))


def main() -> int:
    argv = sys.argv[1:]
    log(argv)
    for seq in FORBIDDEN:
        if contains_subsequence(argv, seq):
            print("forbidden fake specd command: " + " ".join(argv), file=sys.stderr)
            return 99

    exits = json.loads(os.environ.get("SPECD_FAKE_EXITS", "{}"))
    key = " ".join(argv)
    prefix = " ".join(argv[:2])
    rc = int(exits.get(key, exits.get(prefix, os.environ.get("SPECD_FAKE_EXIT", "0"))))

    if argv == ["help", "--json"]:
        print(os.environ.get("SPECD_FAKE_HELP_JSON", '{"commands":["brain","pinky","status","check","context","next"]}'))
        return rc
    if argv == ["help"]:
        print(os.environ.get("SPECD_FAKE_HELP_TEXT", "brain pinky status check context next"))
        return rc
    if argv == ["status", "--json"]:
        out = os.environ.get("SPECD_FAKE_STATUS_JSON")
        if out is None:
            out = '{"specs":[{"spec":"demo","status":"executing","phase":"executing","complete":0,"total":1}]}'
        print(out)
        return rc
    if argv == ["status"]:
        print(os.environ.get("SPECD_FAKE_STATUS_TEXT", "demo [executing]"))
        return rc
    if argv == ["brain", "resume", "--list", "--json"]:
        print(os.environ.get("SPECD_FAKE_SESSIONS_JSON", '{"sessions":[]}'))
        return rc
    if argv and argv[0] == "context":
        print("context for " + (argv[1] if len(argv) > 1 else "?"))
        return rc
    if argv and argv[0] == "next":
        print("next task")
        return rc
    print("fake specd: " + " ".join(argv))
    return rc


if __name__ == "__main__":
    raise SystemExit(main())
