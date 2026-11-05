"use client";

import { useEffect, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "./provider";
import { restore } from "@/lib/auth/session";

export function AuthGate({ children, requireAuth = false }: { children: ReactNode; requireAuth?: boolean }) {
  const auth = useAuth();
  const router = useRouter();
  const redirect = auth.status === "authenticated" ? !requireAuth : auth.status === "anonymous" && requireAuth;
  useEffect(() => {
    if (redirect) router.replace(requireAuth ? "/login" : "/dashboard");
  }, [redirect, requireAuth, router]);
  if (auth.status === "error") return <section className="card"><h1>Не удалось проверить сессию</h1><p role="alert">{auth.error}</p><button onClick={() => void restore()}>Повторить</button></section>;
  if (auth.status === "checking" || redirect) return <section className="card" aria-live="polite"><p>Проверяем сессию…</p></section>;
  return children;
}
