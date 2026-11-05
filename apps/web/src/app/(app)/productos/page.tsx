import type { Metadata } from "next";
import { RequireSession } from "@/features/auth/components/require-session";
import { ProductsPage } from "@/features/catalog/components/products-page";

export const metadata: Metadata = { title: "Productos" };

export default function Page() {
  return (
    <RequireSession permission="products:view">
      <ProductsPage />
    </RequireSession>
  );
}
