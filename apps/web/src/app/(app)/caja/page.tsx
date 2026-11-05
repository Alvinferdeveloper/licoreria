import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { CashPage } from "@/features/cash/components/cash-page";

export const metadata: Metadata = { title: "Caja" };

export default function Page() {
  return (
    <RequireSession permission="cash_shifts:create">
      <CashPage />
    </RequireSession>
  );
}
