import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { ImportPage } from "@/features/catalog/components/import-page";

export const metadata: Metadata = { title: "Importar productos" };

export default function Page() {
  return (
    <RequireSession permission="products:authorize">
      <ImportPage />
    </RequireSession>
  );
}
