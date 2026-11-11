# Frontend Guidelines

## Project Structure

This project uses Next.js 16 App Router, React, and strict TypeScript. Routes and layouts live in `src/app/`; `@/*` resolves to `src/*`. `/login`, `/register` and `/dashboard` provide authentication. `/dashboard/profile` displays the current user and edits their name via `PATCH /api/auth/profile` and avatar via multipart `POST /api/auth/avatar` (optional name saved together); it uses the dashboard layout's auth guard. `src/components/auth/` owns UI and the provider; `src/lib/auth/` owns typed requests and the in-memory session. Avatars use the API origin with `next/image` unoptimized; a local blob preview is uploaded only on save. `src/lib/api.ts` provides shared HTTP transport and file URLs; `src/lib/uploads.ts` exports `uploadFile(file)` for any file type up to 20 MiB and returns URL/name/size/content type. It uses the same in-memory access token and refresh serialization through `authorizedAPIRequest`. No dedicated test framework is configured.

`src/components/monitors/` owns the dashboard's monitor list and creation form; `src/lib/monitors.ts` calls `GET/POST /api/monitors` through `authorizedAPIRequest`. Users enter a positive whole number and choose seconds, minutes or hours; the request stores the equivalent interval in seconds (1–2147483647). Points are loaded after authentication and created points appear immediately. Polling is not implemented; do not display fabricated availability or response time.

Keep monitor state keyed by the authenticated user ID. `CreateMonitorForm` owns creation inputs and errors; `useMonitors` owns loading and cursor pagination (20 records, `next_cursor`). A cross-tab sign-in hides the previous account immediately, and protected requests reject successful responses from an obsolete session generation, including 401 retries. Preserve these checks when adding account-bound features.

## Development Commands

Run commands from `frontend/`. Use Node.js 22.13+ within the 22 release line, or Node.js 24+.

- `npm ci`: install dependencies from the lockfile.
- `npm run dev`: start development at `http://localhost:3000`.
- `npm run lint`: run ESLint with Next.js Core Web Vitals and TypeScript rules.
- `npm test`: run Node.js regression tests for session isolation and stale responses.
- `npm run build`: create a production build.
- `npm start`: serve the production build after building.

## Coding Style & Configuration

Use two-space indentation, double quotes, semicolons, and explicit types where useful. Use PascalCase for React component names and Next.js filenames such as `page.tsx` and `layout.tsx` for route conventions. No separate Prettier configuration exists.

Use `NEXT_PUBLIC_API_URL` and credentials with the existing Go API. Refresh stays in HttpOnly cookie, access only in memory: never web storage or readable cookies. Preserve Web Locks serialization, single-flight refresh and stale-response generation checks; BroadcastChannel carries only events. Client guards control UI; API authorization protects data. Never cache user sessions in server components.

Preserve the explicit Turbopack root in `next.config.ts` for this nested checkout.

## Testing

Session regression tests use the built-in Node.js test runner in `tests/`, without another test framework or a coverage threshold. Validate changes with `npm test`, lint and production build, then inspect affected pages in the browser.

<!-- BEGIN:nextjs-agent-rules -->

## This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->
