"use client";

import { createContext, useContext, useEffect, useSyncExternalStore, type ReactNode } from "react";
import { getSnapshot, getServerSnapshot, subscribe, initialize, restore } from "@/lib/auth/session";

const AuthContext = createContext<ReturnType<typeof getSnapshot> | null>(null);
export function AuthProvider({ children }: { children: ReactNode }) {
  const state = useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
  useEffect(() => {
    initialize();
    const onFocus = () => {
      if (getSnapshot().status === "authenticated") void restore();
    };
    window.addEventListener("focus", onFocus);
    return () => window.removeEventListener("focus", onFocus);
  }, []);
  return <AuthContext value={state}>{children}</AuthContext>;
}
export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error("AuthProvider is missing");
  return value;
}
