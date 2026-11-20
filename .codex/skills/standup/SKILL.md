---
name: standup
description: Summarize yesterday's repository changes for a standup. Use when the user asks for a standup or a summary of yesterday's development work.
---

# How to Prepare a Standup Summary

1. Treat yesterday as the previous calendar day in the user's timezone, falling back to the system timezone. Use the interval from yesterday at 00:00 inclusive to today at 00:00 exclusive, and state the date and timezone in the summary.
2. Review the commit history and corresponding diffs for that interval on the current branch. Use a different date, branch, or author filter when the user requests one. Exclude uncommitted changes from yesterday's work.
3. Read the actual changes, not just commit messages. Group related commits into understandable features, fixes, or maintenance tasks. Avoid counting the same changes twice through merge commits.
4. Present a concise bullet list in the user's language. Explain what changed and why it matters for each item; add short commit hashes when useful for traceability.
5. Base every claim on the inspected history and code. Do not invent completed tests, deployments, blockers, or future plans. If no commits match the interval, say so explicitly.

This skill produces a read-only report; it does not change files, create commits, push branches, or post the summary elsewhere.
