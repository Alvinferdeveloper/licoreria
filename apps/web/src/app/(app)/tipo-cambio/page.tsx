import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { RatesPage } from "@/features/cash/components/rates-page";

export const metadata: Metadata = { title: "Tipo de cambio" };

export default function Page() {
  return (
    <RequireSession permission="exchange_rates:view">
      <RatesPage />
    </RequireSession>
  );
}
