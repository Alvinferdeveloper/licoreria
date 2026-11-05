import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { ShiftsPage } from "@/features/cash/components/shifts-page";

export const metadata: Metadata = { title: "Turnos de caja" };

export default function Page() {
  return (
    <RequireSession permission="cash_shifts:view">
      <ShiftsPage />
    </RequireSession>
  );
}
