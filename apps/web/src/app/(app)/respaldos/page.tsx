import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { BackupsPage } from "@/features/backups/components/backups-page";

export const metadata: Metadata = { title: "Respaldos" };

export default function Page() {
  return (
    <RequireSession permission="backups:view">
      <BackupsPage />
    </RequireSession>
  );
}
