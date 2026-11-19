import { spawnSync } from "node:child_process";
import { readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../", import.meta.url));
const output = new URL("../backend/docs/redoc.html", import.meta.url);
const result = spawnSync("npx", [
  "--yes", "@redocly/cli@2.81.0", "build-docs", "backend/docs/swagger.json",
  "--output", "backend/docs/redoc.html", "--inlineBundle", "--title", "Uptime API",
], {
  cwd: root,
  stdio: "inherit",
  env: { ...process.env, REDOCLY_TELEMETRY: "off" },
});

if (result.error) console.error(result.error.message);
if (result.status !== 0) process.exit(result.status ?? 1);

// Redoc's bundled styles contain trailing spaces that fail the repository's diff check.
writeFileSync(output, readFileSync(output, "utf8").replace(/[\t ]+$/gm, ""));
