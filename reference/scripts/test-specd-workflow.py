#!/usr/bin/env python3
"""Behavior tests for optional specd workflow wrappers."""
from __future__ import annotations

import json
import os
import shutil
import stat
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PY = ROOT / "scripts" / "specd-workflow.py"
SH = ROOT / "scripts" / "specd-workflow.sh"
FAKE = ROOT / "scripts" / "testdata" / "fake-specd.py"


class WorkflowHarness(unittest.TestCase):
    def setUp(self) -> None:
        self.td = tempfile.TemporaryDirectory()
        self.tmp = Path(self.td.name)
        self.repo = self.tmp / "repo"
        self.repo.mkdir()
        (self.repo / ".specd" / "steering").mkdir(parents=True)
        spec_dir = self.repo / ".specd" / "specs" / "demo"
        spec_dir.mkdir(parents=True)
        self.state = spec_dir / "state.json"
        self.tasks = spec_dir / "tasks.md"
        self.state.write_text('{"revision":1}\n', encoding="utf-8")
        self.tasks.write_text("- [ ] T1 demo\n", encoding="utf-8")
        (self.repo / ".specd" / "config.json").write_text('{"orchestration":{"enabled":true},"keep":1}\n', encoding="utf-8")
        self.bin = self.tmp / "bin"
        self.bin.mkdir()
        fake_bin = self.bin / "specd"
        shutil.copy2(FAKE, fake_bin)
        fake_bin.chmod(fake_bin.stat().st_mode | stat.S_IXUSR)
        self.log = self.tmp / "argv.log"
        self.env = os.environ.copy()
        self.env.update(
            PATH=f"{self.bin}{os.pathsep}{self.env.get('PATH', '')}",
            SPECD_FAKE_LOG=str(self.log),
            SPECD_WORKFLOW_PY=str(PY),
            SPECD_WORKFLOW_TESTING="1",
            PYTHONPATH="",
        )

    def tearDown(self) -> None:
        self.td.cleanup()

    def run_py(self, *args: str, env: dict[str, str] | None = None) -> subprocess.CompletedProcess[str]:
        e = self.env.copy()
        if env:
            e.update(env)
        return subprocess.run([sys.executable, str(PY), *args], cwd=self.repo, env=e, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    def run_sh(self, *args: str) -> subprocess.CompletedProcess[str]:
        cmd = ". '%s'; specd_workflow %s" % (SH, " ".join(args))
        return subprocess.run(["/bin/sh", "-c", cmd], cwd=self.repo, env=self.env, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    def argv_log(self) -> list[list[str]]:
        if not self.log.exists():
            return []
        return [json.loads(line) for line in self.log.read_text(encoding="utf-8").splitlines() if line]

    def assert_no_forbidden_mutation(self) -> None:
        self.assertEqual(self.state.read_text(encoding="utf-8"), '{"revision":1}\n')
        self.assertEqual(self.tasks.read_text(encoding="utf-8"), "- [ ] T1 demo\n")
        for argv in self.argv_log():
            self.assertFalse(argv[:1] == ["task"] and "--status" in argv and "complete" in argv, argv)
            self.assertNotEqual(argv[:2], ["pinky", "report"], argv)


class WorkflowTests(WorkflowHarness):
    def test_fake_harness_status_and_brain_outputs(self) -> None:
        p = self.run_py("pinky-brain", "status")
        self.assertEqual(p.returncode, 0, p.stderr)
        self.assertIn("brain: available", p.stdout)
        self.assertIn("orchestration: enabled", p.stdout)
        self.assertIn(["brain", "resume", "--list", "--json"], self.argv_log())

    def test_shell_python_parity_for_delegated_spec_check(self) -> None:
        p1 = self.run_py("spec", "check", "demo")
        log1 = self.argv_log()[-1]
        self.log.unlink()
        p2 = self.run_sh("spec check demo")
        log2 = self.argv_log()[-1]
        self.assertEqual(p1.returncode, 0, p1.stderr)
        self.assertEqual(p2.returncode, 0, p2.stderr)
        self.assertEqual(log1, ["check", "demo"])
        self.assertEqual(log2, ["check", "demo"])

    def test_native_exit_codes_propagate(self) -> None:
        for rc in (0, 1, 2, 3):
            with self.subTest(rc=rc):
                p = self.run_py("spec", "check", "demo", env={"SPECD_FAKE_EXITS": json.dumps({"check demo": rc})})
                self.assertEqual(p.returncode, rc)

    def test_spec_continue_never_auto_completes_task_or_edits_files(self) -> None:
        p = self.run_py("spec", "continue", "demo")
        self.assertEqual(p.returncode, 0, p.stderr)
        self.assertIn(["context", "demo"], self.argv_log())
        self.assertIn(["next", "demo"], self.argv_log())
        self.assert_no_forbidden_mutation()

    def test_steer_memory_prints_existing_and_missing_without_create(self) -> None:
        mem = self.repo / ".specd" / "steering" / "memory.md"
        mem.write_text("known learning\n", encoding="utf-8")
        p = self.run_py("steer", "memory")
        self.assertEqual(p.returncode, 0, p.stderr)
        self.assertIn("--- memory.md ---\nknown learning\n", p.stdout)
        mem.unlink()
        p2 = self.run_py("steer", "memory")
        self.assertEqual(p2.returncode, 0, p2.stderr)
        self.assertIn("warning: memory.md missing", p2.stdout)
        self.assertFalse(mem.exists())

    def test_spec_mode_delegates_only_when_native_supported(self) -> None:
        p = self.run_py("spec", "mode", "demo", "--set", "orchestrated", env={"SPECD_FAKE_HELP_JSON": '{"commands":["mode","status"]}'})
        self.assertEqual(p.returncode, 0, p.stderr)
        self.assertIn(["mode", "demo", "--set", "orchestrated"], self.argv_log())
        self.log.unlink()
        p2 = self.run_py("spec", "mode", "demo", env={"SPECD_FAKE_HELP_JSON": '{"commands":["status"]}', "SPECD_FAKE_HELP_TEXT": "status"})
        self.assertEqual(p2.returncode, 1)
        self.assertIn("mode command unsupported", p2.stderr)
        self.assertNotIn(["mode", "demo"], self.argv_log())

    def test_workers_view_never_forges_pinky_report(self) -> None:
        workers = self.repo / ".specd" / "runtime" / "sessions" / "s1" / "workers" / "w1"
        workers.mkdir(parents=True)
        (workers / "lease.json").write_text('{"status":"claimed","task":"T1"}\n', encoding="utf-8")
        p = self.run_py("pinky-brain", "workers")
        self.assertEqual(p.returncode, 0, p.stderr)
        self.assertIn("s1\tw1\tclaimed\tT1", p.stdout)
        self.assert_no_forbidden_mutation()

    def test_platform_guard_blocks_mutating_orchestration_on_native_windows(self) -> None:
        p = self.run_py("pinky-brain", "start", "demo", env={"SPECD_WORKFLOW_PLATFORM": "Windows"})
        self.assertEqual(p.returncode, 1)
        self.assertIn("POSIX-only", p.stderr)
        self.assertNotIn(["brain", "start", "demo"], self.argv_log())

    def test_unsupported_brain_propagates_gate_failure(self) -> None:
        p = self.run_py("pinky-brain", "start", "demo", env={"SPECD_FAKE_HELP_JSON": '{"commands":["status"]}', "SPECD_FAKE_HELP_TEXT": "status"})
        self.assertEqual(p.returncode, 1)
        self.assertIn("native brain command unsupported", p.stderr)


if __name__ == "__main__":
    unittest.main(verbosity=2)
