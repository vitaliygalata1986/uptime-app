import { MonitorDashboard } from "@/components/monitors/dashboard";

// The client auth guard can leave the prerendered shell empty; opt out of instant-navigation validation.
export const instant = false;

/** Renders account-scoped monitors inside the dashboard layout's client auth guard. */
export default function DashboardPage() {
  return <MonitorDashboard />;
}
