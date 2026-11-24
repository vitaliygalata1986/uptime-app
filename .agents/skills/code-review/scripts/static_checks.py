#!/usr/bin/env python3
"""Run diff-scoped, non-fixing checks; never print suspected secret values."""

import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import subprocess
import sys
import tempfile


class ScopeError(Exception):
    """The requested diff cannot be checked against the current checkout."""


def git(root, *args):
    """Return raw Git output, raising a safe error without patch contents."""
    result = subprocess.run(
        ["git", "--literal-pathspecs", "-C", str(root), *args], capture_output=True, check=False
    )
    if result.returncode:
        raise ScopeError("Git could not resolve/read the requested scope; check refs and checkout.")
    return result.stdout


def resolve(root, ref):
    """Resolve a revision as a commit, preventing option injection."""
    return git(root, "rev-parse", "--verify", "--end-of-options", ref + "^{commit}").decode().strip()


def names(raw):
    """Decode NUL-separated Git filenames without splitting on whitespace."""
    return [os.fsdecode(value) for value in raw.split(b"\0") if value]


def scope(root, args):
    """Return immutable base/head, paths and untracked paths for the requested snapshot."""
    head = resolve(root, "HEAD" if args.worktree else args.commit or args.head)
    untracked = names(git(root, "ls-files", "--others", "--exclude-standard", "-z"))
    dirty = git(root, "status", "--porcelain", "-z", "--untracked-files=all")
    if not args.worktree and (head != resolve(root, "HEAD") or dirty):
        raise ScopeError("Committed checks need HEAD at the reviewed SHA and a clean tree, including untracked files. Use an isolated checkout or --worktree for local changes.")
    if args.worktree:
        base = head
        head = None
    elif args.commit:
        parents = git(root, "rev-list", "--parents", "-n", "1", head).decode().split()
        # A merge is compared with its first parent; a root uses Git's empty tree.
        base = parents[1] if len(parents) > 1 else git(root, "hash-object", "-t", "tree", "--stdin").decode().strip()
    else:
        base = git(root, "merge-base", resolve(root, args.base), head).decode().strip()
    revisions = [base] + ([head] if head else [])
    paths = names(git(root, "diff", "--name-only", "--no-renames", "-z", *revisions, "--"))
    if args.worktree:
        paths.extend(untracked)
    return base, head, sorted(set(paths)), untracked if args.worktree else []


def project_selection(paths):
    """Choose whole-project checks, including deleted paths and shared runtime inputs."""
    shared = any(
        path in {"package.json", "package-lock.json", "npm-shrinkwrap.json", ".node-version", ".nvmrc", "Dockerfile", "Makefile"}
        or path.startswith(("compose.", "docker-compose.", ".github/workflows/", "scripts/"))
        for path in paths
    )
    # Generated Go documentation is imported by the API and must still compile.
    code_paths = [p for p in paths if not p.endswith(".md") and not (
        p.startswith("backend/docs/") and p.endswith((".json", ".yaml", ".yml", ".html"))
    )]
    return shared or any(p.startswith("backend/") for p in code_paths), shared or any(p.startswith("frontend/") for p in code_paths)


def fingerprint(root):
    """Detect content edits even when porcelain status keeps the same path/status."""
    digest = hashlib.sha256()
    digest.update(git(root, "rev-parse", "HEAD"))
    digest.update(git(root, "status", "--porcelain", "-z", "--untracked-files=all"))
    digest.update(git(root, "diff", "--binary", "--no-ext-diff", "--no-textconv", "HEAD", "--"))
    for path in names(git(root, "ls-files", "--others", "--exclude-standard", "-z")):
        file = root / path
        digest.update(os.fsencode(path) + b"\0")
        digest.update(os.fsencode(os.readlink(file)) if file.is_symlink() else file.read_bytes())
    return digest.digest()


