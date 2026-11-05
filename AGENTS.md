# Repository Guidelines

## Project Organization

This repository contains two independently run projects:

- `frontend/`: the web application. See [frontend/AGENTS.md](frontend/AGENTS.md) for its structure, commands, and conventions.
- `backend/`: the backend application. See [backend/AGENTS.md](backend/AGENTS.md) for its structure, commands, and conventions.

Read the applicable directory guide before changing either project. Run commands from the directory specified by that guide.

## Shared Development Guidelines

Keep changes focused and match the surrounding code style. Backend authentication, PostgreSQL persistence through GORM, and Goose migrations are implemented. Frontend registration, login, session restoration and the account dashboard use the Go API. Monitoring is not implemented. Run both projects with `npm run dev` (Node.js 22.13+ or 24+); the launcher waits for readiness and uses the published API port. Ctrl+C stops frontend; `npm run dev:stop` stops backend without deleting data.

Backend HTTP controllers belong to their feature modules (`backend/internal/auth/handler.go`, `backend/internal/health/handler.go`). Keep `backend/internal/httpapi/` focused on route assembly and shared HTTP guards; see `backend/AGENTS.md` for dependency and test placement rules.

Run the backend stack from the repository root with `docker compose --env-file backend/.env up --build -d`. Configure `.env` using `backend/.env.example`; never commit secrets. The current local `backend/.env` sets `API_PORT=8081`, so use `http://localhost:8081` and `http://localhost:8081/healthz`. Compose and `.env.example` default to host port 8080; the Go server inside the container always listens on 8080 (`HTTP_ADDR=:8080`). Check the local `API_PORT` before reporting the running address. Port 18080 in `docs/auth-verification.md` belongs to the historical 2026-10-30 verification. PostgreSQL has no published host port and persists in a named volume. Use the independent `compose.test.yaml` project for database integration tests; do not point tests at application data. See `backend/README.md` for commands from `backend/`.

Validate changes using the affected project's checks. For changes spanning both projects, validate both sides and their interaction. Report which checks ran and distinguish static checks from runtime verification. Run `git diff --check` before submitting changes.

## Commit & Pull Request Guidelines

Recent history uses prefixes such as `feat:` and `chore:`. Write concise, imperative subjects and keep commits focused. Pull requests should describe the change, list validation performed, link relevant issues, and include screenshots for visible UI changes.
