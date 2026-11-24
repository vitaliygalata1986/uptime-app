---
name: standup
description: Summarize repository changes for a standup, using yesterday by default or the period requested by the user. Use when the user asks for a standup or a development summary for a specific date or time range.
---

# How to Prepare a Standup Summary

1. Choose the reporting period from the user's request. If the user invokes `$standup` without specifying a period, use yesterday: the previous calendar day from 00:00 inclusive to today at 00:00 exclusive. If the user specifies a date or time range (for example, today, a particular date, or the past week), use that period instead. Resolve dates in the user's timezone, falling back to the system timezone, and state the reporting period and timezone in the summary.
2. Review the commit history and corresponding diffs for the selected interval on the current branch. Use a different branch or author filter when the user requests one. Exclude uncommitted changes from the report.
3. Read the actual changes, not just commit messages. Group related commits into understandable features, fixes, or maintenance tasks. Avoid counting the same changes twice through merge commits.
4. Present a concise bullet list in the user's language. Explain what changed and why it matters for each item; add short commit hashes when useful for traceability.
5. Base every claim on the inspected history and code. Do not invent completed tests, deployments, blockers, or future plans. If no commits match the interval, say so explicitly.

This skill produces a read-only report; it does not change files, create commits, push branches, or post the summary elsewhere.
