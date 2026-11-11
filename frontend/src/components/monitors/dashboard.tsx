"use client";

import { useRef, useState } from "react";
import { useAuth } from "@/components/auth/provider";
import { formatInterval } from "@/lib/monitors";
import { CreateMonitorForm } from "./create-monitor-form";
import { useMonitors } from "./use-monitors";
import styles from "./dashboard.module.css";

export function MonitorDashboard() {
  const auth = useAuth();
  return auth.user ? <AccountMonitors key={auth.user.id} /> : null;
}

function AccountMonitors() {
  const { monitors, nextCursor, loading, loadError, pageError, loadingMore, retry, loadMore, addMonitor } = useMonitors();
  const [showForm, setShowForm] = useState(false);
  const [notice, setNotice] = useState("");
  const addButton = useRef<HTMLButtonElement>(null);

  function closeForm() {
    setShowForm(false);
    requestAnimationFrame(() => addButton.current?.focus());
  }

  return <section className={styles.dashboard} aria-labelledby="monitors-title">
    <div className={styles.heading}>
      <div>
        <h1 id="monitors-title">Мои сайты</h1>
        <p className="muted">Добавьте сайт и задайте частоту будущих проверок.</p>
      </div>
      <button ref={addButton} type="button" className={styles.add} aria-expanded={showForm} aria-controls="create-monitor"
        disabled={loading || !!loadError || showForm} onClick={() => {
          setShowForm(true);
          setNotice("");
        }}>Добавить сайт</button>
    </div>

    {showForm ? <CreateMonitorForm onClose={closeForm} onCreated={(created) => {
      addMonitor(created);
      closeForm();
      setNotice("Точка мониторинга создана.");
    }} /> : null}

    {notice ? <p className={styles.notice} role="status">{notice}</p> : null}
    {loading ? <p className="muted" role="status">Загружаем сайты…</p> : loadError ? <div>
      <p className="error" role="alert">{loadError}</p>
      <button type="button" className={`secondary ${styles.retry}`} onClick={retry}>Повторить загрузку</button>
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
    {pageError ? <p className="error" role="alert">{pageError}</p> : null}
    {nextCursor ? <button type="button" className={`secondary ${styles.retry}`} disabled={loadingMore} onClick={() => void loadMore()}>
      {loadingMore ? "Загружаем…" : pageError ? "Повторить загрузку сайтов" : "Показать ещё"}
    </button> : null}
  </section>;
}
