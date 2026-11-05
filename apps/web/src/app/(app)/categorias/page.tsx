import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { MastersPage } from "@/features/catalog/components/masters-page";

export const metadata: Metadata = { title: "Categorías, marcas y unidades" };

export default function Page() {
  return (
    <RequireSession permission="categories:view">
      <MastersPage />
    </RequireSession>
  );
}
