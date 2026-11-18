import type { Metadata } from "next";
import { Suspense } from "react";
import { ActivationScreen } from "@/features/auth/components/auth-screens";

export const metadata: Metadata = { title: "Activa tu cuenta" };

export default function ActivatePage() {
  return (
    <Suspense>
      <ActivationScreen />
    </Suspense>
  );
}
