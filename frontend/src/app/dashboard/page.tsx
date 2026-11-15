import { MonitorDashboard } from '../components/monitors/dashboard';

// This page renders only after the auth guard restores the client session.

export const instant = false;

export default function DashboardPage() {
  return <MonitorDashboard />;
}
