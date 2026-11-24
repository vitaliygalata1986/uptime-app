---
name: swaggo-openapi
description: Maintain this project's backend OpenAPI contract through swaggo annotations. Use whenever backend API routes are added, changed or removed, including changes to request or response DTOs, validation and authentication, or when asked to synchronize API documentation.
---

# Maintain the Backend OpenAPI Contract

Keep every backend API operation documented as part of the API change. This project generates Swagger/OpenAPI 2.0 with swaggo; preserve that format and the pinned tools.

## Inspect the Contract Sources

Read [backend/AGENTS.md](../../../backend/AGENTS.md), especially API Documentation and Development Commands. Paths and commands below are relative to the repository root unless a working directory is stated.

Compare the task's diff with the current route table and prefix dispatch in `backend/internal/httpapi/api.go`. Trace each operation through its module handler, request/response DTOs and shared HTTP guards. Inventory all API path/method pairs, including file downloads and health checks; Swagger UI assets and automatic CORS preflight are not separate API operations. Check existing routes as well as the changed ones for missing documentation. Limit implementation edits to the requested API work and its documentation.

## Maintain Annotations

Add or update swaggo comments beside the module handler responsible for each operation. A shared dispatcher can have separate annotated methods, as in `backend/internal/monitor/handler.go`; do not move controllers into `httpapi` to document them.

- Match `@Router` paths and HTTP methods to registration, using `{file}`-style path parameters for prefix routes. For a removed or renamed operation, remove its old annotation without removing annotations for methods that still exist.
- Describe the operation with `@Summary`, `@Tags` and, when needed, `@Description`. Match `@Accept` and `@Produce` to actual content types, including multipart uploads and binary downloads.
- Use `@Param` for actual path, query, header, body or form-data inputs, including requiredness, limits and validation. Keep DTO schema tags consistent with runtime validation and JSON serialization. Do not invent fields or expose internal database models and secrets.
- Use `@Success` and `@Failure` for actual status codes and public response types, including errors produced by shared guards. Preserve bodyless responses such as `204` and file responses instead of documenting every result as JSON.
- Apply `@Security BearerAuth` only where an access JWT is required. Describe refresh-cookie behavior and required Origin headers where applicable; refresh tokens remain HttpOnly cookies, never JSON fields. Inspect current guards rather than assuming every operation has the same requirements.

Global API metadata and the Bearer security definition live in `backend/cmd/api/main.go`; change them only when the API change requires it. Follow neighboring annotations and public DTOs for syntax.

## Regenerate Artifacts

After synchronizing source annotations and DTOs with the routes, run [scripts/sync-docs.sh](scripts/sync-docs.sh) from the repository root:

```bash
bash .agents/skills/swaggo-openapi/scripts/sync-docs.sh
```

The script resolves the repository from its own location, so it also works by absolute path from another working directory. It runs `go generate .` from `backend/`, then immediately runs `npm --prefix .. run docs:redoc` only if contract generation succeeds. Any failed step stops the script with a nonzero exit code. It requires the Go and Node.js/npm versions specified in `backend/AGENTS.md` on `PATH`.

`backend/generate.go` pins swaggo; the root `docs:redoc` script pins Redocly. The helper reuses both commands without duplicating generator configuration. Do not substitute an unpinned global generator or manually edit generated files. Include all four outputs with the API change:

- `backend/docs/docs.go`
- `backend/docs/swagger.json`
- `backend/docs/swagger.yaml`
- `backend/docs/redoc.html`

If generation fails, resolve the cause within the authorized scope or report the exact blocker and incomplete artifacts. Do not claim the contract is synchronized after a failed step.

## Verify Coverage and Behavior

Compare the regenerated path/method inventory with the registered API: every current operation is present, deleted operations are absent, and schemas, content types, responses and security reflect the implementation. Check shared DTO changes across every operation that uses them.

Inspect `backend/internal/httpapi/swagger_test.go`. Its contract test compares the served specification, checked-in JSON and registered routes, including explicit prefix routes and protected-operation expectations. Update those expectations when a route change requires it; preserve coverage rather than weakening assertions to make tests pass.

Run the backend checks required by `backend/AGENTS.md`. For a focused contract check, run `go test ./internal/httpapi -run '^TestSwagger' -count=1` from `backend/`; this does not replace the required build, vet and race tests for backend code changes. Use the isolated test database workflow for integration checks and report skipped database tests explicitly.

Review the source and generated diffs, then run `git diff --check` from the repository root. Report which operations changed, which artifacts were regenerated and which checks actually passed. Distinguish static checks and handler tests from live HTTP or browser verification.
