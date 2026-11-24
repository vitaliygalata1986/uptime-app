---
name: commit
description: Create Git commits following this project's conventions. Use when the user asks to create a commit.
---

# How to Commit

1. Review the diff (`git diff` and `git diff --cached`).
2. Group changes into logical steps, with one logical step per commit. Stage only the changes intended for each commit.
3. Use Conventional Commits: `feat`, `fix`, `refactor`, `test`, `docs`, or `chore`.
4. Keep the subject at most 72 characters and use the imperative mood. Do not use emoji or filler such as "significantly improved".
5. Include a body only when needed to explain why, not what.
6. Run the relevant project checks and `git diff --check`, then create the commits.
