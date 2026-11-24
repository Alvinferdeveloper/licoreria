import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { StatusPage } from "@/features/system/components/status-page";

export const metadata: Metadata = { title: "Estado del sistema" };

export default function Page() {
  return (
    <RequireSession permission="system:view">
      <StatusPage />
    </RequireSession>
  );
}
