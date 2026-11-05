import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { ReportsPage } from "@/features/reports/components/reports-page";

export const metadata: Metadata = { title: "Reportes" };

export default function Page() {
  return (
    <RequireSession permission="reports:view">
      <ReportsPage />
    </RequireSession>
  );
}
