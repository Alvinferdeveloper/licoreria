import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { SettingsPage } from "@/features/settings/components/settings-page";

export const metadata: Metadata = { title: "Configuración" };

export default function Page() {
  return (
    <RequireSession permission="settings:view">
      <SettingsPage />
    </RequireSession>
  );
}
