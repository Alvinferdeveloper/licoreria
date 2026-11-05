import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { RolesPage } from "@/features/roles/components/roles-page";

export const metadata: Metadata = { title: "Roles y permisos" };

export default function Page() {
  return (
    <RequireSession permission="roles:view">
      <RolesPage />
    </RequireSession>
  );
}
