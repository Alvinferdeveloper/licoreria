import type { Metadata } from "next";
import { AuditPage } from "@/features/audit/components/audit-page";
import { RequireSession } from "@/features/auth/components/require-session";

export const metadata: Metadata = { title: "Bitácora" };

export default function Page() {
  return (
    <RequireSession permission="audit:view">
      <AuditPage />
    </RequireSession>
  );
}
