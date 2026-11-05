import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { ShiftDetail } from "@/features/cash/components/shift-detail";

export const metadata: Metadata = { title: "Turno de caja" };

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return (
    <RequireSession permission="cash_shifts:view">
      <ShiftDetail id={id} />
    </RequireSession>
  );
}
