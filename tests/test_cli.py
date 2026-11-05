import contextlib
import io
import json
import os
import stat
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from dev_workspaces.cli import config_path, load_config, main


class CliTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name).resolve()
        self.environment = patch.dict(os.environ, {"XDG_CONFIG_HOME": str(self.root / "config")})
        self.environment.start()
        self.addCleanup(self.environment.stop)
        self.folder = self.root / "project with spaces; $(nothing)"
        self.folder.mkdir()
        self.workspace = self.root / "web.code-workspace"
        self.workspace.write_text('{"folders": []}')

    def invoke(self, *args):
        self.stdout = io.StringIO()
        self.stderr = io.StringIO()
        with contextlib.redirect_stdout(self.stdout), contextlib.redirect_stderr(self.stderr):
            return main(list(args))

    def add(self, name="web", path=None):
        self.assertEqual(self.invoke("add", name, str(path or self.folder)), 0)

    def test_register_folder_and_list(self):
        self.add()
        self.assertEqual(load_config(config_path())["projects"]["web"], str(self.folder))
        self.assertEqual(self.invoke("list"), 0)
        self.assertIn(str(self.folder), self.stdout.getvalue())

    def test_register_workspace(self):
        self.add(path=self.workspace)
        self.assertEqual(load_config(config_path())["projects"]["web"], str(self.workspace))

    def test_relative_and_default_path(self):
        with patch("os.getcwd", return_value=str(self.folder)):
            self.assertEqual(self.invoke("add", "current"), 0)
        self.assertEqual(load_config(config_path())["projects"]["current"], str(self.folder))

    def test_duplicate_requires_replace(self):
        self.add()
        before = config_path().read_bytes()
        self.assertEqual(self.invoke("add", "web", str(self.workspace)), 1)
        self.assertEqual(config_path().read_bytes(), before)
        self.assertEqual(self.invoke("add", "web", str(self.workspace), "--replace"), 0)
        self.assertEqual(load_config(config_path())["projects"]["web"], str(self.workspace))

    def test_invalid_names(self):
        for name in ("add", "open", "two words", "../bad", "-option", ""):
            with self.subTest(name=name):
                # '--' allows argparse to pass names beginning with '-'.
                self.assertEqual(self.invoke("add", "--", name, str(self.folder)), 1)

    def test_reject_missing_or_regular_file(self):
        ordinary = self.root / "file.txt"
        ordinary.touch()
        for path in (ordinary, self.root / "missing"):
            self.assertEqual(self.invoke("add", "bad", str(path)), 1)
        self.assertFalse(config_path().exists())

    def test_remove_keeps_files(self):
        self.add()
        self.assertEqual(self.invoke("remove", "web"), 0)
        self.assertTrue(self.folder.is_dir())
        self.assertEqual(load_config(config_path())["projects"], {})
        self.assertEqual(self.invoke("remove", "web"), 1)

    def test_editor_arguments_and_workspace_passed_literally(self):
        self.add(path=self.workspace)
        self.assertEqual(self.invoke("editor", "code", "--new-window"), 0)
        with patch("dev_workspaces.cli.subprocess.run") as launched:
            launched.return_value.returncode = 0
            self.assertEqual(self.invoke("web"), 0)
            launched.assert_called_once_with(
                ["code", "--new-window", str(self.workspace)], check=False
            )

    def test_real_fake_editor_receives_spaces_and_propagates_status(self):
        self.add()
        capture = self.root / "captured.json"
        script = self.root / "fake editor.py"
        script.write_text(
            "import json, pathlib, sys\n"
            f"pathlib.Path({str(capture)!r}).write_text(json.dumps(sys.argv[1:]))\n"
            "sys.exit(7)\n"
        )
        import sys

        self.assertEqual(self.invoke("editor", sys.executable, str(script), "--flag"), 0)
        self.assertEqual(self.invoke("open", "web"), 7)
        self.assertEqual(json.loads(capture.read_text()), ["--flag", str(self.folder)])

    def test_unknown_name_suggests(self):
        self.add()
        self.assertEqual(self.invoke("webb"), 1)
        self.assertIn("Did you mean web", self.stderr.getvalue())

    def test_missing_target_and_list_marker(self):
        self.add()
        self.folder.rmdir()
        self.assertEqual(self.invoke("web"), 1)
        self.invoke("list")
        self.assertIn("[missing]", self.stdout.getvalue())

    def test_missing_editor(self):
        self.add()
        with patch("dev_workspaces.cli.subprocess.run", side_effect=FileNotFoundError):
            self.assertEqual(self.invoke("web"), 1)
        self.assertIn("dev editor", self.stderr.getvalue())

    def test_editor_requires_command(self):
        self.assertEqual(self.invoke("editor"), 1)
        self.assertFalse(config_path().exists())

    def test_config_path_does_not_create_config(self):
        self.assertEqual(self.invoke("config"), 0)
        self.assertEqual(self.stdout.getvalue().strip(), str(config_path()))
        self.assertFalse(config_path().exists())

    def test_empty_list(self):
        self.assertEqual(self.invoke("list"), 0)
        self.assertIn("dev add", self.stdout.getvalue())

    def test_no_projects_picker_explains_setup(self):
        self.assertEqual(self.invoke(), 1)
        self.assertIn("dev add", self.stderr.getvalue())

    def test_noninteractive_picker_does_not_hang(self):
        self.add()
        with patch("sys.stdin.isatty", return_value=False):
            self.assertEqual(self.invoke(), 1)
        self.assertIn("dev NAME", self.stderr.getvalue())

    def test_picker_filter_then_select(self):
        self.add("calendar")
        self.add("web")
        with (
            patch("sys.stdin.isatty", return_value=True),
            patch("builtins.input", side_effect=["cal", "1"]),
            patch("dev_workspaces.cli.subprocess.run") as launched,
        ):
            launched.return_value.returncode = 0
            self.assertEqual(self.invoke(), 0)
            launched.assert_called_once_with(["code", str(self.folder)], check=False)

    def test_picker_cancel_and_interrupt(self):
        self.add()
        for answer, expected in (("", 0), (EOFError(), 130), (KeyboardInterrupt(), 130)):
            with (
                patch("sys.stdin.isatty", return_value=True),
                patch("builtins.input", side_effect=[answer]),
                patch("dev_workspaces.cli.subprocess.run") as launched,
            ):
                self.assertEqual(self.invoke(), expected)
                launched.assert_not_called()

    def test_malformed_config_preserved(self):
        config_path().parent.mkdir(parents=True)
        for content in ('{"bad":', "[]", '{"editor": "code"}', '{"projects": {"x": 3}}'):
            config_path().write_text(content)
            self.assertEqual(self.invoke("list"), 1)
            self.assertEqual(config_path().read_text(), content)

    def test_private_config_permissions(self):
        self.add()
        self.assertEqual(stat.S_IMODE(config_path().stat().st_mode), 0o600)

    def test_failed_atomic_replace_preserves_config(self):
        self.add()
        before = config_path().read_bytes()
        with patch("dev_workspaces.cli.Path.replace", side_effect=OSError("write failed")):
            self.assertEqual(self.invoke("add", "other", str(self.workspace)), 1)
        self.assertEqual(config_path().read_bytes(), before)
        self.assertEqual(list(config_path().parent.iterdir()), [config_path()])


if __name__ == "__main__":
    unittest.main()
