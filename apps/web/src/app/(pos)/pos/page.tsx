import type { Metadata } from "next";
import { PosScreen } from "@/features/pos";

export const metadata: Metadata = { title: "Punto de venta" };

export default function PosPage() {
  return <PosScreen />;
}
