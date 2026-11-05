import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { SalesPage } from "@/features/sales/components/sales-page";

export const metadata: Metadata = { title: "Ventas" };

export default function Page() {
  return (
    <RequireSession permission="sales:view">
      <SalesPage />
    </RequireSession>
  );
}
