"use client";

import { ApiError } from "./api";
import { authorizedAPIRequest } from "./auth/session";

export interface Monitor {
  id: number;
  url: string;
  interval_seconds: number;
  created_at: string;
}

export const MAX_INTERVAL_SECONDS = 2147483647;

export interface MonitorPage {
  monitors: Monitor[];
  next_cursor: string;
}

export function listMonitors(cursor = ""): Promise<MonitorPage> {
  const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : "";
  return authorizedAPIRequest(`monitors${query}`);
}

export function createMonitor(url: string, intervalSeconds: number): Promise<Monitor> {
  return authorizedAPIRequest("monitors", {
    method: "POST",
    data: { url: url.trim(), interval_seconds: intervalSeconds },
  });
}

export function formatInterval(seconds: number): string {
  const [value, unit]: [number, string] = seconds % 3600 === 0 ? [seconds / 3600, "hour"]
    : seconds % 60 === 0 ? [seconds / 60, "minute"] : [seconds, "second"];
  return new Intl.NumberFormat("ru-RU", { style: "unit", unit, unitDisplay: "long" }).format(value);
}

export function monitorErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.code === "invalid_url") return "Введите полный URL с http:// или https://, без логина, пароля и фрагмента (#), длиной до 2048 байт.";
    if (error.code === "invalid_interval") return "Задайте целый интервал от 1 до 2 147 483 647 секунд.";
    if (error.code === "monitor_limit") return "Достигнут лимит: в аккаунте можно создать до 100 точек мониторинга.";
    if (error.status === 401) return "Сессия истекла. Войдите в аккаунт снова.";
    if (error.status === 429) return "Слишком много запросов. Подождите минуту и попробуйте снова.";
  }
  return "Не удалось выполнить запрос. Проверьте подключение и попробуйте снова.";
}
