import { DashboardShell } from "@/components/auth/dashboard-shell";

export default function DashboardLayout({ children }: LayoutProps<"/dashboard">) {
  return <DashboardShell>{children}</DashboardShell>;
}
