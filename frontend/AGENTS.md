# Frontend Guidelines

## Project Structure

This project uses Next.js 16 App Router, React, and strict TypeScript. Routes and layouts live in `src/app/`; `@/*` resolves to `src/*`. `/login`, `/register` and `/dashboard` provide authentication. `/dashboard/profile` displays the current user and edits only their name via `PATCH /api/auth/profile`; it uses the dashboard layout's auth guard. `src/components/auth/` owns UI and the provider; `src/lib/auth/` owns typed requests and the in-memory session. No dedicated test framework is configured.

## Development Commands

Run commands from `frontend/`. Use Node.js 22.13+ within the 22 release line, or Node.js 24+.

- `npm ci`: install dependencies from the lockfile.
- `npm run dev`: start development at `http://localhost:3000`.
- `npm run lint`: run ESLint with Next.js Core Web Vitals and TypeScript rules.
- `npm run build`: create a production build.
- `npm start`: serve the production build after building.

## Coding Style & Configuration

Use two-space indentation, double quotes, semicolons, and explicit types where useful. Use PascalCase for React component names and Next.js filenames such as `page.tsx` and `layout.tsx` for route conventions. No separate Prettier configuration exists.

Use `NEXT_PUBLIC_API_URL` and credentials with the existing Go API. Refresh stays in HttpOnly cookie, access only in memory: never web storage or readable cookies. Preserve Web Locks serialization, single-flight refresh and stale-response generation checks; BroadcastChannel carries only events. Client guards control UI; API authorization protects data. Never cache user sessions in server components.

Preserve the explicit Turbopack root in `next.config.ts` for this nested checkout.

## Testing

No test framework, test script, or coverage threshold is configured. Validate changes with lint and production build, then inspect affected pages in the browser.

<!-- BEGIN:nextjs-agent-rules -->

## This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->
