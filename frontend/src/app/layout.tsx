import type { Metadata } from "next";
import { AuthProvider } from "@/components/auth/provider";
import "./globals.css";

export const metadata: Metadata = {
  title: "Uptime — личный кабинет",
  description: "Регистрация и вход в Uptime.",
};
/** Shares one browser auth provider across routes while keeping account data out of server rendering. */
export default function RootLayout({ children }: LayoutProps<"/">) {
  return <html lang="ru"><body><main><AuthProvider>{children}</AuthProvider></main><footer className="site-footer">Uptime · Ваш аккаунт</footer></body></html>;
}
