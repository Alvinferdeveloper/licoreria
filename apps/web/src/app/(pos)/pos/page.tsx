import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { PosScreen } from "@/features/pos";

export const metadata: Metadata = { title: "Punto de venta" };

export default function PosPage() {
  return (
    <RequireSession permission="pos:view">
      <PosScreen />
    </RequireSession>
  );
}
