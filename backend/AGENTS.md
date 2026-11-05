# Backend Guidelines

## Project Structure

This is the Go module `uptime-app/backend`. The entry point is `cmd/api/main.go`; place future private application packages in `internal/`.

The entry point validates configuration, opens PostgreSQL, applies embedded Goose SQL migrations, then starts `net/http`. `internal/config/` owns settings, `internal/store/` owns GORM models and transactions, `internal/auth/` owns passwords/JWT/sessions, and `internal/httpapi/` owns routes, cookie, CORS and IP limits. Monitoring and frontend integration are not implemented.

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
