"use client";

import { apiRequest, createAPIClient, type APIRequestOptions } from "../api";
import { ApiError, errorMessage, post, type AuthResult, type User } from "./api";

type State =
  | { status: "checking" | "anonymous"; user: null; error: null }
  | { status: "authenticated"; user: User; error: null }
  | { status: "error"; user: null; error: string };

export const initialState: State = { status: "checking", user: null, error: null };
let state: State = initialState;
let access: { token: string; expires: number } | null = null;
// Account transitions invalidate in-flight responses; profile saves also invalidate older /me reads.
let generation = 0;
let profileGeneration = 0;
let initialized = false;
let refreshFlight: Promise<void> | null = null;
let restoreFlight: Promise<void> | null = null;
let channel: BroadcastChannel | null = null;
const listeners = new Set<() => void>();
const authorizedClient = createAPIClient();

// Dispatch-time injection lets retries use the refreshed token without keeping stale caller headers.
authorizedClient.interceptors.request.use((config) => {
  if (!access) throw new ApiError(401, "session_changed");
  config.headers.set("Authorization", `Bearer ${access.token}`);
  return config;
}, (error: unknown) => { throw error; }, { synchronous: true });

/** Returns the stable client snapshot consumed by useSyncExternalStore. */
export const getSnapshot = () => state;
/** Keeps server rendering and initial hydration free of browser-specific session data. */
export const getServerSnapshot = () => initialState;
/** Subscribes to session changes and returns the cleanup callback required by React. */
export function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
}
function publish(next: State) {
  state = next;
  listeners.forEach((listener) => listener());
}
function clear() {
  access = null;
  publish({ status: "anonymous", user: null, error: null });
}
function save(result: AuthResult) {
  // Refresh five seconds early to allow for request latency; expires_in is in seconds.
  access = { token: result.access_token, expires: Date.now() + result.expires_in * 1000 - 5000 };
}

// Cookie общая для вкладок: ротация и вход/выход используют одну очередь.
async function exclusive<T>(action: () => Promise<T>): Promise<T> {
  if (!navigator.locks || !window.BroadcastChannel) throw new ApiError(0, "unsupported_browser");
  return navigator.locks.request("uptime-auth-session", action);
}

function refresh(): Promise<void> {
  if (refreshFlight) return refreshFlight;
  const revision = generation;
  const flight = exclusive(async () => {
    if (revision !== generation) return;
    const result = await post<AuthResult>("refresh");
    if (revision === generation) save(result);
  });
  refreshFlight = flight;
  void flight.finally(() => { if (refreshFlight === flight) refreshFlight = null; }).catch(() => {});
  return flight;
}

/** Calls a protected auth endpoint with the same refresh and account-isolation rules as other APIs. */
export function authorizedRequest<T>(path: string, options: APIRequestOptions = {}): Promise<T> {
  return authorizedAPIRequest<T>(`auth/${path}`, options);
}

/**
 * Calls a relative API endpoint with the in-memory token and at most one retry after 401.
 * Rejects responses from a previous session generation, including successful retry responses.
 */
export async function authorizedAPIRequest<T>(path: string, options: APIRequestOptions = {}): Promise<T> {
  const revision = generation;
  if (!access || access.expires <= Date.now()) await refresh();
  if (revision !== generation || !access) throw new ApiError(401, "session_changed");
  const token = access.token;
  let result: T;
  try {
    result = await apiRequest<T>(path, options, authorizedClient);
  } catch (error) {
    if (!(error instanceof ApiError) || error.status !== 401 || revision !== generation) throw error;
    // Другой запрос мог уже обновить токен, пока этот ожидал ответ.
    if (access?.token === token) await refresh();
    if (revision !== generation || !access) throw new ApiError(401, "session_changed");
    result = await apiRequest<T>(path, options, authorizedClient);
  }
  if (revision !== generation) throw new ApiError(401, "session_changed");
  return result;
}

/** Saves the name and optional avatar, then publishes the saved user only for the same account. */
export async function updateProfile(name: string, avatar?: File): Promise<User> {
  const revision = generation;
  const userID = state.user?.id;
  let options: APIRequestOptions;
  if (avatar) {
    const body = new FormData();
    body.set("avatar", avatar);
    if (name) body.set("name", name);
    options = { method: "POST", data: body };
  } else {
    options = { method: "PATCH", data: { name } };
  }
  const user = await authorizedRequest<User>(avatar ? "avatar" : "profile", options);
  if (revision !== generation || state.status !== "authenticated" || state.user.id !== userID) {
    throw new ApiError(401, "session_changed");
  }
  profileGeneration++;
  publish({ status: "authenticated", user, error: null });
  return user;
}

/** Coalesces session restoration and publishes /me unless the account or saved profile changed. */
export function restore(): Promise<void> {
  if (restoreFlight) return restoreFlight;
  const revision = generation;
  const profileRevision = profileGeneration;
  // Проверка при возврате из выбора файла сохраняет открытый редактор и черновик.
  if (state.status !== "authenticated") publish(initialState);
  const flight = (async () => {
    try {
      const user = await authorizedRequest<User>("me");
      if (revision === generation && profileRevision === profileGeneration) publish({ status: "authenticated", user, error: null });
    } catch (error) {
      if (revision !== generation) return;
      if (error instanceof ApiError && error.status === 401) clear();
      else publish({ status: "error", user: null, error: errorMessage(error) });
    }
  })();
  restoreFlight = flight;
  void flight.finally(() => { if (restoreFlight === flight) restoreFlight = null; });
  return flight;
}

/** Serializes login/registration across tabs and publishes the current account after loading /me. */
export async function signIn(mode: "login" | "register", email: string, password: string) {
  const revision = ++generation;
  await exclusive(async () => {
    if (revision !== generation) return;
    const result = await post<AuthResult>(mode, { email, password });
    if (revision !== generation) return;
    save(result);
    channel?.postMessage("signed-in");
  });
  if (revision !== generation) return;
  const user = await authorizedRequest<User>("me");
  if (revision === generation) publish({ status: "authenticated", user, error: null });
}

/** Revokes the cookie-backed session before clearing local state and notifying other tabs. */
export async function logout() {
  const revision = ++generation;
  await exclusive(async () => {
    if (revision !== generation) return;
    await post<void>("logout");
    if (revision === generation) clear();
    channel?.postMessage("signed-out");
  });
}

/** Installs cross-tab session synchronization and starts restoration once per browser module instance. */
export function initialize() {
  if (initialized) return;
  initialized = true;
  if (window.BroadcastChannel) {
    channel = new BroadcastChannel("uptime-auth-session");
    channel.onmessage = (event: MessageEvent<unknown>) => {
      if (event.data !== "signed-in" && event.data !== "signed-out") return;
      generation++;
      access = null;
      // Старые ответы больше не имеют права изменять текущую сессию.
      refreshFlight = null;
      restoreFlight = null;
      if (event.data === "signed-out") clear();
      else {
        publish(initialState);
        void restore();
      }
    };
  }
  void restore();
}
