"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import {
  createMonitor, formatInterval, listMonitors, MAX_INTERVAL_SECONDS, monitorErrorMessage, type Monitor,
} from "@/lib/monitors";
import styles from "./dashboard.module.css";

export function MonitorDashboard() {
  const [monitors, setMonitors] = useState<Monitor[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [attempt, setAttempt] = useState(0);
  const [showForm, setShowForm] = useState(false);
  const [url, setURL] = useState("");
  const [amount, setAmount] = useState("1");
  const [unit, setUnit] = useState(60);
  const [pending, setPending] = useState(false);
  const [formError, setFormError] = useState("");
  const [notice, setNotice] = useState("");
  const addButton = useRef<HTMLButtonElement>(null);
  const submitting = useRef(false);
  const maxAmount = Math.floor(MAX_INTERVAL_SECONDS / unit);

  useEffect(() => {
    let active = true;
    listMonitors().then(({ monitors }) => {
      if (active) setMonitors(monitors);
    }).catch((error: unknown) => {
      if (active) setLoadError(monitorErrorMessage(error));
    }).finally(() => {
      if (active) setLoading(false);
    });
    return () => { active = false; };
  }, [attempt]);

  function closeForm() {
    setShowForm(false);
    setFormError("");
    requestAnimationFrame(() => addButton.current?.focus());
  }

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting.current || loading || loadError) return;
    const interval = Number(amount) * unit;
    if (!Number.isInteger(Number(amount)) || !Number.isSafeInteger(interval) || interval < 1 || interval > MAX_INTERVAL_SECONDS) {
      setFormError(`Введите целое число от 1 до ${maxAmount.toLocaleString("ru-RU")}.`);
      return;
    }
    submitting.current = true;
    setPending(true);
    setFormError("");
    setNotice("");
    try {
      const created = await createMonitor(url, interval);
      setMonitors((current) => [created, ...current]);
      setURL("");
      setAmount("1");
      setUnit(60);
      closeForm();
      setNotice("Точка мониторинга создана.");
    } catch (error) {
      setFormError(monitorErrorMessage(error));
    } finally {
      submitting.current = false;
      setPending(false);
    }
  }

  return <section className={styles.dashboard} aria-labelledby="monitors-title">
    <div className={styles.heading}>
      <div>
        <h1 id="monitors-title">Мои сайты</h1>
        <p className="muted">Добавьте сайт и задайте частоту будущих проверок.</p>
      </div>
      <button ref={addButton} type="button" className={styles.add} aria-expanded={showForm} aria-controls="create-monitor"
        disabled={loading || !!loadError || pending} onClick={() => {
          setShowForm(true);
          setFormError("");
          setNotice("");
        }}>Добавить сайт</button>
    </div>

    {showForm ? <form id="create-monitor" className={styles.form} onSubmit={handleCreate} aria-labelledby="create-title" aria-busy={pending}>
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
        {formError ? <p className="error" role="alert">{formError}</p> : null}
        <div className={styles.formActions}>
          <button type="submit">{pending ? "Создаём…" : "Создать"}</button>
          <button type="button" className="secondary" onClick={closeForm}>Отмена</button>
        </div>
      </fieldset>
    </form> : null}

    {notice ? <p className={styles.notice} role="status">{notice}</p> : null}
    {loading ? <p className="muted" role="status">Загружаем сайты…</p> : loadError ? <div>
      <p className="error" role="alert">{loadError}</p>
      <button type="button" className={`secondary ${styles.retry}`} onClick={() => {
        setLoadError("");
        setLoading(true);
        setAttempt((current) => current + 1);
      }}>Повторить загрузку</button>
    </div> : monitors.length === 0 ? <div className={styles.empty}>
      <h2>Пока нет сайтов</h2>
      <p className="muted">Нажмите «Добавить сайт», чтобы создать первую точку мониторинга.</p>
    </div> : <ul className={styles.list} aria-label="Точки мониторинга">
      {monitors.map((monitor) => <li key={monitor.id} className={styles.monitor}>
        <div className={styles.details}>
          <a href={monitor.url} target="_blank" rel="noopener noreferrer">{monitor.url}</a>
          <p className="muted">Периодичность: {formatInterval(monitor.interval_seconds)}</p>
        </div>
        <span className={styles.status}>Опрос ещё не запущен</span>
      </li>)}
    </ul>}
  </section>;
}
