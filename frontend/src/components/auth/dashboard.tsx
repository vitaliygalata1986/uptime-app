"use client";

import { useState } from "react";
import { useAuth } from "./provider";
import { AuthGate } from "./gate";
import { logout } from "@/lib/auth/session";
import { errorMessage } from "@/lib/auth/api";

export function Dashboard() {
  const auth = useAuth();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  async function handleLogout() {
    setPending(true);
    setError("");
    try { await logout(); }
    catch (error) { setError(errorMessage(error)); }
    finally { setPending(false); }
  }
  return <AuthGate requireAuth><section className="card">
    <p className="eyebrow">UPTIME / ЛИЧНЫЙ КАБИНЕТ</p>
    <h1>Вы вошли в аккаунт</h1>
    <p className="muted">Ваша сессия активна.</p>
    {auth.user ? <dl><dt>Email</dt><dd>{auth.user.email}</dd><dt>Дата регистрации</dt><dd>{new Date(auth.user.created_at).toLocaleDateString("ru-RU", { timeZone: "UTC" })}</dd></dl> : null}
    {error ? <p className="error" role="alert">{error}</p> : null}
    <button className="secondary" onClick={handleLogout} disabled={pending}>{pending ? "Выходим…" : "Выйти из аккаунта"}</button>
  </section></AuthGate>;
}
