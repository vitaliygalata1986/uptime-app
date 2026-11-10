"use client";

import { useState, type ReactNode } from "react";
import Link from "next/link";
import { logout } from "@/lib/auth/session";
import { errorMessage } from "@/lib/auth/api";
import { AuthGate } from "./gate";
import { useAuth } from "./provider";
import styles from "./dashboard-shell.module.css";

export function DashboardShell({ children }: { children: ReactNode }) {
  const auth = useAuth();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");

  async function handleLogout() {
    if (pending) return;
    setPending(true);
    setError("");
    try { await logout(); }
    catch (error) { setError(errorMessage(error)); }
    finally { setPending(false); }
  }

  return <AuthGate requireAuth>
    <div className={styles.shell}>
      <header className={styles.header}>
        <p className={styles.title}>Личный кабинет</p>
        {auth.user ? <div className={styles.actions}>
          <details className={styles.profile} onKeyDown={(event) => {
            if (event.key === "Escape") {
              event.preventDefault();
              event.currentTarget.open = false;
              event.currentTarget.querySelector("summary")?.focus();
            }
          }} onBlur={(event) => {
            if (event.relatedTarget && !event.currentTarget.contains(event.relatedTarget)) {
              event.currentTarget.open = false;
            }
          }}>
            <summary className={styles.trigger}>
              <span className={styles.avatar} aria-hidden="true">{Array.from(auth.user.name || auth.user.email)[0].toUpperCase()}</span>
              <span className={styles.identity}>
                <span className={styles.label}>{auth.user.name || "Профиль"}</span>
                <span className={styles.email}>{auth.user.email}</span>
              </span>
              <span className={styles.chevron} aria-hidden="true">⌄</span>
            </summary>
            <div className={styles.popover}>
              <p className={styles.heading}>Профиль пользователя</p>
              <dl className={styles.info}>
                <dt>Имя</dt>
                <dd>{auth.user.name || "Не указано"}</dd>
                <dt>Email</dt>
                <dd>{auth.user.email}</dd>
                <dt>Дата регистрации</dt>
                <dd>{new Date(auth.user.created_at).toLocaleDateString("ru-RU", { timeZone: "UTC" })}</dd>
              </dl>
              <Link href="/dashboard/profile" onClick={(event) => {
                event.currentTarget.closest("details")?.removeAttribute("open");
              }}>Просмотр и редактирование профиля</Link>
            </div>
          </details>
          <button type="button" className={`secondary ${styles.logout}`} onClick={handleLogout} disabled={pending}>
            {pending ? "Выходим…" : "Выйти из аккаунта"}
          </button>
        </div> : null}
      </header>
      {error ? <p className="error" role="alert">{error}</p> : null}
      <div className={styles.content}>{children}</div>
    </div>
  </AuthGate>;
}
