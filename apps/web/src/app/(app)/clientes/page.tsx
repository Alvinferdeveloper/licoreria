import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { PartiesPage } from "@/features/parties/components/parties-page";

export const metadata: Metadata = { title: "Clientes" };

export default function Page() {
  return (
    <RequireSession permission="customers:view">
      <PartiesPage kind="customers" />
    </RequireSession>
  );
}
