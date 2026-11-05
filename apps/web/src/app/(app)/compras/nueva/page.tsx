import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { PurchaseEditor } from "@/features/purchases/components/purchase-editor";

export const metadata: Metadata = { title: "Nueva compra" };

export default function Page() {
  return (
    <RequireSession permission="purchases:create">
      <PurchaseEditor id={null} />
    </RequireSession>
  );
}
