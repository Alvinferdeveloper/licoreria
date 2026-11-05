import type { Metadata } from "next";
import { Suspense } from "react";
import { LoginScreen } from "@/features/auth/components/auth-screens";

export const metadata: Metadata = { title: "Iniciar sesión" };

export default function LoginPage() {
  return (
    <Suspense>
      <LoginScreen />
    </Suspense>
  );
}
