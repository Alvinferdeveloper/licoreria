import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { ReportView } from "@/features/reports/components/report-view";

export const metadata: Metadata = { title: "Reporte" };

export default async function Page({ params }: { params: Promise<{ key: string }> }) {
  const { key } = await params;
  return (
    <RequireSession permission="reports:view">
      <ReportView reportKey={key} />
    </RequireSession>
  );
}
