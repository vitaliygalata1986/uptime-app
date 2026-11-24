"""Regression tests for review routing and secret-safe tool diagnostics."""

import contextlib
import io
from pathlib import Path
import runpy
import subprocess
import sys
import tempfile
import unittest
from unittest import mock


HELPER = Path(__file__).with_name("static_checks.py")
REVIEW = runpy.run_path(str(HELPER))


class DiagnosticTests(unittest.TestCase):
    """Check observable output and status using real subprocess diagnostics."""

    def test_provider_token_in_stderr_is_hidden(self):
        """A compiler-style literal diagnostic must not expose a provider token."""
        token = "ghp_" + "Z" * 36
        checks = REVIEW["Checks"](10)
        output = io.StringIO()
        with tempfile.TemporaryDirectory() as temporary:
            # The subprocess receives the value from a file so RUN cannot echo it.
            diagnostic = Path(temporary) / "diagnostic.txt"
            diagnostic.write_text('error: Type "' + token + '" is not assignable to type "public"')
            command = [sys.executable, "-c", "import pathlib, sys; print(pathlib.Path(sys.argv[1]).read_text(), file=sys.stderr); sys.exit(1)", str(diagnostic)]
            with contextlib.redirect_stdout(output):
                ok = checks.run("TypeScript", command, temporary)
        self.assertFalse(ok)
        self.assertNotIn(token, output.getvalue())
        self.assertIn("tool output redacted", output.getvalue())
        self.assertEqual(checks.results, [("TypeScript", "FAIL")])

    def test_safe_diagnostics_remain_visible(self):
        """Redaction must preserve useful nonsecret diagnostics and failed status."""
        checks = REVIEW["Checks"](10)
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            ok = checks.run("compiler", [sys.executable, "-c", "print('error: missing argument'); raise SystemExit(1)"], HELPER.parent)
        self.assertFalse(ok)
        self.assertIn("error: missing argument", output.getvalue())
        self.assertEqual(checks.results, [("compiler", "FAIL")])


class WorkingSnapshotTests(unittest.TestCase):
    """Exercise the CLI flow against temporary Git working snapshots."""

    def setUp(self):
        """Create a clean repository with the helper at its supported location."""
        self.temporary = tempfile.TemporaryDirectory(prefix="uptime-review-tests-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.helper = self.root / ".agents/skills/code-review/scripts/static_checks.py"
        self.helper.parent.mkdir(parents=True)
        self.helper.write_bytes(HELPER.read_bytes())
        for args in (("init", "-q"), ("add", "."), ("-c", "user.name=Review fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")):
            subprocess.run(["git", "-C", str(self.root), *args], check=True, capture_output=True)

    def test_invalid_generated_go_does_not_pass_as_documentation(self):
        """Changed docs.go must select Go checks even without a usable toolchain."""
        docs = self.root / "backend/docs/docs.go"
        docs.parent.mkdir(parents=True)
        docs.write_text("invalid Go source\n")
        (self.root / "backend/go.mod").write_text("module fixture\n\ngo 9999.0\n")
        # Commit the module first so docs.go is the only working change.
        subprocess.run(["git", "-C", str(self.root), "add", "backend/go.mod"], check=True, capture_output=True)
        subprocess.run(["git", "-C", str(self.root), "-c", "user.name=Review fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "module"], check=True, capture_output=True)
        result = subprocess.run([sys.executable, str(self.helper), "--worktree"], capture_output=True, text=True, check=False)
        self.assertEqual(result.returncode, 2, result.stdout)
        self.assertIn("[BLOCKED] Go checks", result.stdout)
        self.assertNotIn("[SKIP] Go checks", result.stdout)

    def test_plain_credential_from_diff_is_hidden_without_name_in_diagnostic(self):
        """A scanned credential remains hidden when the compiler omits its name."""
        credential = "fixture" + "Q" * 24
        source = self.root / "backend/main.go"
        source.parent.mkdir()
        source.write_text('package main\nconst password = "' + credential + '"\n')
        diagnostic = self.root / "diagnostic.txt"
        diagnostic.write_text('cannot use string constant "' + credential + '" as int')

        def compiler(root, checks):
            """Simulate a compiler echoing an unnamed string constant."""
            checks.run("compiler", [sys.executable, "-c", "import pathlib, sys; print(pathlib.Path(sys.argv[1]).read_text()); sys.exit(1)", str(diagnostic)], root)

        output = io.StringIO()
        with mock.patch.dict(REVIEW["main"].__globals__, {"__file__": str(self.helper), "backend_checks": compiler}), mock.patch.object(sys, "argv", [str(self.helper), "--worktree"]), contextlib.redirect_stdout(output):
            status = REVIEW["main"]()
        self.assertEqual(status, 1)
        self.assertNotIn(credential, output.getvalue())
        self.assertIn("[FAIL] secret heuristics", output.getvalue())
        self.assertIn("[FAIL] compiler: exit 1", output.getvalue())
        self.assertIn("tool output redacted", output.getvalue())

    def test_documentation_formats_skip_language_checks(self):
        """Noncompiled generated artifacts alone do not require Go or Node."""
        for suffix in ("json", "yaml", "yml", "html", "md"):
            with self.subTest(suffix=suffix):
                self.assertEqual(REVIEW["project_selection"](["backend/docs/swagger." + suffix]), (False, False))


if __name__ == "__main__":
    unittest.main()
