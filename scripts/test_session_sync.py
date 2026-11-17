"""No Chrome cookies, real SSH, systemd state or server mutations in these tests."""
import argparse
from contextlib import ExitStack
import fcntl
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

from scripts import manage_session_sync as manager


class ManagerTest(unittest.TestCase):
    def test_target_validation_and_unit_escaping(self):
        manager.validate("root@example.com:/opt/project", 9011)
        for target in ("-oProxyCommand=bad:/opt/project", "root@host:/", "root@host:relative",
                       "root@host:/opt/\nbad"):
            with self.subTest(target=target), self.assertRaises(ValueError):
                manager.validate(target, 9011)
        for port in (0, 65536):
            with self.assertRaises(ValueError):
                manager.validate("root@host:/opt/project", port)
        self.assertEqual(manager.unit_quote('a $HOME %x "b"'), '"a $$HOME %%x \\"b\\""')
        with self.assertRaises(ValueError):
            manager.unit_quote("line\nbreak")

    @patch.object(manager.subprocess, "run")
    @patch.object(manager, "systemctl")
    def test_install_creates_hourly_units_and_keeps_secrets_out(self, ctl, process):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            args = argparse.Namespace(target="root@host:/opt/project with space", port=9011,
                                      no_import_chatgpt=False)
            with patch.object(manager, "UNITS", root / "units"), patch.object(
                manager, "SETTINGS", root / "settings/config.json"
            ):
                manager.install(args)
                service = (root / f"units/{manager.NAME}.service").read_text()
                timer = (root / f"units/{manager.NAME}.timer").read_text()
                self.assertIn('"--batch"', service)
                self.assertIn('"--import-chatgpt"', service)
                self.assertIn('"root@host:/opt/project with space"', service)
                self.assertIn("OnCalendar=hourly", timer)
                self.assertIn("Persistent=true", timer)
                self.assertIn("TimeoutStartSec=45min", service)
                self.assertNotIn("OPENROUTER", service)
                self.assertEqual((root / "settings/config.json").stat().st_mode & 0o777, 0o600)
                ctl.assert_any_call("enable", "--now", f"{manager.NAME}.timer")

    @patch.object(manager, "systemctl")
    def test_control_actions(self, ctl):
        for action in ("restart", "stop", "status"):
            manager.perform(action, None)
        ctl.assert_any_call("restart", f"{manager.NAME}.timer")
        ctl.assert_any_call("disable", "--now", f"{manager.NAME}.timer")
        with patch.object(manager.subprocess, "run") as process:
            manager.perform("logs", None)
            self.assertIn("journalctl", process.call_args.args[0])

    def test_menu_and_cli_dispatch(self):
        with patch("sys.argv", ["manager", "status"]), patch.object(manager, "perform") as perform:
            manager.main()
            self.assertEqual(perform.call_args.args[0], "status")
        with patch("sys.argv", ["manager"]), patch("builtins.input", side_effect=["invalid", "5", "0"]), patch.object(
            manager, "perform", side_effect=ValueError("synthetic failure")
        ) as perform:
            manager.main()
            self.assertEqual(perform.call_count, 1)

    def test_uninstall_cancel_and_install_invalid_target(self):
        with patch("builtins.input", return_value="n"), patch.object(manager, "systemctl") as ctl:
            manager.perform("uninstall", None)
            ctl.assert_not_called()
        args = argparse.Namespace(target="bad", port=9011, no_import_chatgpt=True)
        with self.assertRaises(ValueError):
            manager.install(args)

    @unittest.skipUnless(shutil.which("systemd-analyze"), "systemd-analyze not installed")
    def test_systemd_accepts_rendered_units(self):
        settings = {"target": "root@host:/opt/project", "port": 9011, "import_chatgpt": False}
        with tempfile.TemporaryDirectory() as folder:
            service = Path(folder) / f"{manager.NAME}.service"
            timer = Path(folder) / f"{manager.NAME}.timer"
            service.write_text(manager.render_service(settings))
            timer.write_text(manager.TIMER)
            result = subprocess.run(["systemd-analyze", "--user", "verify", str(service), str(timer)],
                                    capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)

    def test_optional_import_and_nonblocking_run(self):
        service = manager.render_service({"target": "root@host:/opt/project", "port": 22,
                                          "import_chatgpt": False})
        self.assertNotIn("--import-chatgpt", service)
        with patch.object(manager, "systemctl") as ctl:
            manager.perform("run", None)
            ctl.assert_called_once_with("start", "--no-block", f"{manager.NAME}.service")

    @patch.object(manager, "systemctl")
    def test_uninstall_only_removes_owned_files(self, ctl):
        with tempfile.TemporaryDirectory() as folder, ExitStack() as stack:
            root = Path(folder)
            stack.enter_context(patch.object(manager, "UNITS", root))
            stack.enter_context(patch.object(manager, "SETTINGS", root / "settings.json"))
            stack.enter_context(patch("builtins.input", return_value="y"))
            stack.enter_context(patch.object(manager.subprocess, "run", return_value=subprocess.CompletedProcess([], 3)))
            for name in (f"{manager.NAME}.timer", f"{manager.NAME}.service", "settings.json", "keep.txt"):
                (root / name).write_text("synthetic")
            manager.perform("uninstall", None)
            self.assertEqual([p.name for p in root.iterdir()], ["keep.txt"])

    def test_uninstall_refuses_active_sync(self):
        with patch("builtins.input", return_value="y"), patch.object(
            manager.subprocess, "run", return_value=subprocess.CompletedProcess([], 0)
        ), self.assertRaisesRegex(ValueError, "running"):
            manager.perform("uninstall", None)


class SyncScriptTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        (self.root / "scripts").mkdir()
        source = Path(__file__).with_name("sync_sessions_to_server.sh")
        self.script = self.root / "scripts/sync_sessions_to_server.sh"
        shutil.copyfile(source, self.script)
        for name in ("COOKIES", "CHATGPT_COOKIES"):
            (self.root / name).mkdir()
            (self.root / name / "synthetic.txt").write_text("SYNTHETIC_NOT_A_COOKIE")
        self.bin = self.root / "bin"
        self.bin.mkdir()
        code = (f"#!{shutil.which('python3')}\nimport os,sys,json\n"
                "with open(os.environ['SYNC_TEST_LOG'],'a') as f: "
                "f.write(json.dumps([os.path.basename(sys.argv[0]),*sys.argv[1:]])+'\\n')\n"
                "if os.environ.get('SYNC_TEST_FAIL') == os.path.basename(sys.argv[0]): sys.exit(1)\n"
                "if sys.argv[0].endswith('ssh') and 'stat -c' in sys.argv[-1]: print('1200:1201')\n")
        for name in ("ssh", "rsync"):
            path = self.bin / name
            path.write_text(code)
            path.chmod(0o700)
        self.log = self.root / "calls.jsonl"
        self.env = dict(os.environ, PATH=f"{self.bin}:{os.environ['PATH']}", SYNC_TEST_LOG=str(self.log))

    def run_script(self, *args):
        return subprocess.run(["bash", str(self.script), *args], env=self.env,
                              capture_output=True, text=True, timeout=10)

    def calls(self):
        return [json.loads(row) for row in self.log.read_text().splitlines()] if self.log.exists() else []

    def test_batch_sync_preserves_owner_and_restarts_after_both_transfers(self):
        result = self.run_script("--batch", "--ssh-port", "9011", "root@host:/opt/project with space")
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = self.calls()
        transfers = [row for row in calls if row[0] == "rsync"]
        self.assertEqual(len(transfers), 2)
        for transfer in transfers:
            self.assertIn("--chown=1200:1201", transfer)
            self.assertIn("BatchMode=yes", " ".join(transfer))
            self.assertNotIn("--delete", transfer)
        self.assertIn("--force-recreate aistudio", calls[-1][-1])
        self.assertNotIn("SYNTHETIC_NOT_A_COOKIE", result.stdout + result.stderr)

    def test_failure_does_not_restart_server(self):
        self.env["SYNC_TEST_FAIL"] = "rsync"
        result = self.run_script("--batch", "root@host:/opt/project")
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(any("force-recreate" in str(row) for row in self.calls()))

    def test_no_restart_and_prune_are_explicit(self):
        result = self.run_script("--no-restart", "--prune", "root@host:/opt/project")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(all("--delete" in row for row in self.calls() if row[0] == "rsync"))
        self.assertFalse(any("force-recreate" in str(row) for row in self.calls()))

    def test_second_run_skips_while_lock_held(self):
        (self.root / ".local").mkdir()
        with (self.root / ".local/session-sync.lock").open("w") as lock:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            result = self.run_script("root@host:/opt/project")
        self.assertEqual(result.returncode, 0)
        self.assertIn("already running", result.stdout)
        self.assertEqual(self.calls(), [])

    def test_invalid_target_and_port_never_connect(self):
        for args in [("root@host:/",), ("root@host:relative",),
                     ("--ssh-port", "65536", "root@host:/opt/project"),
                     ("--ssh-port", "0", "root@host:/opt/project")]:
            with self.subTest(args=args):
                self.assertNotEqual(self.run_script(*args).returncode, 0)
        self.assertEqual(self.calls(), [])


if __name__ == "__main__":
    unittest.main()
