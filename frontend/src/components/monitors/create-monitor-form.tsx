"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import { createMonitor, MAX_INTERVAL_SECONDS, monitorErrorMessage, type Monitor } from "@/lib/monitors";
import styles from "./dashboard.module.css";

interface Props {
  onCreated: (monitor: Monitor) => void;
  onClose: () => void;
}

export function CreateMonitorForm({ onCreated, onClose }: Props) {
  const [url, setURL] = useState("");
  const [amount, setAmount] = useState("1");
  const [unit, setUnit] = useState(60);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const submitting = useRef(false);
  const mounted = useRef(false);
  const maxAmount = Math.floor(MAX_INTERVAL_SECONDS / unit);

  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; };
  }, []);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting.current) return;
    const interval = Number(amount) * unit;
    if (!Number.isInteger(Number(amount)) || !Number.isSafeInteger(interval) || interval < 1 || interval > MAX_INTERVAL_SECONDS) {
      setError(`Введите целое число от 1 до ${maxAmount.toLocaleString("ru-RU")}.`);
      return;
    }
    submitting.current = true;
    setPending(true);
    setError("");
    try {
      const created = await createMonitor(url, interval);
      if (mounted.current) onCreated(created);
    } catch (error) {
      if (mounted.current) setError(monitorErrorMessage(error));
    } finally {
      submitting.current = false;
      if (mounted.current) setPending(false);
    }
  }

  return <form id="create-monitor" className={styles.form} onSubmit={submit} aria-labelledby="create-title" aria-busy={pending}>
    <h2 id="create-title">Новая точка мониторинга</h2>
    <fieldset disabled={pending}>
      <label htmlFor="monitor-url">URL сайта</label>
      <input id="monitor-url" name="url" type="url" required maxLength={2048} placeholder="https://example.com"
        autoFocus value={url} onChange={(event) => setURL(event.target.value)} aria-describedby="url-hint" />
      <p id="url-hint" className="hint">Полный адрес с http:// или https://, без логина, пароля и фрагмента (#).</p>
      <div className={styles.interval}>
        <div>
          <label htmlFor="monitor-amount">Проверять каждые</label>
          <input id="monitor-amount" name="amount" type="number" required min={1} max={maxAmount} step={1}
            value={amount} onChange={(event) => setAmount(event.target.value)} />
        </div>
        <div>
          <label htmlFor="monitor-unit">Единица времени</label>
          <select id="monitor-unit" name="unit" className={styles.select} value={unit} onChange={(event) => setUnit(Number(event.target.value))}>
            <option value={1}>Секунды</option>
            <option value={60}>Минуты</option>
            <option value={3600}>Часы</option>
          </select>
        </div>
      </div>
      {error ? <p className="error" role="alert">{error}</p> : null}
      <div className={styles.formActions}>
        <button type="submit">{pending ? "Создаём…" : "Создать"}</button>
        <button type="button" className="secondary" onClick={onClose}>Отмена</button>
      </div>
    </fieldset>
  </form>;
}
