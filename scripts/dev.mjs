#!/usr/bin/env node
import { spawn, spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
const root = fileURLToPath(new URL("../", import.meta.url));
const [major, minor] = process.versions.node.split(".").map(Number);
if (!((major === 22 && minor >= 13) || major >= 24)) {
  console.error("Нужен Node.js 22.13+ или 24+. Текущий: " + process.version);
  process.exit(1);
}
const compose = ["compose", "--env-file", "backend/.env"];
if (!existsSync(new URL("../backend/.env", import.meta.url))) {
  console.error("Создайте backend/.env по инструкции backend/README.md."); process.exit(1);
}
if (process.argv.includes("--stop")) {
  const result = spawnSync("docker", [...compose, "stop"], { cwd: root, stdio: "inherit" });
  process.exit(result.status ?? 1);
}
if (!existsSync(new URL("../frontend/node_modules/next", import.meta.url))) {
  console.error("Установите зависимости: npm --prefix frontend ci"); process.exit(1);
}
const result = spawnSync("docker", [...compose, "config", "--format", "json"], { cwd: root, encoding: "utf8" });
if (result.status !== 0) { console.error(result.stderr || "Docker Compose недоступен."); process.exit(1); }
const config = JSON.parse(result.stdout);
const port = config.services.api.ports.find((item) => item.target === 8080)?.published;
const frontendPort = process.env.FRONTEND_PORT || "3000";
const origin = `http://localhost:${frontendPort}`;
if (!String(config.services.api.environment.ALLOWED_ORIGINS).split(",").map((item) => item.trim()).includes(origin)) {
  console.error(`Добавьте ${origin} в ALLOWED_ORIGINS в backend/.env.`); process.exit(1);
}
if (String(config.services.api.environment.LOCAL_HTTP) !== "true") {
  console.error("Для локального запуска задайте LOCAL_HTTP=true в backend/.env."); process.exit(1);
}
let child;
let stopping = false;
function stop(signal) {
  stopping = true;
  if (child?.pid) { try { process.kill(-child.pid, signal); } catch { /* уже завершился */ } }
}
process.on("SIGINT", () => stop("SIGINT"));
process.on("SIGTERM", () => stop("SIGTERM"));
function run(command, args, options = {}) {
  return new Promise((resolve, reject) => {
    child = spawn(command, args, { cwd: root, stdio: "inherit", detached: true, ...options });
    child.on("error", reject);
    child.on("exit", (code) => resolve(code ?? (stopping ? 0 : 1)));
  });
}
try {
  console.log("Запускаем PostgreSQL и backend; ожидаем готовности…");
  const code = await run("docker", [...compose, "up", "--build", "-d", "--wait"]);
  if (stopping || code !== 0) process.exit(code);
  const apiURL = process.env.NEXT_PUBLIC_API_URL || `http://localhost:${port}`;
  let ready = false;
  for (let attempt = 0; attempt < 30 && !stopping; attempt++) {
    try { ready = (await fetch(`${apiURL}/healthz`, { signal: AbortSignal.timeout(1500) })).ok; } catch { /* ждём запуска */ }
    if (ready) break;
    await new Promise((resolve) => setTimeout(resolve, 1000));
  }
  if (stopping) process.exit(0);
  if (!ready) throw new Error("Backend не прошёл /healthz. Проверьте docker compose logs api.");
  console.log(`Backend: ${apiURL}\nFrontend: ${origin}\nCtrl+C остановит frontend. Backend: npm run dev:stop`);
  process.exitCode = await run("npm", ["--prefix", "frontend", "run", "dev", "--", "--hostname", "localhost", "--port", frontendPort], { env: { ...process.env, NEXT_PUBLIC_API_URL: apiURL } });
} catch (error) { console.error(error.message); process.exitCode = 1; }
