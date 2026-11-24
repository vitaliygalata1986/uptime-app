# Review Checklist

Read every time. Evaluate IDs against the diff and affected call chain. Good/bad signs are examples, not demands to rewrite unchanged code. Record evidence for findings and reasons for conditional sections not applicable.

## Common: Every Diff

| ID | Check | Good signs | Bad signs |
| --- | --- | --- | --- |
| C1 | Scope/correctness | Actual named diff, related callers, renames/deletions/untracked files inspected; behavior matches requirements. | Reviewing messages alone or checking a different revision. |
| C2 | Secrets/data | Config references environment; explicit fake examples; public DTOs and safe logs. | Real keys, credential URLs, `.env`, private keys, password/token/cookie logs; secrets in `NEXT_PUBLIC_*` or client bundles. Manually inspect literals even after a passing scan; redact values. |
| C3 | Naming/contracts | Names express domain and units; public functions document contracts; arithmetic explains limits/units; comments match behavior. | Ambiguous units/names, stale comments, undocumented public behavior; broad renaming with no value. Match surrounding Go/TS conventions. |
| C4 | Reachability/simplicity | New code has callers; superseded paths/config/imports removed; errors keep useful context. | Dead branches/helpers, swallowed errors, duplicated rules drifting apart. Search framework conventions/reflection before declaring code dead. |
| C5 | Changed-risk tests | Assertions cover new behavior, negative/boundary inputs and a regression that fails before the fix. | Happy-path-only ownership tests, unasserted mocks, implementation-mirroring tests, weakened assertions. No project coverage percentage is required. |
| C6 | Evidence | Static, unit, DB and browser results separated; failures/skips visible. | Lint presented as proof of persistence/security; skipped tests called passed; heuristic match called a proven leak. |

## Go Backend: Backend Code/Config or Shared Runtime Changes

| ID | Check | Good signs | Bad signs |
| --- | --- | --- | --- |
| B1 | Layers | Wiring in `cmd/api`; feature-local handlers/services; GORM in `store`; shared guards in `httpapi`; independent `httpx` and generic uploads. | ORM in handlers, modules importing router, feature/cookie logic moved into routing. SQL readiness in `health` is intentional. |
| B2 | Go/resource lifetime | Errors checked, wrapping respected with `errors.Is`; contexts reach DB/IO; files/timers close; goroutines terminate; shared state synchronized. | Dropped IO/commit errors, data races, goroutine leaks, background contexts hiding cancellation, panic for user input. |
| B3 | HTTP | Route/method table, Allow/preflight/CORS agree; strict JSON single object ≤8 KiB, no unknown fields; public DTOs/error envelopes. | Guards bypassed by prefix routes, wildcard credentialed CORS, spoofable proxy headers trusted, body with 204, leaked models/internal errors. Check guard order per route. |
| B4 | Ownership | Validated JWT/context supplies owner; reads/writes always scope by owner; per-user limits after auth. | Owner from body/query/cursor without authorization, missing DB filters, frontend guard treated as API authorization. |
| B5 | Input/bounds | Email normalized; password unchanged, 8–128 Unicode code points; trimmed name 1–100 without controls; HTTP(S) monitor URL, positive INTEGER seconds. | Byte/rune mismatch, negative/overflowed interval, URL credentials/fragments, unbounded decoding. Stored URLs are not fetched today; assess SSRF if server-side fetching is introduced. |
| B6 | Tests/API docs | Relevant handler/service/guard tests; swaggo matches routes, DTOs, auth, errors and binary responses; all four artifacts updated. | API contract changed without `docs.go`, JSON, YAML and ReDoc synchronization, internal secrets in docs. |

For backend code the guide requires build, vet and `go test -race ./...`; the helper covers build/vet, not tests. Run race tests when possible. DB tests need an explicitly isolated `TEST_DATABASE_URL` or the separate `compose.test.yaml` project; read the backend guide first. Never use application data. Record skipped DB tests; do not start containers/migrate a live DB merely to finish a review.

## Auth: Auth, Protected Calls or Account-Bound State

| ID | Check | Good signs | Bad signs |
| --- | --- | --- | --- |
| A1 | JWT/passwords/cookies | JWT algorithm/claims checked; Argon2id/secure randomness retained; refresh hashes stored; HttpOnly/path/SameSite/production Secure preserved. | Algorithm confusion, predictable tokens, plaintext credentials, refresh in JSON/readable storage, lost Origin protection on cookie-changing requests. |
| A2 | Refresh transaction/expiry | Lock session, re-read token; commit reuse revocation before 401; fixed 24h session and 15min access. | Revocation rolled back by transaction callback error; two concurrent refreshes accepted; rotation extends session. Issued JWT remaining valid until expiry after logout is intentional. |
| A3 | Browser concurrency | Access only in memory; Web Locks serialize cookie changes; single-flight refresh, one 401 retry; BroadcastChannel events only. | Token in web storage/server globals/messages, recursive retries, duplicate cookie rotation, obsolete caller Authorization header. |
| A4 | Account isolation | Generation checked on success/retry; account-keyed lists reset immediately on switch/logout; delayed response/cross-tab regression coverage. | Prior account response overwrites UI; retry skips generation check; stale `/me` undoes profile save; shared cache leaks another user's records. |

