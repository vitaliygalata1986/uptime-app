# GitHub Flow

Apply this workflow only when starting a new, separate task, and follow it through that task's completion. Follow-up requests, clarifications, fixes, and PR feedback for the current task continue in its existing branch and PR; do not restart the workflow or create another task branch.

## Commit & Pull Request Guidelines

- When starting a new task, before making changes, create a separate branch from the up-to-date default branch (`master` in this repository).
- Name task branches `feat/<nazvanie-zadachi>` for new features and other planned changes, or `fix/<nazvanie-zadachi>` for bug fixes. Write the task name in lowercase Latin transliteration with hyphens between words, for example `feat/redaktirovanie-profilya` or `fix/sohranenie-imeni`.
- Keep each branch focused on one task. Do not commit task changes directly to the default branch.
- After completing the task and passing the relevant checks, push the branch and open a pull request into the default branch. Merge completed work through the pull request after review and successful required checks.

### Pull Request Workflow

- When asked to open a PR for an existing task branch, keep that branch and include all of its changes relative to the target branch. Read the complete diff and commit history before writing the title and description.
- Fetch `origin` and verify the default branch on GitHub before choosing the PR base; do not assume it is named `main`. Use an explicitly requested base when it exists; if it does not exist, clarify the target before opening the PR. Check for an existing PR for the same head/base pair and update it instead of creating a duplicate.
- Run the applicable checks from the frontend/backend guides and `git diff --check`. Review the staged paths, exclude secrets and unrelated files, commit intended changes, then push the task branch with an upstream. Do not force-push or merge as part of a request only to open a PR.
- Use a Conventional Commits title, `<type>[optional scope]: <imperative summary>`, at most 72 characters, describing the complete change against the base branch.
- Write a detailed description covering the problem and resulting behavior, frontend/backend changes, API contracts and migrations, relevant security behavior, and known limitations. Scale the sections to the change and link relevant issues when available.
- List the exact validation commands and their results. Distinguish static checks, automated integration tests, and browser/runtime verification; label earlier verification with its date and report skipped checks explicitly. Include repository screenshots for visible UI changes, using links that resolve from the PR.
- Create the PR with explicit head and base branches. With GitHub CLI, use `gh pr create --base <base> --head <branch> --title '<title>' --body-file <file>`; keep multiline Markdown in a UTF-8 file so newlines and code examples are preserved.
- After creating or updating the PR, verify its URL, title, head/base branches and published description, check available CI/review status, and report the link and any pending checks. Leave merging for a separate instruction after review and successful required checks.
- When asked to process PR feedback, read conversation comments, submitted reviews and inline discussions. Implement applicable changes on the existing head branch, rerun affected checks, push a focused commit and update the PR description and validation evidence. Report which feedback was addressed and leave the PR open for review unless merging was explicitly requested.
