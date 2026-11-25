"use client";

import { useEffect, useId, useRef, useState, type FormEvent } from "react";
import { deleteMonitor, monitorErrorMessage, type Monitor } from "@/lib/monitors";
import styles from "./dashboard.module.css";

interface Props {
  monitor: Monitor;
  onDeleted: (id: number) => void;
  onClose: () => void;
}

/** Confirms removal of the selected monitor and keeps the dialog open if the request fails. */
export function DeleteMonitorDialog({ monitor, onDeleted, onClose }: Props) {
  const id = useId();
  const dialog = useRef<HTMLDialogElement>(null);
  const mounted = useRef(false);
  const submitting = useRef(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const element = dialog.current;
    if (!element) return;
    mounted.current = true;
    element.showModal();
    return () => {
      mounted.current = false;
      element.close();
    };
  }, []);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting.current) return;
    submitting.current = true;
    setPending(true);
    setError("");
    try {
      await deleteMonitor(monitor.id);
      if (mounted.current) onDeleted(monitor.id);
    } catch (error) {
      if (mounted.current) setError(monitorErrorMessage(error));
    } finally {
      submitting.current = false;
      if (mounted.current) setPending(false);
    }
  }

  return <dialog ref={dialog} className={styles.dialog} aria-labelledby={`${id}-title`} aria-describedby={`${id}-url`}
    onCancel={(event) => {
      event.preventDefault();
      if (!submitting.current) onClose();
    }}>
    <h2 id={`${id}-title`}>Удалить сайт?</h2>
    <p id={`${id}-url`}>{monitor.url}</p>
    <form onSubmit={submit} aria-busy={pending}>
      {error ? <p className="error" role="alert">{error}</p> : null}
      <fieldset className={styles.formActions} disabled={pending}>
        <button type="button" className="secondary" autoFocus onClick={onClose}>Отмена</button>
        <button type="submit" className={styles.danger}>{pending ? "Удаляем…" : "Удалить"}</button>
      </fieldset>
    </form>
  </dialog>;
}
