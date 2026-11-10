# Backend Guidelines

## Project Structure

This is the Go module `uptime-app/backend`. The entry point is `cmd/api/main.go`; place future private application packages in `internal/`.

The entry point validates configuration, opens PostgreSQL, applies embedded Goose SQL migrations, wires module handlers, then starts `net/http`. `internal/config/` owns settings, `internal/store/` owns GORM models and transactions, `internal/auth/` owns passwords/JWT/sessions and auth HTTP handlers (including refresh cookies and auth error mapping), and `internal/health/` owns the database readiness handler. `internal/httpapi/` only assembles routes and applies shared HTTP guards, CORS and IP limits. `internal/httpx/` provides shared strict JSON decoding and response helpers, without module dependencies. The Next.js frontend calls auth endpoints with credentials and Bearer access tokens. Monitoring is not implemented.

Keep each module's controllers in its own `handler.go` alongside its service code; register them in `internal/httpapi/` and construct dependencies in `cmd/api/`. Do not put module request handling, service calls or database readiness queries back in `httpapi`, and do not import `httpapi` from modules. Keep cookie tests alongside the auth handler; route/guard tests and cross-module API integration tests stay in `httpapi`.

The auth module also owns the current user's profile: `GET /api/auth/me` returns the public user including `name`; `PATCH /api/auth/profile` accepts only `name` (trimmed, 1–100 Unicode characters, no controls) and derives the user ID from the access JWT. Schema changes use additive Goose migrations; existing accounts start with an empty name.

## Development Commands

Run commands from `backend/` with Go 1.27+; Docker builds use Go 1.27.3 without changing the system toolchain:

- `go run ./cmd/api`: start the API with exported configuration including `DATABASE_URL`; `.env` is read by Compose, not the binary.
- `go build -o bin/api ./cmd/api`: compile the backend binary.
- `go vet ./...`: run static analysis.
- `go test -race ./...`: run unit tests; integration tests skip explicitly unless `TEST_DATABASE_URL` is set to a separate test PostgreSQL.
- `docker compose -f ../compose.yaml --env-file .env up --build -d`: run API and persistent PostgreSQL 18. Prepare `.env` using README; never commit it.
- `docker compose -p uptime-auth-tests -f ../compose.test.yaml up --build --abort-on-container-exit --exit-code-from tests`: build, vet and run all tests against an isolated PostgreSQL in Docker.
- `docker compose -p uptime-auth-tests -f ../compose.test.yaml down`: remove the temporary test containers after checks.

The current local `.env` sets `API_PORT=8081`: the host API URL is `http://localhost:8081`, with readiness at `/healthz`. Compose and `.env.example` default to host port 8080; Compose sets the container's `HTTP_ADDR=:8080` independently of `API_PORT`. Check `.env` before choosing the host URL for HTTP commands. The 18080 address in the dated verification report is historical.

## Coding Style & Naming

Format Go code with `gofmt`. Use lowercase package names and PascalCase for exported identifiers. Keep executable startup in `cmd/api/` and application logic in focused packages under `internal/`.

Keep ORM calls in `internal/store/` and use public DTOs in responses. Change schema through new versioned SQL migrations, never `AutoMigrate`. Refresh rotation must lock the session, re-read the token and commit reuse revocation before returning `401`; returning that error from a GORM transaction callback would roll back revocation. Keep access TTL at 15 minutes and session expiry fixed at 24 hours from login. Do not trust proxy headers for rate limiting by default.

## Testing

Use Go's standard `testing` package. Add tests alongside the code as `*_test.go`, using `TestName` functions. No coverage threshold is configured. Run build, vet and race tests when changing backend code. Integration tests create independent schemas in a separate test DB; without that DB, report them as skipped. Verify HTTP through the published API port and persistence across container restarts separately. `/healthz` checks HTTP and database readiness. Run `git diff --check` from the repository root before submitting changes.
