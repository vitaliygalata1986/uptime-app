"use client";

import Link from "next/link";
import { useState, type FormEvent } from "react";
import { signIn } from "@/lib/auth/session";
import { errorMessage } from "@/lib/auth/api";
import { AuthGate } from "./gate";

export function AuthForm({ mode }: { mode: "login" | "register" }) {
  const registering = mode === "register";
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    const form = event.currentTarget;
    const data = new FormData(form);
    const email = String(data.get("email")).trim();
    const password = String(data.get("password"));
    const length = Array.from(password).length;
    if (length < 8 || length > 128) { setError("Пароль должен содержать от 8 до 128 символов."); return; }
    if (registering && password !== data.get("confirm")) { setError("Пароли не совпадают."); return; }
    setPending(true);
    setError("");
    try { await signIn(mode, email, password); form.reset(); }
    catch (error) { setError(errorMessage(error)); }
    finally { setPending(false); }
  }
  return <AuthGate><section className="card">
    <p className="eyebrow">UPTIME / АККАУНТ</p>
    <h1>{registering ? "Создайте аккаунт" : "С возвращением"}</h1>
    <p className="muted">{registering ? "Зарегистрируйтесь, чтобы войти в личный кабинет." : "Войдите в свой личный кабинет."}</p>
    <form onSubmit={submit} aria-busy={pending}>
      <fieldset disabled={pending}>
        <label htmlFor={`${mode}-email`}>Email</label>
        <input id={`${mode}-email`} name="email" type="email" autoComplete="email" required maxLength={254} placeholder="you@example.com" />
        <label htmlFor={`${mode}-password`}>Пароль</label>
        <input id={`${mode}-password`} name="password" type="password" autoComplete={registering ? "new-password" : "current-password"} required aria-describedby={`${mode}-password-hint`} />
        <p className="hint" id={`${mode}-password-hint`}>От 8 до 128 символов. Пробелы учитываются.</p>
        {registering ? <><label htmlFor={`${mode}-confirm`}>Повторите пароль</label><input id={`${mode}-confirm`} name="confirm" type="password" autoComplete="new-password" required /></> : null}
        {error ? <p className="error" role="alert">{error}</p> : null}
        <button type="submit">{pending ? "Подождите…" : registering ? "Создать аккаунт" : "Войти"}</button>
      </fieldset>
    </form>
    <p className="footer">{registering ? "Уже есть аккаунт? " : "Нет аккаунта? "}<Link href={registering ? "/login" : "/register"}>{registering ? "Войти" : "Зарегистрироваться"}</Link></p>
  </section></AuthGate>;
}