PRIVATE_KEY = re.compile(r"-----BEGIN (?:RSA |EC |OPENSSH |DSA |ENCRYPTED )?PRIVATE KEY-----")
TOKEN = re.compile(r"\b(?:AKIA[0-9A-Z]{16}|ASIA[0-9A-Z]{16}|gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|sk-(?:proj-|svcacct-)?[A-Za-z0-9_-]{20,})\b")
ASSIGNMENT = re.compile(r"(?i)[\"']?\b(?:password|passwd|api[_-]?key|client[_-]?secret|jwt[_-]?secret|access[_-]?token|refresh[_-]?token|database_url)[\"']?\s*[:=]\s*[\"']([^\"'\r\n]{12,})[\"']")
ENV_SECRET = re.compile(r"(?i)^\s*(?:export\s+)?(?:JWT_SECRET|API_KEY|CLIENT_SECRET|DATABASE_URL|PASSWORD)\s*=\s*([^\s\"']{12,})\s*$")
CREDENTIAL_URL = re.compile(r"://[^/\s:@]+:([^@\s/]+)@")


def placeholder(value):
    """Recognize explicit placeholders, not entire example/test directories."""
    lower = value.lower()
    return (
        lower.startswith(("test-", "test_", "dummy-", "dummy_", "example-", "example_", "replace-", "replace_", "your-", "your_", "${", "<"))
        or lower in {"correct horse battery staple", "change-me", "changeme", "test_only_password"}
    )


def secret_kinds(line):
    """Return heuristic rule names only; never return captured values."""
    kinds = []
    if PRIVATE_KEY.search(line):
        kinds.append("private-key marker")
    if TOKEN.search(line):
        kinds.append("provider-token pattern")
    if any(not placeholder(m.group(1)) for pattern in (ASSIGNMENT, ENV_SECRET, CREDENTIAL_URL) for m in pattern.finditer(line)):
        kinds.append("possible hard-coded credential")
    return kinds


def added_lines(patch):
    """Yield new-side line numbers and text from a zero-context Git patch."""
    number = None
    for line in patch.decode("utf-8", errors="replace").splitlines():
        match = re.match(r"^@@ .*\+(\d+)(?:,\d+)? @@", line)
        if match:
            number = int(match.group(1))
        elif line.startswith("diff --git "):
            number = None
        elif number is not None and line.startswith("+"):
            yield number, line[1:]
            number += 1
        elif number is not None and line.startswith(" "):
            number += 1


def inspect_changes(root, revisions, paths, untracked):
    """Inspect added content and filenames without following untracked symlinks."""
    secrets, whitespace = [], []
    untracked = set(untracked)
    for path in paths:
        filename = PurePosixPath(path).name
        exists = (root / path).exists() or (root / path).is_symlink()
        sensitive_name = (filename == ".env" or filename.startswith(".env.") and filename != ".env.example") or filename.endswith((".p12", ".pfx", ".key", ".pem"))
        if exists and sensitive_name:
            secrets.append(f"{path!r}: sensitive filename; inspect manually")
        if path in untracked:
            file = root / path
            raw = os.fsencode(os.readlink(file)) if file.is_symlink() else file.read_bytes()
            lines = [] if b"\0" in raw else enumerate(raw.decode("utf-8", errors="replace").splitlines(), 1)
        else:
            patch = git(root, "diff", "--no-ext-diff", "--no-textconv", "--no-color", "--no-renames", "--unified=0", *revisions, "--", path)
            lines = added_lines(patch)
        for number, line in lines:
            for kind in secret_kinds(line):
                secrets.append(f"{path!r}:{number}: {kind} [value redacted]")
            if path in untracked and (line.endswith((" ", "\t")) or re.match(r"^(?:<{7}|={7}|>{7})(?: |$)", line)):
                whitespace.append(f"{path!r}:{number}: trailing whitespace or conflict marker")
    return secrets, whitespace