## Persistence: Store, SQL or Persistence-Affecting Services

| ID | Check | Good signs | Bad signs |
| --- | --- | --- | --- |
| D1 | Atomicity/queries | Owner lock makes quota count+insert atomic (100 monitors/account); parameterized SQL; constraints/indexes support queries; rollback/commit intent correct. | Count/insert race, user SQL concatenation, partial multi-step update, ignored transaction errors, new N+1/unbounded hot-path queries. |
| D2 | Migrations | New additive Goose migration; existing rows and embedded ordering considered; recovery follows `docs/guides/migrations.md`; no AutoMigrate. | Editing applied migrations, destructive change without recovery, incompatible existing rows, schema change hidden in startup. |
| D3 | Pagination/persistence | Owner-scoped keyset `(created_at,id)`, 20 rows and stable next_cursor; timestamp ties/end tested; save agrees with re-read. | Unscoped cursor, missing tie-breaker, offset shifts, success despite stale stored state. Without DB execution this is source evidence only. |

## Files: Upload, Avatar, Download or File URLs

| ID | Check | Good signs | Bad signs |
| --- | --- | --- | --- |
| U1 | Validation/storage | Avatar JPEG/PNG content/dimensions checked (5 MiB, 4096/side, 16 MP); generic upload ≤20 MiB; random exclusive server filenames and strict paths. | Image extension/MIME trusted alone, client names used as paths, traversal/symlink escape, unbounded decoding, partial orphan files. |
| U2 | Save/serving | Avatar+name persist together; failed DB save removes new file; old removed after success; arbitrary downloads use attachment/sandbox/nosniff. | Old avatar deleted before commit, stored XSS via inline arbitrary files, private-file promises for intentionally public URLs, leaked local paths. |

## Next.js: Frontend Code/Config or Shared Runtime Changes

| ID | Check | Good signs | Bad signs |
| --- | --- | --- | --- |
| F1 | App Router/boundaries | Routes/layouts in app, browser APIs behind client boundaries; installed Next docs consulted for version-sensitive APIs; Turbopack root retained. | Browser globals during SSR, client secrets, stale framework assumptions, user sessions cached in server components. |
| F2 | Requests/state | Feature UI/hooks own state; lib owns typed requests; shared Axios credentials/timeout/dispatch-time Bearer; objects/FormData as data. | Duplicate transport/session logic, stale manual headers, manual multipart boundary, invented response types. |
| F3 | React/types | Correct hook dependencies/cleanup; obsolete view responses rejected; stable keys; narrow types/error handling. | Conditional hooks, races, stale closures, unstable keys, assertions/any hiding contracts, redundant derived state. Memoization needs a reason. |
| F4 | UI/accessibility | Loading/error/empty/retry states; labels, focus/keyboard/mobile behavior; duplicate submit prevented; blob previews revoked. | Silent failures, success before save, unlabeled controls, inaccessible errors, memory leaks. Static checks do not prove browser behavior. |
| F5 | Product/tests | Account-keyed monitor state, cursor deduplication; integer unit conversion within bounds; existing Node session tests preserved. | Account bleed, duplicate pages, fractional/overflowed seconds, fabricated uptime/latency. Polling is not implemented. Do not require a new test framework by default. |

Frontend code checks from its guide: `npm test`, lint, production build and affected-page browser inspection. Helper typegen/tsc does not replace production build. Node transport tests exercise real modules with controlled Axios adapters, not browser/API/DB. Report unavailable runs.

## Shared Configuration, Dependencies, Documentation and Tooling

| ID | Check | Good signs | Bad signs |
| --- | --- | --- | --- |
| X1 | Runtime/dependencies | Shared runtime changes checked across both projects; lockfiles agree; ports from current config; persistence/nonroot behavior preserved. | Deleted volumes, exposed application DB, hard-coded historical ports, accidental dependency drift, unsupported security claims. Verify advisory claims with current authoritative sources. |
| X2 | Docs/helpers | Instructions agree with execution; documented working directories; deleted/untracked files and failed/missing tools handled; secret values redacted. | Destructive review automation, hidden skips, scanner pass called proof of safety, hand-edited generated artifacts, unrelated changes. |

For pure docs/skills, language checks usually do not apply. Validate scripts with realistic temporary fixtures including failure/blocked paths; validate skills with the skill validator. Shared runtime changes select both language groups even without `.go`/`.tsx` changes.
