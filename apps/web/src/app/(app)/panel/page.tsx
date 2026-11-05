import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { DashboardPage } from "@/features/dashboard/components/dashboard-page";

export const metadata: Metadata = { title: "Panel" };

export default function Page() {
  return (
    <RequireSession permission="dashboard:view">
      <DashboardPage />
    </RequireSession>
  );
}