class Checks:
    """Collect independent results and preserve failures alongside blocked checks."""

    def __init__(self, timeout):
        self.results = []
        self.timeout = timeout
        self.suppress_output = False

    def record(self, label, status, detail=""):
        """Record one status without changing previously recorded results."""
        self.results.append((label, status))
        print(f"[{status}] {label}" + (f": {detail}" if detail else ""), flush=True)

    def run(self, label, command, cwd, env=None, empty_output=False):
        """Run a non-fixing command, suppressing diagnostics that may expose secrets."""
        print(f"RUN {label}: {' '.join(map(str, command))}", flush=True)
        try:
            result = subprocess.run(command, cwd=cwd, env=env, capture_output=True, text=True, errors="replace", timeout=self.timeout, check=False)
        except (OSError, subprocess.TimeoutExpired) as exc:
            self.record(label, "BLOCKED", type(exc).__name__)
            return False
        output = result.stdout + result.stderr
        if output:
            # Compilers can echo a scanned credential without its assignment/name.
            if self.suppress_output or any(secret_kinds(line) for line in output.splitlines()):
                print("[tool output redacted: possible secret content]", flush=True)
            else:
                print(output.rstrip(), flush=True)
        ok = result.returncode == 0 and (not empty_output or not output.strip())
        self.record(label, "PASS" if ok else "FAIL", f"exit {result.returncode}")
        return ok

    def finish(self):
        """Print an explicit summary and return 0/pass, 1/fail or 2/incomplete."""
        print("\nCheck summary:", flush=True)
        for label, status in self.results:
            print(f"  {status}: {label}")
        states = {status for _, status in self.results}
        return 2 if "BLOCKED" in states else 1 if "FAIL" in states else 0


def backend_checks(root, checks):
    """Run Go formatting, vet and compilation without executing tests or migrations."""
    env = {**os.environ, "GOTOOLCHAIN": "local", "GOTELEMETRY": "off"}
    required = re.search(r"^go (\d+)\.(\d+)", (root / "backend/go.mod").read_text(), re.M)
    version = subprocess.run(["go", "version"], env=env, capture_output=True, text=True, check=False) if shutil.which("go") else None
    actual = re.search(r"go(\d+)\.(\d+)", version.stdout) if version and version.returncode == 0 else None
    if not required or not actual or tuple(map(int, actual.groups())) < tuple(map(int, required.groups())) or not shutil.which("gofmt"):
        checks.record("Go checks", "BLOCKED", "Go/gofmt missing or Go below backend/go.mod; no toolchain installed automatically")
        return
    files = names(git(root, "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", "backend/"))
    files = sorted({p for p in files if p.endswith(".go") and (root / p).is_file() and not (root / p).is_symlink()})
    if files:
        checks.run("gofmt", ["gofmt", "-l", *files], root, env, empty_output=True)
    checks.run("go vet", ["go", "vet", "-mod=readonly", "./..."], root / "backend", env)
    with tempfile.TemporaryDirectory(prefix="uptime-review-go-") as output:
        checks.run("go build", ["go", "build", "-mod=readonly", "-o", str(Path(output) / "api"), "./cmd/api"], root / "backend", env)


def frontend_checks(root, checks):
    """Run installed ESLint, Next route type generation and TypeScript checks."""
    frontend = root / "frontend"
    version = subprocess.run(["node", "--version"], capture_output=True, text=True, check=False) if shutil.which("node") else None
    actual = re.match(r"v(\d+)\.(\d+)", version.stdout) if version and version.returncode == 0 else None
    supported = actual and (int(actual[1]) >= 24 or int(actual[1]) == 22 and int(actual[2]) >= 13)
    tools = [frontend / "node_modules" / p for p in ("eslint/bin/eslint.js", "next/dist/bin/next", "typescript/bin/tsc")]
    if not supported or not shutil.which("npm") or not all(p.is_file() for p in tools):
        checks.record("Next.js checks", "BLOCKED", "need Node 22.13+ (22.x) or 24+, npm and installed frontend dependencies")
        return
    try:
        manifest = json.loads((frontend / "package.json").read_text())
        locked = json.loads((frontend / "package-lock.json").read_text())["packages"]
        for section in ("dependencies", "devDependencies"):
            if manifest.get(section, {}) != locked[""].get(section, {}):
                checks.record("Next.js dependencies", "BLOCKED", "package.json and lockfile declarations disagree")
                return
            for name in manifest.get(section, {}):
                installed = json.loads((frontend / "node_modules" / name / "package.json").read_text())
                if installed["version"] != locked["node_modules/" + name]["version"]:
                    checks.record("Next.js dependencies", "BLOCKED", f"installed {name} does not match the reviewed lockfile")
                    return
    except (OSError, ValueError, KeyError, TypeError):
        checks.record("Next.js dependencies", "BLOCKED", "missing/invalid manifest, lockfile or installed direct dependency metadata")
        return
    env = {**os.environ, "NEXT_TELEMETRY_DISABLED": "1"}
    checks.run("ESLint", ["npm", "run", "lint", "--", "--no-cache"], frontend, env)
    generated = checks.run("Next.js typegen", ["node", str(tools[1]), "typegen"], frontend, env)
    if generated:
        checks.run("TypeScript", ["node", str(tools[2]), "--noEmit", "--incremental", "false"], frontend, env)
    else:
        checks.record("TypeScript", "BLOCKED", "route type generation failed")


