"use client";

import { useEffect, useId, useRef, useState, type FormEvent } from "react";
import {
  createMonitor, intervalUnit, MAX_INTERVAL_SECONDS, MAX_MONITOR_URL_LENGTH, monitorErrorMessage, updateMonitor, type Monitor,
} from "@/lib/monitors";
import styles from "./dashboard.module.css";

interface Props {
  monitor?: Monitor;
  onSaved: (monitor: Monitor) => void;
  onClose: () => void;
}

/** Submits a monitor configuration and reports success only while this account's form is mounted. */
export function MonitorForm({ monitor, onSaved, onClose }: Props) {
  const id = useId();
  const initialInterval = monitor?.interval_seconds ?? 60;
  const initialUnit = intervalUnit(initialInterval);
  const [url, setURL] = useState(monitor?.url ?? "");
  const [amount, setAmount] = useState(String(initialInterval / initialUnit));
  const [unit, setUnit] = useState(initialUnit);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const submitting = useRef(false);
  const mounted = useRef(false);
  // Floor keeps whole minutes/hours within the API's maximum number of seconds.
  const maxAmount = Math.floor(MAX_INTERVAL_SECONDS / unit);
  const interval = Number(amount) * unit;
  const trimmedURL = url.trim();
  const unchanged = monitor?.url === trimmedURL && monitor.interval_seconds === interval;

  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; };
  }, []);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting.current || unchanged) return;
    if (!Number.isInteger(Number(amount)) || !Number.isSafeInteger(interval) || interval < 1 || interval > MAX_INTERVAL_SECONDS) {
      setError(`Введите целое число от 1 до ${maxAmount.toLocaleString("ru-RU")}.`);
      return;
    }
    submitting.current = true;
    setPending(true);
    setError("");
    try {
      const saved = monitor ? await updateMonitor(monitor.id, {
        url: trimmedURL !== monitor.url ? trimmedURL : undefined,
        interval_seconds: interval !== monitor.interval_seconds ? interval : undefined,
      }) : await createMonitor(trimmedURL, interval);
      if (mounted.current) onSaved(saved);
    } catch (error) {
      if (mounted.current) setError(monitorErrorMessage(error));
    } finally {
      submitting.current = false;
      if (mounted.current) setPending(false);
    }
  }

  return <form id={monitor ? `edit-monitor-${monitor.id}` : "create-monitor"}
    className={monitor ? styles.editForm : styles.form} onSubmit={submit} aria-labelledby={`${id}-title`} aria-busy={pending}>
    <h2 id={`${id}-title`}>{monitor ? "Редактирование сайта" : "Новая точка мониторинга"}</h2>
    <fieldset disabled={pending}>
      <label htmlFor={`${id}-url`}>URL сайта</label>
      <input id={`${id}-url`} name="url" type="url" required maxLength={MAX_MONITOR_URL_LENGTH} placeholder="https://example.com"
        autoFocus value={url} onChange={(event) => setURL(event.target.value)} aria-describedby={`${id}-hint`} />
      <p id={`${id}-hint`} className="hint">Полный адрес с http:// или https://, без логина, пароля и фрагмента (#).</p>
      <div className={styles.interval}>
        <div>
          <label htmlFor={`${id}-amount`}>Проверять каждые</label>
          <input id={`${id}-amount`} name="amount" type="number" required min={1} max={maxAmount} step={1}
            value={amount} onChange={(event) => setAmount(event.target.value)} />
        </div>
        <div>
          <label htmlFor={`${id}-unit`}>Единица времени</label>
          <select id={`${id}-unit`} name="unit" className={styles.select} value={unit} onChange={(event) => setUnit(Number(event.target.value))}>
            <option value={1}>Секунды</option>
            <option value={60}>Минуты</option>
            <option value={3600}>Часы</option>
          </select>
        </div>
      </div>
      {error ? <p className="error" role="alert">{error}</p> : null}
      <div className={styles.formActions}>
        <button type="submit" disabled={unchanged}>
          {monitor ? (pending ? "Сохраняем…" : "Сохранить") : (pending ? "Создаём…" : "Создать")}
        </button>
        <button type="button" className="secondary" onClick={onClose}>Отмена</button>
      </div>
    </fieldset>
  </form>;
}
