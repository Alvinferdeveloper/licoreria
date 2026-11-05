import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { InventoryPage } from "@/features/inventory/components/inventory-page";

export const metadata: Metadata = { title: "Inventario" };

export default function Page() {
  return (
    <RequireSession permission="inventory:view">
      <InventoryPage />
    </RequireSession>
  );
}
