---
name: code-review
description: Review this product's Go backend and Next.js frontend commits, branches, PRs or working changes using automatic static checks and an architecture-aware checklist. Return prioritized findings without applying fixes.
---

# Product Code Review

Inspect the actual diff and affected callers, guards, persistence and tests. Report in the user's language. This workflow does not edit product code, install dependencies, regenerate contracts, create branches, commit, push or post reviews. Tooling can execute repository configuration: run it only in a trusted checkout, or an approved isolated environment for untrusted contributions.

## 1. Establish the Target

Read root `AGENTS.md` and the affected directory guides. Preserve local changes. Resolve revisions to immutable SHAs and state the target in the report:

- **Commit:** `git rev-parse --verify '<ref>^{commit}'`, `git show --format=fuller --stat <sha>`, then `git diff <sha>^ <sha>`. For a merge use the first parent and disclose that choice; for a root commit use the empty tree.
- **Branch/PR:** identify base/head, then review `git merge-base <base> <head>` through head. `gh pr view <number> --json baseRefName,headRefName,headRefOid` supplies PR metadata; fetch missing refs only as needed and permitted. Never post a review implicitly.
- **Working changes:** inspect `git diff HEAD`, `git diff --cached` and nonignored untracked files. The target is the final working snapshot relative to HEAD, including staged and unstaged edits; staged-only changes subsequently undone on disk are not part of that snapshot.
- **Unspecified:** use working changes if present; otherwise current branch against its merge-base with the default branch. If empty, review `HEAD` as one commit and disclose the fallback.

Inventory paths with `git diff --name-status --no-renames <base> <head>` so both sides of renames and deletions are considered. Read the diff to route checks, but perform the checklist-based source review after attempting the automatic checks.

## 2. Run Automatic Checks First

Use Python 3.9+; examples from the repository root:

```bash
python3 .agents/skills/code-review/scripts/static_checks.py --commit HEAD
python3 .agents/skills/code-review/scripts/static_checks.py --base origin/master --head HEAD
python3 .agents/skills/code-review/scripts/static_checks.py --worktree
```

`--base` uses its merge-base with `--head` (default HEAD). The helper prints resolved SHAs/scope, selects checks by changed paths, and continues other applicable checks after a failure:

- Every diff: whitespace and redacted heuristic scanning of added lines/sensitive filenames. This is neither a complete secret scanner nor a dependency vulnerability audit.
- `backend/`: nonmutating `gofmt -l`, `go vet -mod=readonly ./...`, API compilation into a temporary directory. Go must satisfy `backend/go.mod`.
- `frontend/`: installed ESLint, Next.js route type generation and TypeScript without emitting JS/incremental cache. Node must satisfy the frontend guide; installed direct dependencies must match the reviewed lockfile. Type generation can update ignored `.next/` and `next-env.d.ts`; report unexpected tracked changes.
- Shared runtime configuration (Compose, root package/lockfiles, launcher, CI) selects both projects. Pure documentation/skills skip language checks; review their tooling/instructions instead.

Committed targets require HEAD to equal the reviewed head and a clean tracked/nonignored untracked tree. Otherwise use an isolated checkout of that SHA or report checks blocked. Never reset/stash the user's files or claim checks on a different tree validate this target. Working-change checks use files on disk. Recheck status afterward for concurrent edits.

Exit `0`: applicable checks passed; `1`: failures; `2`: incomplete/blocked (missing prerequisites, invalid target, timeout). Read the per-check summary: failures can coexist with blocked checks. Record failures/limitations and continue source review; passing tools do not replace the checklist.

The helper does not run tests, start services/containers, access a DB or install tools. Go may need its module cache/proxy. Use installed tools on PATH; do not silently substitute unsupported versions.

Language-tool diagnostics are suppressed when the diff contains possible secrets or the diagnostics match secret heuristics; check statuses and exit codes remain visible. Generated `backend/docs/*.go` files select Go checks because the API imports them. To verify the helper itself, run `python3 -B -m unittest discover -s .agents/skills/code-review/scripts -p 'test_*.py'` from the repository root.

