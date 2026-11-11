import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import vm from "node:vm";
import ts from "typescript";

class ApiError extends Error {
  constructor(status, code) {
    super(code);
    this.status = status;
    this.code = code;
  }
}

function deferred() {
  let resolve;
  const promise = new Promise((done) => { resolve = done; });
  return { promise, resolve };
}

async function sessionHarness(request) {
  const channels = [];
  const user = { id: 1, email: "first@example.com", name: "", avatar_url: "", created_at: "2026-11-11T00:00:00Z" };
  class Channel {
    constructor() { channels.push(this); }
    postMessage() {}
  }
  const exports = {};
  const source = readFileSync(new URL("../src/lib/auth/session.ts", import.meta.url), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  const context = vm.createContext({
    exports, Headers,
    navigator: { locks: { request: async (_name, action) => action() } },
    window: { BroadcastChannel: Channel }, BroadcastChannel: Channel,
    require: (name) => {
      if (name === "../api") return { apiRequest: (target) => target === "auth/me" ? Promise.resolve(user) : request(target) };
      if (name === "./api") return {
        ApiError, errorMessage: () => "Request failed",
        post: async () => ({ user, access_token: "test-access", expires_in: 900 }),
      };
      throw new Error(`Unexpected import: ${name}`);
    },
  });
  vm.runInContext(compiled, context, { filename: "session.js" });
  exports.initialize();
  await exports.restore();
  assert.equal(exports.getSnapshot().status, "authenticated");
  return { session: exports, channels };
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
  const { session } = await sessionHarness(() => {
    if (++attempts === 1) return Promise.reject(new ApiError(401, "unauthorized"));
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
