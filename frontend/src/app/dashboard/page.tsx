import { MonitorDashboard } from "@/components/monitors/dashboard";

// The auth guard renders this page only after restoring the client session.
export const instant = false;

export default function DashboardPage() {
  return <MonitorDashboard />;
}
