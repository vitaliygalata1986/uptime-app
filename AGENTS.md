# Repository Guidelines

## Project Organization

This repository contains two independently run projects:

- `frontend/`: the web application. See [frontend/AGENTS.md](frontend/AGENTS.md) for its structure, commands, and conventions.
- `backend/`: the backend application. See [backend/AGENTS.md](backend/AGENTS.md) for its structure, commands, and conventions.

Read the applicable directory guide before changing either project. Run commands from the directory specified by that guide.

## Shared Development Guidelines

Keep changes focused and match the surrounding code style. Backend authentication, PostgreSQL persistence through GORM, and Goose migrations are implemented. Frontend registration, login, session restoration and the account dashboard use the Go API. Monitor creation and per-user listing are implemented; site polling is not implemented. Run both projects with `npm run dev` (Node.js 22.13+ or 24+); the launcher waits for readiness and uses the published API port. Ctrl+C stops frontend; `npm run dev:stop` stops backend without deleting data.

Validate changes using the affected project's checks. For changes spanning both projects, validate both sides and their interaction. Report which checks ran and distinguish static checks from runtime verification. Run `git diff --check` before submitting changes.

## Code Comments

- Explain why, rather than what the code does when that is already clear.
- Do not comment on obvious code; prefer clear, accurate names.
- Add a doc comment to every public function describing its contract.
- Explain complex arithmetic next to the calculation.
- Keep comments up to date whenever the related code changes.

## API Documentation

When adding, changing or removing backend API routes or their contracts, use the [swaggo-openapi skill](.codex/skills/swaggo-openapi/SKILL.md) to maintain swaggo annotations and regenerate the contract according to [backend/AGENTS.md](backend/AGENTS.md#api-documentation).

## When starting a new task use GitHub Flow

Apply these rules only when starting a new task. Continue the current task in its existing branch and PR without restarting the workflow.

Details in file: [docs/rules/github-flow.md](docs/rules/github-flow.md).

## When committing

Use the [commit skill](.codex/skills/commit/SKILL.md).

## When creating Pull Request Workflow

Use rules in file: [docs/rules/pr.md](docs/rules/pr.md).
