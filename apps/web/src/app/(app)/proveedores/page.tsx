import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { PartiesPage } from "@/features/parties/components/parties-page";

export const metadata: Metadata = { title: "Proveedores" };

export default function Page() {
  return (
    <RequireSession permission="suppliers:view">
      <PartiesPage kind="suppliers" />
    </RequireSession>
  );
}
