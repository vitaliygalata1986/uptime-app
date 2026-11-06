"use client";

import { useAuth } from "./provider";

export function Dashboard() {
  const auth = useAuth();
  return <section className="card">
    <p className="eyebrow">UPTIME / ЛИЧНЫЙ КАБИНЕТ</p>
    <h1>Вы вошли в аккаунт</h1>
    <p className="muted">Ваша сессия активна.</p>
    {auth.user ? <dl><dt>Email</dt><dd>{auth.user.email}</dd><dt>Дата регистрации</dt><dd>{new Date(auth.user.created_at).toLocaleDateString("ru-RU", { timeZone: "UTC" })}</dd></dl> : null}
  </section>;
}
