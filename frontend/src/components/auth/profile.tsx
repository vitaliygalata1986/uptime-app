"use client";

import Link from "next/link";
import { useEffect, useRef, useState, type ChangeEvent, type FormEvent } from "react";
import { errorMessage, type User } from "@/lib/auth/api";
import { updateProfile } from "@/lib/auth/session";
import { Avatar } from "./avatar";
import { useAuth } from "./provider";

export function Profile() {
  const auth = useAuth();
  return auth.user ? <ProfileEditor key={auth.user.id} user={auth.user} /> : null;
}

function ProfileEditor({ user }: { user: User }) {
  const fileInput = useRef<HTMLInputElement>(null);
  const [avatar, setAvatar] = useState<{ file: File; preview: string } | null>(null);
  useEffect(() => () => { if (avatar) URL.revokeObjectURL(avatar.preview); }, [avatar]);
  const [name, setName] = useState(user.name);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);

  function chooseAvatar(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    setSaved(false);
    if (!["image/jpeg", "image/png"].includes(file.type)) {
      setError("Выберите изображение JPEG или PNG.");
      return;
    }
    if (file.size > 5 * 1024 * 1024) {
      setError("Изображение должно быть не больше 5 МБ.");
      return;
    }
    setError("");
    setAvatar({ file, preview: URL.createObjectURL(file) });
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    const trimmed = name.trim();
    const length = Array.from(trimmed).length;
    setError("");
    setSaved(false);
    if (!(avatar && !trimmed && !user.name) && (length < 1 || length > 100 || /\p{Cc}/u.test(trimmed))) {
      setError("Имя должно содержать от 1 до 100 символов без управляющих символов.");
      return;
    }
    setPending(true);
    try {
      const updated = await updateProfile(trimmed, avatar?.file);
      setName(updated.name);
      setAvatar(null);
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
        <div className="profile-avatar">
          <Avatar user={user} preview={avatar?.preview} size={96} />
          <button type="button" className="secondary" onClick={() => fileInput.current?.click()}>Загрузить аватар</button>
          <input ref={fileInput} id="profile-avatar" type="file" accept="image/jpeg,image/png" aria-label="Выбрать аватар" aria-describedby="profile-avatar-hint" hidden onChange={chooseAvatar} />
          <p className="hint" id="profile-avatar-hint">JPEG или PNG, до 5 МБ, до 4096 пикселей по стороне и 16 мегапикселей.</p>
          {avatar ? <>
            <p className="hint" role="status">Новый аватар выбран. Сохраните изменения.</p>
            <button type="button" className="secondary" onClick={() => setAvatar(null)}>Отменить выбор аватара</button>
          </> : null}
        </div>
        <label htmlFor="profile-name">Имя</label>
        <input id="profile-name" name="name" autoComplete="name" value={name} required={!(avatar && !user.name)} aria-describedby="profile-name-hint" onChange={(event) => {
          setName(event.target.value);
          setSaved(false);
          setError("");
        }} />
        <p className="hint" id="profile-name-hint">От 1 до 100 символов.</p>
        {error ? <p className="error" role="alert">{error}</p> : null}
        {saved ? <p className="hint" role="status">Профиль сохранён.</p> : null}
        <button type="submit">{pending ? "Сохраняем…" : "Всё сохранить"}</button>
      </fieldset>
    </form>
  </section>;
}
