import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import vm from "node:vm";
import ts from "typescript";
import axios from "axios";

function responseFor(config, data, status = 200) {
  const response = { data, status, statusText: "", headers: {}, config };
  if (status >= 400) throw new axios.AxiosError("Request failed", "ERR_BAD_REQUEST", config, null, response);
  return response;
}

function loadClient(adapter) {
  const modules = new Map();
  const channels = [];
  class Channel {
    constructor() { channels.push(this); }
    postMessage() {}
  }
  const transport = { ...axios, create: (config) => axios.create({ ...config, adapter }) };
  function load(url) {
    if (modules.has(url.href)) return modules.get(url.href);
    const exports = {};
    modules.set(url.href, exports);
    const compiled = ts.transpileModule(readFileSync(url, "utf8"), {
      compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022, esModuleInterop: true },
    }).outputText;
    vm.runInNewContext(compiled, {
      exports, FormData,
      process: { env: { NEXT_PUBLIC_API_URL: "http://localhost:8081" } },
      navigator: { locks: { request: async (_name, action) => action() } },
      window: { BroadcastChannel: Channel }, BroadcastChannel: Channel,
      require: (name) => name === "axios" ? transport : load(new URL(`${name}.ts`, url)),
    }, { filename: url.pathname });
    return exports;
  }
  return { load: (path) => load(new URL(`../src/lib/${path}.ts`, import.meta.url)), channels };
}

function deferred() {
  let resolve;
  const promise = new Promise((done) => { resolve = done; });
  return { promise, resolve };
}

async function sessionHarness(request, onRefresh) {
  const requests = [];
  let refreshes = 0;
  const user = { id: 1, email: "first@example.com", name: "", avatar_url: "", created_at: "2026-11-11T00:00:00Z" };
  const { load, channels } = loadClient(async (config) => {
    requests.push(config);
    if (config.url === "auth/refresh") {
      refreshes++;
      await onRefresh?.(refreshes);
      return responseFor(config, { user, access_token: `test-access-${refreshes}`, expires_in: 900 });
    }
    if (config.url === "auth/me") return responseFor(config, user);
    if (config.url === "auth/logout") return responseFor(config, "", 204);
    return responseFor(config, await request(config));
  });
  const session = load("auth/session");
  session.initialize();
  await session.restore();
  assert.equal(session.getSnapshot().status, "authenticated");
  return { session, channels, requests, load, refreshes: () => refreshes };
}

test("discard a successful response after logout changes the session", async () => {
  const response = deferred();
  const { session } = await sessionHarness(() => response.promise);
  const request = session.authorizedAPIRequest("monitors");
  const rejected = assert.rejects(request, (error) => error.code === "session_changed");
  await session.logout();
  response.resolve({ monitors: [{ id: 1 }] });
  await rejected;
  assert.equal(session.getSnapshot().status, "anonymous");
});

test("discard a successful 401 retry if the session changed while waiting", async () => {
  const retried = deferred();
  const response = deferred();
  let attempts = 0;
  const { session } = await sessionHarness((config) => {
    if (++attempts === 1) return responseFor(config, { error: { code: "unauthorized" } }, 401);
    retried.resolve();
    return response.promise;
  });
  const request = session.authorizedAPIRequest("monitors");
  const rejected = assert.rejects(request, (error) => error.code === "session_changed");
  await retried.promise;
  await session.logout();
  response.resolve({ monitors: [{ id: 1 }] });
  await rejected;
  assert.equal(attempts, 2);
});

test("a sign-in event hides the previous account before restoring the session", async () => {
  const { session, channels } = await sessionHarness(() => Promise.resolve({}));
  channels[0].onmessage({ data: "signed-in" });
  assert.equal(session.getSnapshot().status, "checking");
  assert.equal(session.getSnapshot().user, null);
  await session.restore();
  assert.equal(session.getSnapshot().status, "authenticated");
});

test("Axios adds authorization centrally to JSON and multipart requests, keeping public requests separate", async () => {
  const { session, requests, load } = await sessionHarness(() => ({ id: 1 }));
  await load("monitors").createMonitor("https://example.com", 600);
  await load("uploads").uploadFile(new File(["content"], "sample.txt"));
  const protectedRequests = requests.filter((config) => config.url !== "auth/refresh");
  for (const config of protectedRequests) {
    assert.equal(config.headers.get("Authorization"), "Bearer test-access-1");
    assert.equal(config.withCredentials, true);
    assert.equal(config.timeout, 15000);
    assert.equal(config.baseURL, "http://localhost:8081/api/");
  }
  const json = requests.find((config) => config.url === "monitors");
  assert.equal(json.headers.get("Content-Type"), "application/json");
  assert.deepEqual(JSON.parse(json.data), { url: "https://example.com", interval_seconds: 600 });
  const upload = requests.find((config) => config.url === "uploads");
  assert.ok(upload.data instanceof FormData);
  assert.equal(upload.data.get("file").name, "sample.txt");
  await session.logout();
  for (const config of requests.filter((config) => ["auth/refresh", "auth/logout"].includes(config.url))) {
    assert.equal(config.headers.get("Authorization"), undefined);
  }
});

test("a 401 retries with the refreshed token once and concurrent requests share refresh", async () => {
  const refreshing = deferred();
  const release = deferred();
  const harness = await sessionHarness((config) => {
    if (config.headers.get("Authorization") === "Bearer test-access-1") {
      return responseFor(config, { error: { code: "unauthorized" } }, 401);
    }
    return { monitors: [] };
  }, async (count) => {
    if (count === 2) { refreshing.resolve(); await release.promise; }
  });
  const first = harness.session.authorizedAPIRequest("monitors");
  const second = harness.session.authorizedAPIRequest("monitors");
  await refreshing.promise;
  release.resolve();
  await Promise.all([first, second]);
  assert.equal(harness.refreshes(), 2);
  const tokens = harness.requests.filter((config) => config.url === "monitors").map((config) => config.headers.get("Authorization"));
  assert.deepEqual(tokens, ["Bearer test-access-1", "Bearer test-access-1", "Bearer test-access-2", "Bearer test-access-2"]);
});

test("a persistent 401 stops after one retry", async () => {
  const { session, requests, refreshes } = await sessionHarness((config) => responseFor(config, { error: { code: "unauthorized" } }, 401));
  await assert.rejects(session.authorizedAPIRequest("monitors"), (error) => error.status === 401 && error.code === "unauthorized");
  assert.equal(requests.filter((config) => config.url === "monitors").length, 2);
  assert.equal(refreshes(), 2);
});

test("Axios preserves API error codes, network errors, invalid JSON and empty logout responses", async () => {
  const { load } = loadClient(async (config) => {
    if (config.url === "network") throw new axios.AxiosError("Offline", "ERR_NETWORK", config);
    if (config.url === "timeout") throw new axios.AxiosError("Timeout", "ECONNABORTED", config);
    if (config.url === "invalid") return responseFor(config, "<html>bad gateway</html>");
    if (config.url === "logout") return responseFor(config, "", 204);
    if (config.url === "server") return responseFor(config, "<html>bad gateway</html>", 503);
    return responseFor(config, JSON.stringify({ error: { code: "monitor_limit" } }), 409);
  });
  const { apiRequest } = load("api");
  for (const [path, status, code] of [["quota", 409, "monitor_limit"], ["server", 503, "request_failed"], ["network", 0, "network_error"], ["timeout", 0, "network_error"], ["invalid", 0, "invalid_response"]]) {
    await assert.rejects(apiRequest(path), (error) => error.status === status && error.code === code, path);
  }
  assert.equal(await apiRequest("logout"), undefined);
});
