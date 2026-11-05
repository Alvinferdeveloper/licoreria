import type { Metadata } from "next";
import { Suspense } from "react";
import { ResetPasswordScreen } from "@/features/auth/components/auth-screens";

export const metadata: Metadata = { title: "Contraseña nueva" };

export default function ResetPasswordPage() {
  return (
    <Suspense>
      <ResetPasswordScreen />
    </Suspense>
  );
}