## 3. Review Every Applicable Checklist Item

Read [references/review-checklist.md](references/review-checklist.md) on **every invocation**. Apply common items to all diffs and conditional sections to affected files and dependencies. Record every ID as `checked`, `finding`, `not verified` or `not applicable`, with a short reason; group IDs only when status/applicability agree. Missing execution evidence is never a passed check.

### Current Product Architecture

The product is a modular Go monolith plus a separately run Next.js App Router frontend. Verify this map against current guides/code; a deliberate, documented architectural change is reviewable rather than automatically wrong.

| Responsibility | Location and boundary |
| --- | --- |
| Startup/wiring | `backend/cmd/api/main.go`: configuration, PostgreSQL, embedded Goose migrations, constructors, HTTP lifecycle. |
| HTTP assembly | `backend/internal/httpapi/`: routes/methods, CORS, content types, IP/user limits; no feature logic or GORM queries. |
| Feature handlers/services | `internal/auth`, `monitor`, `health`, `upload`: feature-local handlers, domain rules in services. Modules do not import `httpapi`; `health` intentionally queries SQL readiness directly. |
| Persistence | `internal/store/`: GORM models, owner filtering, locks, transactions; additive Goose SQL in `store/migrations/`, never `AutoMigrate`. |
| Shared helpers/storage | `internal/httpx/`: JSON/errors/identity context with no feature dependencies; `upload/storage.go`: generic storage independent of auth. Auth owns avatar validation. |
| Routes/UI | `frontend/src/app/`: routes/layouts; `src/components/auth/` and `monitors/`: UI/providers/hooks. Browser state stays behind client boundaries. |
| Requests/session | `src/lib/api.ts`: shared Axios; `src/lib/auth/`: auth/session; `monitors.ts`/`uploads.ts`: feature requests. Protected calls use `authorizedAPIRequest`. |

Current contracts: access JWT in browser memory only, 15-minute lifetime; HttpOnly refresh cookie, fixed 24-hour session. Preserve refresh serialization, one 401 retry, generation checks and account-keyed UI. Monitor creation/listing exists; polling does not. These are project contracts, not universal Go/Next.js rules.

Trace changed behavior through callers and old/new implementations before claiming a regression. Separate new defects from preexisting observations. Do not invent dependency advisories, architecture requirements or coverage thresholds.

## 4. Return Prioritized Findings

Lead with actionable findings, ordered by severity. Use P0–P3 as the bounded P0–Px scale:

| Priority | Meaning |
| --- | --- |
| P0 | Immediate blocker: demonstrated broadly exploitable breach, exposed active production credential, catastrophic data loss or widespread critical-flow outage. Evidence is required. |
| P1 | High: concrete authorization/session-isolation failure, likely data corruption, or broken critical behavior in realistic conditions. Fix before release. |
| P2 | Normal: reproducible localized functional issue, material performance regression, or meaningful missing regression coverage for changed risky behavior. |
| P3 | Low: concrete maintainability, naming, dead-code, documentation or accessibility issue with limited impact. Avoid subjective preferences and duplicate linter findings. |

Security impact determines severity; the word “security” or a failed tool alone does not make a P0/P1. Deduplicate by root cause. For each finding include `[P#] actionable title`, the smallest relevant **changed** file/line location, trigger, consequence, supporting code/test evidence and a focused correction. Cite callers as needed; redact secrets. Put unresolved questions and preexisting issues separately, not among confirmed regressions.

Then include scope (SHAs/snapshot and base choice), commands with pass/fail/blocked/skipped results, and a compact checklist coverage table containing every ID. Separate static checks, automated tests, DB/HTTP and browser verification. Test source and commit messages do not prove a run. If no actionable defects were found, say so and still give limitations/coverage. Apply fixes only when separately requested.