def main():
    """Resolve the scope, run every applicable static group and summarize evidence."""
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--commit", help="single commit, first parent for merges")
    mode.add_argument("--base", help="branch/PR base; compared from merge-base")
    mode.add_argument("--worktree", action="store_true", help="final working files versus HEAD, including untracked")
    parser.add_argument("--head", default="HEAD", help="head for --base only")
    parser.add_argument("--timeout", type=int, default=300, help="seconds per language command (default 300)")
    args = parser.parse_args()
    if args.timeout <= 0 or args.head != "HEAD" and not args.base:
        parser.error("timeout must be positive; --head is only valid with --base")
    checks = Checks(args.timeout)
    # The helper is portable to a temporary checkout when copied with the skill.
    root = Path(__file__).resolve().parents[4]
    try:
        if Path(os.fsdecode(git(root, "rev-parse", "--show-toplevel")).strip()).resolve() != root:
            raise ScopeError("Place the helper at .agents/skills/code-review/scripts/ inside the target repository.")
        base, head, paths, untracked = scope(root, args)
        before = fingerprint(root)
        print(f"Scope: {base} -> {head or 'working tree (staged + unstaged + untracked)'}", flush=True)
        for path in paths:
            print(f"  {path!r}")
        revisions = [base] + ([head] if head else [])
        diff = subprocess.run(["git", "-C", str(root), "diff", "--check", *revisions, "--"], capture_output=True, check=False)
        secrets, whitespace = inspect_changes(root, revisions, paths, untracked)
        checks.suppress_output = bool(secrets)
        # Git's whitespace output can include source lines; report locations only.
        locations = [line for line in diff.stdout.decode("utf-8", errors="replace").splitlines() if not line.startswith("+")]
        checks.record("diff whitespace", "FAIL" if diff.returncode or whitespace else "PASS", "\n".join(locations + whitespace))
        checks.record("secret heuristics", "FAIL" if secrets else "PASS", "\n".join(secrets) or "added text only; manual review still required")
        backend, frontend = project_selection(paths)
        if backend:
            try:
                backend_checks(root, checks)
            except OSError as exc:
                checks.record("Go prerequisites", "BLOCKED", type(exc).__name__)
        else:
            checks.record("Go checks", "SKIP", "no affected backend/shared runtime paths")
        if frontend:
            try:
                frontend_checks(root, checks)
            except OSError as exc:
                checks.record("Next.js prerequisites", "BLOCKED", type(exc).__name__)
        else:
            checks.record("Next.js checks", "SKIP", "no affected frontend/shared runtime paths")
        after = fingerprint(root)
        if before != after:
            checks.record("snapshot stability", "BLOCKED", "working-tree content or status changed during checks; inspect changes and rerun")
    except (ScopeError, OSError) as exc:
        checks.record("review scope/prerequisites", "BLOCKED", str(exc) if isinstance(exc, ScopeError) else type(exc).__name__)
    return checks.finish()


if __name__ == "__main__":
    sys.exit(main())
