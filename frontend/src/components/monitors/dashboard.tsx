"use client";

import { useRef, useState } from "react";
import { useAuth } from "@/components/auth/provider";
import { formatInterval, type Monitor } from "@/lib/monitors";
import { DeleteMonitorDialog } from "./delete-monitor-dialog";
import { MonitorForm } from "./monitor-form";
import { useMonitors } from "./use-monitors";
import styles from "./dashboard.module.css";

/** Resets monitor data and form state by remounting the dashboard when the authenticated ID changes. */
export function MonitorDashboard() {
  const auth = useAuth();
  return auth.user ? <AccountMonitors key={auth.user.id} /> : null;
}

function AccountMonitors() {
  const {
    monitors, nextCursor, loading, loadError, pageError, loadingMore,
    retry, loadMore, addMonitor, replaceMonitor, removeMonitor,
  } = useMonitors();
  const [action, setAction] = useState<MonitorAction | null>(null);
  const [notice, setNotice] = useState("");
  const addButton = useRef<HTMLButtonElement>(null);
  const actionButton = useRef<HTMLButtonElement>(null);
  const creating = action?.type === "create";

  function openAction(next: MonitorAction, button: HTMLButtonElement) {
    actionButton.current = button;
    setNotice("");
    setAction(next);
  }

  function closeAction() {
    const button = actionButton.current;
    setAction(null);
    requestAnimationFrame(() => {
      (button?.isConnected ? button : addButton.current)?.focus();
    });
  }

  return <section className={styles.dashboard} aria-labelledby="monitors-title">
    <div className={styles.heading}>
      <div>
        <h1 id="monitors-title">Мои сайты</h1>
        <p className="muted">Добавьте сайт и задайте частоту будущих проверок.</p>
      </div>
      <button ref={addButton} type="button" className={styles.add} aria-expanded={creating}
        aria-controls={creating ? "create-monitor" : undefined}
        disabled={loading || !!loadError || action !== null}
        onClick={(event) => openAction({ type: "create" }, event.currentTarget)}>Добавить сайт</button>
    </div>

    {creating ? <MonitorForm onClose={closeAction} onSaved={(created) => {
      addMonitor(created);
      closeAction();
      setNotice("Точка мониторинга создана.");
    }} /> : null}

    {action?.type === "delete" ? <DeleteMonitorDialog monitor={action.monitor} onClose={closeAction} onDeleted={(id) => {
      removeMonitor(id);
      closeAction();
      setNotice("Сайт удалён.");
    }} /> : null}

    {notice ? <p className={styles.notice} role="status">{notice}</p> : null}
    {loading ? <p className="muted" role="status">Загружаем сайты…</p> : loadError ? <div>
      <p className="error" role="alert">{loadError}</p>
      <button type="button" className={`secondary ${styles.retry}`} onClick={retry}>Повторить загрузку</button>
    </div> : monitors.length === 0 && !nextCursor ? <div className={styles.empty}>
      <h2>Пока нет сайтов</h2>
      <p className="muted">Нажмите «Добавить сайт», чтобы создать первую точку мониторинга.</p>
    </div> : monitors.length > 0 ? <ul className={styles.list} aria-label="Точки мониторинга">
      {monitors.map((monitor) => {
        const editing = action?.type === "edit" && action.monitor.id === monitor.id;
        return <li key={monitor.id} className={styles.monitor}>
          <div className={styles.monitorSummary}>
            <div className={styles.details}>
              <a href={monitor.url} target="_blank" rel="noopener noreferrer">{monitor.url}</a>
              <p className="muted">Периодичность: {formatInterval(monitor.interval_seconds)}</p>
            </div>
            <span className={styles.status}>Опрос ещё не запущен</span>
            <div className={styles.rowActions}>
              <button type="button" className="secondary" disabled={action !== null}
                aria-label={`Редактировать ${monitor.url}`} aria-expanded={editing}
                aria-controls={editing ? `edit-monitor-${monitor.id}` : undefined}
                onClick={(event) => openAction({ type: "edit", monitor }, event.currentTarget)}>Редактировать</button>
              <button type="button" className={styles.deleteAction} disabled={action !== null}
                aria-label={`Удалить ${monitor.url}`} aria-haspopup="dialog"
                onClick={(event) => openAction({ type: "delete", monitor }, event.currentTarget)}>Удалить</button>
            </div>
          </div>
          {editing ? <MonitorForm monitor={monitor} onClose={closeAction} onSaved={(updated) => {
            replaceMonitor(updated);
            closeAction();
            setNotice("Настройки сайта сохранены.");
          }} /> : null}
        </li>;
      })}
    </ul> : null}
    {pageError ? <p className="error" role="alert">{pageError}</p> : null}
    {nextCursor ? <button type="button" className={`secondary ${styles.retry}`} disabled={loadingMore} onClick={() => void loadMore()}>
      {loadingMore ? "Загружаем…" : pageError ? "Повторить загрузку сайтов" : "Показать ещё"}
    </button> : null}
  </section>;
}

type MonitorAction = { type: "create" } | { type: "edit" | "delete"; monitor: Monitor };
