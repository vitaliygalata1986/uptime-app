import type { Metadata } from "next";
import { AuthProvider } from "@/components/auth/provider";
import "./globals.css";

export const metadata: Metadata = {
  title: "Uptime — личный кабинет",
  description: "Регистрация и вход в Uptime.",
};
export default function RootLayout({ children }: LayoutProps<"/">) {
  return <html lang="ru"><body><header className="brand">uptime<span>●</span></header><main><AuthProvider>{children}</AuthProvider></main><footer className="site-footer">Uptime · Ваш аккаунт</footer></body></html>;
}
