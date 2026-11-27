"use client";

import { ApiError } from "./api";
import { authorizedAPIRequest } from "./auth/session";

export interface Monitor {
  id: number;
  url: string;
  interval_seconds: number;
  created_at: string;
}

// Keep the frontend limit aligned with the API's PostgreSQL INTEGER interval column.
export const MAX_INTERVAL_SECONDS = 2147483647;
export const MAX_MONITOR_URL_LENGTH = 4096;

export interface MonitorPage {
  monitors: Monitor[];
  next_cursor: string;
}

/** Loads an account-scoped page; pass the previous next_cursor unchanged to continue. */
export function listMonitors(cursor = ""): Promise<MonitorPage> {
  const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : "";
  return authorizedAPIRequest(`monitors${query}`);
}

/** Stores a monitor configuration using a whole-second interval; polling is not started. */
export function createMonitor(url: string, intervalSeconds: number): Promise<Monitor> {
  return authorizedAPIRequest("monitors", {
    method: "POST",
    data: { url: url.trim(), interval_seconds: intervalSeconds },
  });
}

/** Updates the supplied fields of a monitor and returns its persisted settings. */
export function updateMonitor(id: number, changes: Partial<Pick<Monitor, "url" | "interval_seconds">>): Promise<Monitor> {
  return authorizedAPIRequest(`monitors/${id}`, { method: "PATCH", data: changes });
}

/** Deletes a monitor owned by the current account. */
export function deleteMonitor(id: number): Promise<void> {
  return authorizedAPIRequest(`monitors/${id}`, { method: "DELETE" });
}

/** Formats positive whole seconds in Russian using the largest exact unit up to hours. */
export function formatInterval(seconds: number): string {
  const secondsPerUnit = intervalUnit(seconds);
  const unit = secondsPerUnit === 3600 ? "hour" : secondsPerUnit === 60 ? "minute" : "second";
  return new Intl.NumberFormat("ru-RU", { style: "unit", unit, unitDisplay: "long" }).format(seconds / secondsPerUnit);
}

/** Returns the largest supported unit that expresses the interval as a whole number. */
export function intervalUnit(seconds: number): number {
  return seconds % 3600 === 0 ? 3600 : seconds % 60 === 0 ? 60 : 1;
}

/** Maps monitor API failures to Russian UI text with a fallback for unknown errors. */
export function monitorErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.code === "invalid_url") return `Введите полный URL с http:// или https://, без логина, пароля и фрагмента (#), длиной до ${MAX_MONITOR_URL_LENGTH} байт.`;
    if (error.code === "invalid_interval") return "Задайте целый интервал от 1 до 2 147 483 647 секунд.";
    if (error.code === "monitor_limit") return "Достигнут лимит: в аккаунте можно создать до 100 точек мониторинга.";
    if (error.code === "monitor_not_found") return "Сайт больше недоступен. Обновите страницу, чтобы загрузить список.";
    if (error.status === 401) return "Сессия истекла. Войдите в аккаунт снова.";
    if (error.status === 429) return "Слишком много запросов. Подождите минуту и попробуйте снова.";
  }
  return "Не удалось выполнить запрос. Проверьте подключение и попробуйте снова.";
}
