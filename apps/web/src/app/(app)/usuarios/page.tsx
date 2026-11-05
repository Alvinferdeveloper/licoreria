import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { UsersPage } from "@/features/users/components/users-page";

export const metadata: Metadata = { title: "Usuarios" };

export default function Page() {
  return (
    <RequireSession permission="users:view">
      <UsersPage />
    </RequireSession>
  );
}
