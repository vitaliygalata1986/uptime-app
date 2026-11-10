"use client";

import Link from "next/link";
import { useState, type FormEvent } from "react";
import { errorMessage, type User } from "@/lib/auth/api";
import { updateProfile } from "@/lib/auth/session";
import { useAuth } from "./provider";

export function Profile() {
  const auth = useAuth();
  return auth.user ? <ProfileEditor key={auth.user.id} user={auth.user} /> : null;
}

function ProfileEditor({ user }: { user: User }) {
  const [name, setName] = useState(user.name);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    const trimmed = name.trim();
    const length = Array.from(trimmed).length;
    setError("");
    setSaved(false);
    if (length < 1 || length > 100 || /\p{Cc}/u.test(trimmed)) {
      setError("Имя должно содержать от 1 до 100 символов без управляющих символов.");
      return;
    }
    setPending(true);
    try {
      const updated = await updateProfile(trimmed);
      setName(updated.name);
      setSaved(true);
    } catch (error) {
      setError(errorMessage(error));
    } finally {
      setPending(false);
    }
  }

  return <section className="card">
    <Link href="/dashboard">← Личный кабинет</Link>
    <h1>Мой профиль</h1>
    <dl>
      <dt>Email</dt>
      <dd>{user.email}</dd>
      <dt>Дата регистрации</dt>
      <dd>{new Date(user.created_at).toLocaleDateString("ru-RU", { timeZone: "UTC" })}</dd>
    </dl>
    <form onSubmit={submit} aria-busy={pending}>
      <fieldset disabled={pending}>
        <label htmlFor="profile-name">Имя</label>
        <input id="profile-name" name="name" autoComplete="name" value={name} required aria-describedby="profile-name-hint" onChange={(event) => {
          setName(event.target.value);
          setSaved(false);
          setError("");
        }} />
        <p className="hint" id="profile-name-hint">От 1 до 100 символов. Можно изменить только имя.</p>
        {error ? <p className="error" role="alert">{error}</p> : null}
        {saved ? <p className="hint" role="status">Имя сохранено.</p> : null}
        <button type="submit">{pending ? "Сохраняем…" : "Сохранить изменения"}</button>
      </fieldset>
    </form>
  </section>;
}
