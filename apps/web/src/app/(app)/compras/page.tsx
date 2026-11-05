import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { PurchasesPage } from "@/features/purchases/components/purchases-page";

export const metadata: Metadata = { title: "Compras" };

export default function Page() {
  return (
    <RequireSession permission="purchases:view">
      <PurchasesPage />
    </RequireSession>
  );
}
