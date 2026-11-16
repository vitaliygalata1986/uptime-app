# GitHub Flow

Apply this workflow only when starting a new, separate task, and follow it through that task's completion. Follow-up requests, clarifications, fixes, and PR feedback for the current task continue in its existing branch and PR; do not restart the workflow or create another task branch.

## Branch Workflow

- When starting a new task, before making changes, create a separate branch from the up-to-date default branch (`master` in this repository).
- Name task branches `feat/<nazvanie-zadachi>` for new features and other planned changes, or `fix/<nazvanie-zadachi>` for bug fixes. Write the task name in lowercase Latin transliteration with hyphens between words, for example `feat/redaktirovanie-profilya` or `fix/sohranenie-imeni`.
- Keep each branch focused on one task. Do not commit task changes directly to the default branch.
- After completing the task and passing the relevant checks, push the branch and open a pull request into the default branch. Merge completed work through the pull request after review and successful required checks.
