import { DashboardShell } from "@/components/auth/dashboard-shell";

/** Applies the shared client auth guard and account shell to all dashboard routes. */
export default function DashboardLayout({ children }: LayoutProps<"/dashboard">) {
  return <DashboardShell>{children}</DashboardShell>;
}
