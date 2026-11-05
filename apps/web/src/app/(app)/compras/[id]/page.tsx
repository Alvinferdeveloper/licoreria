import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { PurchaseEditor } from "@/features/purchases/components/purchase-editor";

export const metadata: Metadata = { title: "Compra" };

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return (
    <RequireSession permission="purchases:view">
      <PurchaseEditor id={id} />
    </RequireSession>
  );
}
