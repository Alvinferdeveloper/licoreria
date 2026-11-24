import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { RiskPage } from "@/features/risk/components/risk-page";

export const metadata: Metadata = { title: "Riesgos de caja" };

export default function Page() {
  return (
    <RequireSession permission="audit:view">
      <RiskPage />
    </RequireSession>
  );
}
