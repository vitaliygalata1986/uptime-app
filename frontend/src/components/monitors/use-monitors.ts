"use client";

import { useEffect, useRef, useState } from "react";
import { listMonitors, monitorErrorMessage, type Monitor } from "@/lib/monitors";

// The owning dashboard is keyed by account ID; its data never survives a switch.
export function useMonitors() {
  const [monitors, setMonitors] = useState<Monitor[]>([]);
  const [nextCursor, setNextCursor] = useState("");
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [pageError, setPageError] = useState("");
  const [loadingMore, setLoadingMore] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const mounted = useRef(false);
  const paging = useRef(false);

  useEffect(() => {
    let active = true;
    mounted.current = true;
    listMonitors().then((page) => {
      if (!active) return;
      setMonitors(page.monitors);
      setNextCursor(page.next_cursor);
    }).catch((error: unknown) => {
      if (active) setLoadError(monitorErrorMessage(error));
    }).finally(() => {
      if (active) setLoading(false);
    });
    return () => { active = false; mounted.current = false; };
  }, [attempt]);

  function retry() {
    setLoadError("");
    setLoading(true);
    setAttempt((current) => current + 1);
  }

  async function loadMore() {
    if (paging.current || loading || !nextCursor) return;
    paging.current = true;
    setLoadingMore(true);
    setPageError("");
    try {
      const page = await listMonitors(nextCursor);
      if (!mounted.current) return;
      setMonitors((current) => {
        const existing = new Set(current.map((monitor) => monitor.id));
        return [...current, ...page.monitors.filter((monitor) => !existing.has(monitor.id))];
      });
      setNextCursor(page.next_cursor);
    } catch (error) {
      if (mounted.current) setPageError(monitorErrorMessage(error));
    } finally {
      paging.current = false;
      if (mounted.current) setLoadingMore(false);
    }
  }

  function addMonitor(created: Monitor) {
    setMonitors((current) => [created, ...current]);
  }

  return { monitors, nextCursor, loading, loadError, pageError, loadingMore, retry, loadMore, addMonitor };
}
